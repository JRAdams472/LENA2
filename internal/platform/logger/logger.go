package logger

import (
	"log/slog"
	"os"
	"strings"
)

// sensitiveKeys contains substrings that identify PII or credential fields
// that should be redacted from log output.
var sensitiveKeys = []string{
	"email",
	"displayname",
	"notes",
	"password",
	"token",
	"authorization",
	"secret",
	"api_key",
	"apikey",
	"phone",
	"address",
	"credentials",
	"jwt",
}

// isSensitive reports whether a log key should have its value redacted.
func isSensitive(key string) bool {
	k := strings.ToLower(key)
	for _, s := range sensitiveKeys {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

// New creates a structured JSON logger with the requested level.
// It redacts common PII and credential fields from all emitted records.
func New(level string) *slog.Logger {
	var lv slog.Level
	switch level {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     lv,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if isSensitive(a.Key) {
				a.Value = slog.StringValue("[REDACTED]")
			}
			return a
		},
	})
	return slog.New(handler)
}
