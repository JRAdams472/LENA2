package notifier

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/notifier/sqlc"
)

// ErrTokenGone reports that the provider considers a device token dead
// (uninstalled, invalidated). The worker deletes such tokens instead of
// retrying them.
var ErrTokenGone = errors.New("device token no longer registered")

// PushMessage is the provider-neutral payload: title/body for display plus
// opaque deep-link fields the app routes on.
type PushMessage struct {
	Title string
	Body  string
	Data  map[string]string
}

// SendResult reports the per-token outcome of one multicast — FCM gives a
// result per token even when the batch call itself succeeds.
type SendResult struct {
	Token string
	Err   error
}

// Sender delivers one rendered message to a batch of device tokens. One
// implementation logs (dev/e2e), one calls FCM (production).
type Sender interface {
	SendEach(ctx context.Context, tokens []string, msg PushMessage) []SendResult
}

// LogSender records every would-be send — the default provider so dev and
// e2e stacks exercise the full pipeline without Firebase credentials.
type LogSender struct{}

// SendEach records one log line per token instead of sending — token
// redacted, outcome reported as success so the pipeline drains.
func (LogSender) SendEach(_ context.Context, tokens []string, msg PushMessage) []SendResult {
	out := make([]SendResult, len(tokens))
	for i, t := range tokens {
		slog.Info("push delivery (log provider)",
			"token", tokenSuffix(t), "title", msg.Title, "body", msg.Body, "data", msg.Data)
		out[i] = SendResult{Token: t}
	}
	return out
}

// tokenSuffix redacts tokens to their last 6 chars for logs — they are
// provider credentials and must never be logged in full.
func tokenSuffix(t string) string {
	if len(t) <= 6 {
		return "…"
	}
	return "…" + t[len(t)-6:]
}

// DeliveryConfig tunes the worker loop.
type DeliveryConfig struct {
	// PollInterval between outbox drains.
	PollInterval time.Duration
	// BatchSize caps rows claimed per drain.
	BatchSize int32
	// MaxAttempts bounds retries before a row is marked failed.
	MaxAttempts int32
	// BaseBackoff seeds the exponential retry delay (doubled per attempt).
	BaseBackoff time.Duration
}

func (c DeliveryConfig) withDefaults() DeliveryConfig {
	if c.PollInterval <= 0 {
		c.PollInterval = 5 * time.Second
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 50
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 5
	}
	if c.BaseBackoff <= 0 {
		c.BaseBackoff = 30 * time.Second
	}
	return c
}

// DeliveryWorker drains push_delivery rows to device tokens — the worker
// side of the push pipeline whose writer side is the outbox inserts in
// household.CreateNotification and the reminder sweep. Delivery is
// at-least-once: a crash between send and mark can replay a row, which is
// preferable to silently eating a push. Lifecycle mirrors the reminder
// sweep: Start spawns a goroutine, Stop cancels and waits for in-flight
// sends.
type DeliveryWorker struct {
	q      sqlc.Querier
	sender Sender
	cfg    DeliveryConfig

	mu   sync.Mutex
	stop context.CancelFunc
	wg   sync.WaitGroup
}

// NewDeliveryWorker builds a worker over the outbox queries and sender,
// filling any unset config values with production defaults.
func NewDeliveryWorker(q sqlc.Querier, sender Sender, cfg DeliveryConfig) *DeliveryWorker {
	return &DeliveryWorker{q: q, sender: sender, cfg: cfg.withDefaults()}
}

// AttachDelivery builds and starts the push delivery worker on the
// service's own querier — same lifecycle as the sweep (Stop halts both).
// Called once during server wiring; a nil worker is fine when the
// delivery channel is unused (tests).
func (s *Service) AttachDelivery(ctx context.Context, sender Sender, cfg DeliveryConfig) {
	s.delivery = NewDeliveryWorker(s.q, sender, cfg)
	s.delivery.Start(ctx)
}

// Start spawns the drain loop: an immediate drain, then one per
// PollInterval until the context cancels. A second call is a no-op.
func (w *DeliveryWorker) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stop != nil {
		return
	}
	cctx, cancel := context.WithCancel(ctx)
	w.stop = cancel
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.drain(cctx)
		t := time.NewTicker(w.cfg.PollInterval)
		defer t.Stop()
		for {
			select {
			case <-cctx.Done():
				return
			case <-t.C:
				w.drain(cctx)
			}
		}
	}()
}

// Stop cancels the drain loop and waits for any in-flight send to finish.
func (w *DeliveryWorker) Stop() {
	w.mu.Lock()
	stop := w.stop
	w.mu.Unlock()
	if stop != nil {
		stop()
		w.wg.Wait()
	}
}

func (w *DeliveryWorker) drain(ctx context.Context) {
	rows, err := w.q.ListDuePushDeliveries(ctx, w.cfg.BatchSize)
	if err != nil {
		slog.Warn("push delivery poll failed", "error", err)
		return
	}
	for _, r := range rows {
		w.deliver(ctx, r)
	}
}

