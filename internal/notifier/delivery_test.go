package notifier

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/notifier/sqlc"
	"github.com/JRAdams472/LENA2/internal/notifier/sqlc/mock"
)

type fakeSender struct {
	results []SendResult
	calls   [][]string // token batches, in call order
	err     error      // whole-call failure override
}

func (f *fakeSender) SendEach(_ context.Context, tokens []string, _ PushMessage) []SendResult {
	f.calls = append(f.calls, tokens)
	if f.err != nil {
		out := make([]SendResult, len(tokens))
		for i, t := range tokens {
			out[i] = SendResult{Token: t, Err: f.err}
		}
		return out
	}
	return f.results
}

func newWorker(t *testing.T) (*DeliveryWorker, *mock.MockQuerier, *fakeSender) {
	t.Helper()
	mq := mock.NewMockQuerier(gomock.NewController(t))
	fs := &fakeSender{}
	return NewDeliveryWorker(mq, fs, DeliveryConfig{MaxAttempts: 3, BaseBackoff: time.Second}), mq, fs
}

func deliveryRow() sqlc.HouseholdPushDelivery {
	return sqlc.HouseholdPushDelivery{
		PushDeliveryID: 42,
		UserID:         7,
		Kind:           "event_created",
		ActorUserID:    pgtype.Int8{Int64: 9, Valid: true},
		FoodEventID:    pgtype.Int8{Int64: 5, Valid: true},
	}
}

func tokens(ts ...string) []sqlc.IdentityDeviceToken {
	out := make([]sqlc.IdentityDeviceToken, len(ts))
	for i, t := range ts {
		out[i] = sqlc.IdentityDeviceToken{Token: t}
	}
	return out
}

func TestDeliver_SendsAndMarksSent(t *testing.T) {
	w, mq, fs := newWorker(t)
	d := deliveryRow()
	mq.EXPECT().MarkPushDeliverySending(gomock.Any(), int64(42)).Return(int64(1), nil)
	mq.EXPECT().ListDeviceTokensForUsers(gomock.Any(), []int64{7}).Return(tokens("tok-a", "tok-b"), nil)
	mq.EXPECT().ListDisplayNamesForUsers(gomock.Any(), []int64{9}).Return([]sqlc.ListDisplayNamesForUsersRow{
		{UserID: 9, DisplayName: pgtype.Text{String: "Alice", Valid: true}},
	}, nil)
	mq.EXPECT().MarkPushDeliverySent(gomock.Any(), int64(42)).Return(nil)
	fs.results = []SendResult{
		{Token: "tok-a"}, {Token: "tok-b"},
	}

	w.deliver(context.Background(), d)

	require.Len(t, fs.calls, 1)
	assert.Equal(t, []string{"tok-a", "tok-b"}, fs.calls[0])
}

func TestDeliver_NoTokensMarksSent(t *testing.T) {
	w, mq, fs := newWorker(t)
	mq.EXPECT().MarkPushDeliverySending(gomock.Any(), int64(42)).Return(int64(1), nil)
	mq.EXPECT().ListDeviceTokensForUsers(gomock.Any(), []int64{7}).Return(nil, nil)
	mq.EXPECT().MarkPushDeliverySent(gomock.Any(), int64(42)).Return(nil)

	w.deliver(context.Background(), deliveryRow())
	assert.Empty(t, fs.calls)
}

func TestDeliver_ClaimLostSkips(t *testing.T) {
	w, mq, fs := newWorker(t)
	mq.EXPECT().MarkPushDeliverySending(gomock.Any(), int64(42)).Return(int64(0), nil)
	w.deliver(context.Background(), deliveryRow())
	assert.Empty(t, fs.calls)
}

func TestDeliver_DeadTokenPrunedAndFailed(t *testing.T) {
	w, mq, fs := newWorker(t)
	mq.EXPECT().MarkPushDeliverySending(gomock.Any(), int64(42)).Return(int64(1), nil)
	mq.EXPECT().ListDeviceTokensForUsers(gomock.Any(), []int64{7}).Return(tokens("dead-tok"), nil)
	mq.EXPECT().ListDisplayNamesForUsers(gomock.Any(), []int64{9}).Return(nil, nil)
	mq.EXPECT().DeleteDeviceToken(gomock.Any(), "dead-tok").Return(nil)
	mq.EXPECT().MarkPushDeliveryFailed(gomock.Any(), gomock.Cond(func(a sqlc.MarkPushDeliveryFailedParams) bool {
		return a.PushDeliveryID == 42 && a.LastError.Valid
	})).Return(nil)
	fs.results = []SendResult{{Token: "dead-tok", Err: ErrTokenGone}}

	w.deliver(context.Background(), deliveryRow())
}

