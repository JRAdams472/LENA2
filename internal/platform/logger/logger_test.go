package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactsPII(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if isSensitive(a.Key) {
				a.Value = slog.StringValue("[REDACTED]")
			}
			return a
		},
	})
	log := slog.New(handler)

	log.Info("user action", "email", "user@example.com", "request_id", "123")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("unmarshal log: %v", err)
	}

	if rec["email"] != "[REDACTED]" {
		t.Fatalf("expected email redacted, got %v", rec["email"])
	}
	if rec["request_id"] != "123" {
		t.Fatalf("expected request_id intact, got %v", rec["request_id"])
	}
}

func TestIsSensitive(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		// sensitive keys
		{"email", true},
		{"Email", true},
		{"USER_EMAIL", true},
		{"displayName", true},
		{"password", true},
		{"access_token", true},
		{"Authorization", true},
		{"client_secret", true},
		{"api_key", true},
		{"apikey", true},
		{"phone_number", true},
		{"home_address", true},
		{"credentials", true},
		{"jwt", true},
		{"notes", true},
		// non-sensitive keys
		{"request_id", false},
		{"user_id", false},
		{"level", false},
		{"msg", false},
		{"status", false},
		{"duration_ms", false},
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			assert.Equal(t, tc.want, isSensitive(tc.key))
		})
	}
}

// captureLogger builds a logger writing JSON to buf, mirroring New but
// with a controllable writer.
func captureLogger(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if isSensitive(a.Key) {
				a.Value = slog.StringValue("[REDACTED]")
			}
			return a
		},
	}))
}

func TestRedactionCoversAllSensitiveFields(t *testing.T) {
	var buf bytes.Buffer
	log := captureLogger(&buf, slog.LevelInfo)

	log.Info("auth event",
		"email", "u@example.com",
		"displayname", "Jane Doe",
		"notes", "some notes",
		"password", "hunter2",
		"token", "tok123",
		"authorization", "Bearer xyz",
		"secret", "s3cret",
		"api_key", "key",
		"phone", "555-1234",
		"address", "1 Main St",
		"credentials", "creds",
		"jwt", "header.payload.sig",
		"safe_field", "visible",
	)

	var rec map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rec))

	for _, k := range []string{
		"email", "displayname", "notes", "password", "token",
		"authorization", "secret", "api_key", "phone", "address",
		"credentials", "jwt",
	} {
		assert.Equal(t, "[REDACTED]", rec[k], "key %q should be redacted", k)
	}
	assert.Equal(t, "visible", rec["safe_field"])
}

func TestNewLevelFiltering(t *testing.T) {
	// New writes to os.Stdout; verify level selection via Enabled instead.
	cases := []struct {
		level   string
		debugOn bool
		infoOn  bool
		warnOn  bool
		errorOn bool
	}{
		{"debug", true, true, true, true},
		{"info", false, true, true, true},
		{"warn", false, false, true, true},
		{"error", false, false, false, true},
		{"bogus", false, true, true, true}, // unknown defaults to info
		{"", false, true, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.level, func(t *testing.T) {
			log := New(tc.level)
			ctx := t.Context()
			assert.Equal(t, tc.debugOn, log.Enabled(ctx, slog.LevelDebug))
			assert.Equal(t, tc.infoOn, log.Enabled(ctx, slog.LevelInfo))
			assert.Equal(t, tc.warnOn, log.Enabled(ctx, slog.LevelWarn))
			assert.Equal(t, tc.errorOn, log.Enabled(ctx, slog.LevelError))
		})
	}
}