// deliver claims one row, fans out to the recipient's tokens, and records
// the outcome. Sending is the slow part (provider HTTP), so the claim uses
// a status flip rather than a held row lock — stale 'sending' rows are
// rescued by the poll query.
func (w *DeliveryWorker) deliver(ctx context.Context, d sqlc.HouseholdPushDelivery) {
	claimed, err := w.q.MarkPushDeliverySending(ctx, d.PushDeliveryID)
	if err != nil {
		slog.Warn("push delivery claim failed", "delivery", d.PushDeliveryID, "error", err)
		return
	}
	if claimed == 0 {
		return // another dispatcher claimed it
	}

	tokens, err := w.q.ListDeviceTokensForUsers(ctx, []int64{d.UserID})
	if err != nil {
		slog.Warn("push token lookup failed", "delivery", d.PushDeliveryID, "error", err)
		w.retryOrFail(ctx, d, "device token lookup: "+err.Error())
		return
	}
	if len(tokens) == 0 {
		// Nothing to reach — delivery to zero devices is vacuously done.
		if err := w.q.MarkPushDeliverySent(ctx, d.PushDeliveryID); err != nil {
			slog.Warn("push delivery mark-sent failed", "delivery", d.PushDeliveryID, "error", err)
		}
		return
	}

	title, body := w.titleBody(ctx, d)
	results := w.sender.SendEach(ctx, tokenStrings(tokens), PushMessage{
		Title: title,
		Body:  body,
		Data:  pushData(d),
	})
	w.settle(ctx, d, results)
}

// titleBody renders the copy: stored text for sweep reminders, generated
// sentences for event kinds (see pushtext.go).
func (w *DeliveryWorker) titleBody(ctx context.Context, d sqlc.HouseholdPushDelivery) (title, body string) {
	actorName := ""
	if d.ActorUserID.Valid {
		names, err := w.q.ListDisplayNamesForUsers(ctx, []int64{d.ActorUserID.Int64})
		if err == nil && len(names) > 0 {
			actorName = names[0].DisplayName.String
		}
	}
	return pushText(d, actorName)
}

// settle records the multicast outcome: any delivered token means the row
// is done (re-sending would duplicate for the successes); all-dead tokens
// fail the row outright; transient failures retry with backoff until the
// attempt budget runs out.
func (w *DeliveryWorker) settle(ctx context.Context, d sqlc.HouseholdPushDelivery, results []SendResult) {
	sent, gone := 0, 0
	var lastErr string
	for _, r := range results {
		switch {
		case r.Err == nil:
			sent++
		case errors.Is(r.Err, ErrTokenGone):
			gone++
			if err := w.q.DeleteDeviceToken(ctx, r.Token); err != nil {
				slog.Warn("dead token prune failed", "token", tokenSuffix(r.Token), "error", err)
			}
		default:
			lastErr = r.Err.Error()
		}
	}
	switch {
	case sent > 0:
		if err := w.q.MarkPushDeliverySent(ctx, d.PushDeliveryID); err != nil {
			slog.Warn("push delivery mark-sent failed", "delivery", d.PushDeliveryID, "error", err)
		}
	case lastErr == "" && gone > 0:
		if err := w.q.MarkPushDeliveryFailed(ctx, failParams(d, "no live device tokens")); err != nil {
			slog.Warn("push delivery mark-failed failed", "delivery", d.PushDeliveryID, "error", err)
		}
	default:
		w.retryOrFail(ctx, d, lastErr)
	}
}

// retryOrFail schedules the next attempt or marks the row terminally
// failed once the attempt budget is spent.
func (w *DeliveryWorker) retryOrFail(ctx context.Context, d sqlc.HouseholdPushDelivery, cause string) {
	if cause == "" {
		cause = "send failed"
	}
	if d.Attempts+1 >= w.cfg.MaxAttempts {
		if err := w.q.MarkPushDeliveryFailed(ctx, failParams(d, cause)); err != nil {
			slog.Warn("push delivery mark-failed failed", "delivery", d.PushDeliveryID, "error", err)
		}
		return
	}
	backoff := w.cfg.BaseBackoff << d.Attempts
	next := time.Now().Add(backoff)
	if err := w.q.MarkPushDeliveryRetry(ctx, sqlc.MarkPushDeliveryRetryParams{
		PushDeliveryID: d.PushDeliveryID,
		NextAttemptAt:  next,
		LastError:      pgtype.Text{String: cause, Valid: true},
	}); err != nil {
		slog.Warn("push delivery retry mark failed", "delivery", d.PushDeliveryID, "error", err)
		return
	}
	slog.Debug("push delivery scheduled for retry",
		"delivery", d.PushDeliveryID, "attempt", d.Attempts+1, "next", next)
}

func failParams(d sqlc.HouseholdPushDelivery, cause string) sqlc.MarkPushDeliveryFailedParams {
	return sqlc.MarkPushDeliveryFailedParams{
		PushDeliveryID: d.PushDeliveryID,
		LastError:      pgtype.Text{String: cause, Valid: true},
	}
}

func tokenStrings(rows []sqlc.IdentityDeviceToken) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Token
	}
	return out
}

// pushData packs the deep-link fields the app routes on — opaque IDs only,
// mirroring the feed's link columns.
func pushData(d sqlc.HouseholdPushDelivery) map[string]string {
	data := map[string]string{"kind": d.Kind}
	if d.RecipeID.Valid {
		data["recipeId"] = strconv.FormatInt(d.RecipeID.Int64, 10)
	}
	if d.ItemID.Valid {
		data["itemId"] = strconv.FormatInt(d.ItemID.Int64, 10)
	}
	if d.FoodEventID.Valid {
		data["foodEventId"] = strconv.FormatInt(d.FoodEventID.Int64, 10)
	}
	if d.InviteID.Valid {
		data["inviteId"] = strconv.FormatInt(d.InviteID.Int64, 10)
	}
	if d.HouseholdID.Valid {
		data["householdId"] = strconv.FormatInt(d.HouseholdID.Int64, 10)
	}
	return data
}