func TestDeliver_PartialDeadStillSucceeds(t *testing.T) {
	w, mq, fs := newWorker(t)
	mq.EXPECT().MarkPushDeliverySending(gomock.Any(), int64(42)).Return(int64(1), nil)
	mq.EXPECT().ListDeviceTokensForUsers(gomock.Any(), []int64{7}).Return(tokens("live", "dead"), nil)
	mq.EXPECT().ListDisplayNamesForUsers(gomock.Any(), []int64{9}).Return(nil, nil)
	mq.EXPECT().DeleteDeviceToken(gomock.Any(), "dead").Return(nil)
	mq.EXPECT().MarkPushDeliverySent(gomock.Any(), int64(42)).Return(nil)
	fs.results = []SendResult{
		{Token: "live"}, {Token: "dead", Err: ErrTokenGone},
	}

	w.deliver(context.Background(), deliveryRow())
}

func TestDeliver_TransientErrorRetriesWithBackoff(t *testing.T) {
	w, mq, fs := newWorker(t)
	d := deliveryRow()
	d.Attempts = 1
	mq.EXPECT().MarkPushDeliverySending(gomock.Any(), int64(42)).Return(int64(1), nil)
	mq.EXPECT().ListDeviceTokensForUsers(gomock.Any(), []int64{7}).Return(tokens("tok"), nil)
	mq.EXPECT().ListDisplayNamesForUsers(gomock.Any(), []int64{9}).Return(nil, nil)
	mq.EXPECT().MarkPushDeliveryRetry(gomock.Any(), gomock.Cond(func(a sqlc.MarkPushDeliveryRetryParams) bool {
		// attempt 1 -> backoff = BaseBackoff << 1 = 2s in the future
		return a.PushDeliveryID == 42 &&
			a.NextAttemptAt.After(time.Now().Add(time.Second)) &&
			a.LastError.String == "boom"
	})).Return(nil)
	fs.results = []SendResult{{Token: "tok", Err: errors.New("boom")}}

	w.deliver(context.Background(), d)
}

func TestDeliver_BudgetExhaustedMarksFailed(t *testing.T) {
	w, mq, fs := newWorker(t)
	d := deliveryRow()
	d.Attempts = 2 // MaxAttempts 3 -> this attempt is the last
	mq.EXPECT().MarkPushDeliverySending(gomock.Any(), int64(42)).Return(int64(1), nil)
	mq.EXPECT().ListDeviceTokensForUsers(gomock.Any(), []int64{7}).Return(tokens("tok"), nil)
	mq.EXPECT().ListDisplayNamesForUsers(gomock.Any(), []int64{9}).Return(nil, nil)
	mq.EXPECT().MarkPushDeliveryFailed(gomock.Any(), gomock.Cond(func(a sqlc.MarkPushDeliveryFailedParams) bool {
		return a.PushDeliveryID == 42 && a.LastError.String == "boom"
	})).Return(nil)
	fs.results = []SendResult{{Token: "tok", Err: errors.New("boom")}}

	w.deliver(context.Background(), d)
}

func TestDeliver_TokenLookupErrorRetries(t *testing.T) {
	w, mq, fs := newWorker(t)
	mq.EXPECT().MarkPushDeliverySending(gomock.Any(), int64(42)).Return(int64(1), nil)
	mq.EXPECT().ListDeviceTokensForUsers(gomock.Any(), []int64{7}).Return(nil, errors.New("db gone"))
	mq.EXPECT().MarkPushDeliveryRetry(gomock.Any(), gomock.Any()).Return(nil)

	w.deliver(context.Background(), deliveryRow())
	assert.Empty(t, fs.calls)
}

func TestDrain_DispatchesDueRows(t *testing.T) {
	w, mq, _ := newWorker(t)
	mq.EXPECT().ListDuePushDeliveries(gomock.Any(), int32(50)).Return([]sqlc.HouseholdPushDelivery{
		deliveryRow(),
	}, nil)
	mq.EXPECT().MarkPushDeliverySending(gomock.Any(), int64(42)).Return(int64(1), nil)
	mq.EXPECT().ListDeviceTokensForUsers(gomock.Any(), []int64{7}).Return(nil, nil)
	mq.EXPECT().MarkPushDeliverySent(gomock.Any(), int64(42)).Return(nil)

	w.drain(context.Background())
}

func TestPushData_LinkFields(t *testing.T) {
	d := sqlc.HouseholdPushDelivery{
		Kind:        "item_expiring",
		ItemID:      pgtype.Int8{Int64: 9, Valid: true},
		HouseholdID: pgtype.Int8{Int64: 3, Valid: true},
	}
	data := pushData(d)
	assert.Equal(t, "item_expiring", data["kind"])
	assert.Equal(t, "9", data["itemId"])
	assert.Equal(t, "3", data["householdId"])
	assert.NotContains(t, data, "recipeId")
}

func TestTokenSuffix_Redacts(t *testing.T) {
	assert.Equal(t, "…cdef12", tokenSuffix("abcdef12"))
	assert.Equal(t, "…", tokenSuffix("abc"))
}

var _ Sender = (*fakeSender)(nil)
var _ Sender = LogSender{}
