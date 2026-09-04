package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
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
