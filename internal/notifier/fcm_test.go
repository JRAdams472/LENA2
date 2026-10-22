package notifier

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	firebase "firebase.google.com/go/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
)

// fcmTestSender builds a real messaging.Client against a stub FCM
// endpoint — the SDK's error classification only fires on genuine
// *internal.FirebaseError values, so the response has to come over HTTP.
func fcmTestSender(t *testing.T, status int, body string) *FCMSender {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)

	app, err := firebase.NewApp(context.Background(), &firebase.Config{ProjectID: "lena-test"},
		option.WithEndpoint(srv.URL), option.WithoutAuthentication())
	require.NoError(t, err)
	client, err := app.Messaging(context.Background())
	require.NoError(t, err)
	return &FCMSender{client: client}
}

// fcmErrorBody is the error envelope FCM's HTTP v1 API returns — the
// messagingErrorCode detail is what IsUnregistered/IsInvalidArgument key
// on.
func fcmErrorBody(code string) string {
	return fmt.Sprintf(`{"error":{"status":"INVALID_ARGUMENT","message":"bad","details":[`+
		`{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":%q}]}}`, code)
}

// The M8 regression: a payload-level INVALID_ARGUMENT is a bad message,
// not a dead registration — it must not become ErrTokenGone or the
// dispatcher would prune every one of the user's devices over a bad
// notification.
func TestFCMSender_InvalidArgumentDoesNotPrune(t *testing.T) {
	s := fcmTestSender(t, http.StatusBadRequest, fcmErrorBody("INVALID_ARGUMENT"))
	res := s.SendEach(context.Background(), []string{"tok-1"}, PushMessage{Title: "t", Body: "b"})
	require.Len(t, res, 1)
	require.Error(t, res[0].Err)
	assert.NotErrorIs(t, res[0].Err, ErrTokenGone)
}

// Token-dead responses still classify for pruning.
func TestFCMSender_DeadTokenClassifiesGone(t *testing.T) {
	for _, code := range []string{"UNREGISTERED", "SENDER_ID_MISMATCH"} {
		t.Run(code, func(t *testing.T) {
			s := fcmTestSender(t, http.StatusBadRequest, fcmErrorBody(code))
			res := s.SendEach(context.Background(), []string{"tok-1"}, PushMessage{Title: "t", Body: "b"})
			require.Len(t, res, 1)
			assert.ErrorIs(t, res[0].Err, ErrTokenGone)
		})
	}
}

func TestFCMSender_Success(t *testing.T) {
	s := fcmTestSender(t, http.StatusOK, `{"name":"projects/lena-test/messages/1"}`)
	res := s.SendEach(context.Background(), []string{"tok-1"}, PushMessage{Title: "t", Body: "b"})
	require.Len(t, res, 1)
	assert.NoError(t, res[0].Err)
}
