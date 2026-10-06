package notifier

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// FCMSender delivers through Firebase Cloud Messaging's HTTP v1 API.
// APNs reaches iOS through the same path when that platform ships — the
// device_token.platform column already distinguishes them.
type FCMSender struct {
	client *messaging.Client
}

// NewFCMSender builds the sender from a service-account JSON file (the
// LENA_FCM_CREDENTIALS_FILE path mounted into the container).
func NewFCMSender(ctx context.Context, credentialsFile string) (*FCMSender, error) {
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(credentialsFile))
	if err != nil {
		return nil, fmt.Errorf("firebase app init: %w", err)
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("fcm client init: %w", err)
	}
	return &FCMSender{client: client}, nil
}

func (s *FCMSender) SendEach(ctx context.Context, tokens []string, msg PushMessage) []SendResult {
	res, err := s.client.SendEachForMulticast(ctx, &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: msg.Title,
			Body:  msg.Body,
		},
		Data: msg.Data,
		// Reminders are time-sensitive — high priority wakes the device
		// even in doze rather than batching to the next maintenance window.
		Android: &messaging.AndroidConfig{Priority: "high"},
	})
	out := make([]SendResult, len(tokens))
	for i, t := range tokens {
		out[i].Token = t
	}
	if err != nil {
		// Whole-batch failure (auth, network): every token shares the error.
		for i := range out {
			out[i].Err = err
		}
		return out
	}
	for i, r := range res.Responses {
		if !r.Success {
			out[i].Err = classifyFCMError(r.Error)
		}
	}
	return out
}

// classifyFCMError maps provider errors onto delivery semantics: a dead or
// malformed token is ErrTokenGone (prune, don't retry); everything else is
// a transient send error eligible for backoff retry.
func classifyFCMError(err error) error {
	if messaging.IsUnregistered(err) ||
		messaging.IsInvalidArgument(err) ||
		messaging.IsRegistrationTokenNotRegistered(err) ||
		messaging.IsMismatchedCredential(err) {
		return ErrTokenGone
	}
	return err
}
