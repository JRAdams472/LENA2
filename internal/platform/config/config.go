// Package config provides platform plumbing for the LENA2 service.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all runtime configuration for the LENA2 application.
type Config struct {
	Port        string `envconfig:"PORT" default:"8080"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`
	DatabaseURL string `envconfig:"DATABASE_URL"`
	// GoogleClientID is the Google OAuth client ID handed to clients (e.g.
	// NEXT_PUBLIC_GOOGLE_CLIENT_ID); it is not used for server-side
	// audience validation, so it is optional here.
	GoogleClientID string `envconfig:"GOOGLE_CLIENT_ID" default:""`
	// AdminEmails is a comma-separated bootstrap list of emails promoted to
	// the 'admin' role on their next authenticated request.
	AdminEmails string `envconfig:"ADMIN_EMAILS" default:""`
	// ProtectedEmails is a comma-separated list of admin emails that can
	// never be demoted or deactivated (banned) — the escape hatch that
	// keeps an owner from being locked out of their own deployment.
	ProtectedEmails string `envconfig:"PROTECTED_EMAILS" default:""`
	AuthIssuers     string `envconfig:"AUTH_ISSUERS" default:"https://accounts.google.com"`
	// TrustedProxyCIDRs is a comma-separated list of trusted proxy CIDRs
	// for X-Forwarded-For parsing. Leave empty to trust loopback only.
	TrustedProxyCIDRs  string `envconfig:"TRUSTED_PROXY_CIDRS" default:""`
	AuthAudiences      string `envconfig:"AUTH_AUDIENCES"`
	CORSAllowedOrigins string `envconfig:"CORS_ALLOWED_ORIGINS" default:"http://localhost"`
	// ServiceName is the OpenTelemetry service.name resource attribute.
	ServiceName string `envconfig:"OTEL_SERVICE_NAME" default:"lena2"`
	// OTLPEndpoint is the OTLP gRPC collector endpoint for traces; empty
	// disables trace export while metrics remain available on /metrics.
	OTLPEndpoint string `envconfig:"OTEL_EXPORTER_OTLP_ENDPOINT" default:""`
	// OTLPInsecure disables TLS on the OTLP gRPC exporter. Defaults to true
	// to preserve existing behaviour (collector on the private compose
	// network); set false when exporting to a TLS-enabled collector.
	OTLPInsecure bool `envconfig:"OTEL_EXPORTER_OTLP_INSECURE" default:"true"`
	// GraphQL hardening knobs: maximum query depth, maximum raw query
	// length in bytes, and per-user request rate limiting.
	GraphQLMaxDepth           int `envconfig:"GRAPHQL_MAX_DEPTH" default:"15"`
	GraphQLMaxQueryLength     int `envconfig:"GRAPHQL_MAX_QUERY_LENGTH" default:"8192"`
	GraphQLRateLimitPerMinute int `envconfig:"GRAPHQL_RATE_LIMIT_PER_MINUTE" default:"120"`
	GraphQLRateLimitBurst     int `envconfig:"GRAPHQL_RATE_LIMIT_BURST" default:"20"`
	// IP-keyed limiting applied before authentication so unauthenticated
	// traffic (including JWKS-refresh amplification) is throttled. Looser
	// than the per-user limit because one IP can front several users.
	IPRateLimitPerMinute int `envconfig:"IP_RATE_LIMIT_PER_MINUTE" default:"300"`
	IPRateLimitBurst     int `envconfig:"IP_RATE_LIMIT_BURST" default:"60"`
	// GraphQLBodyLimit caps the HTTP request body accepted by /graphql
	// (echo BodyLimit syntax, e.g. "64K"). The query-length limit above
	// applies to the query string inside the body, this applies to the
	// whole payload before it is read.
	GraphQLBodyLimit string `envconfig:"GRAPHQL_BODY_LIMIT" default:"4M"`
	// GraphQLTimeout bounds each GraphQL execution; resolvers outliving a
	// disconnected client are cancelled and reported as TIMEOUT.
	GraphQLTimeout time.Duration `envconfig:"GRAPHQL_TIMEOUT" default:"10s"`
	// GraphQLMaxCost bounds the number of field resolutions a single
	// request may perform; depth/length limits alone do not bound
	// cardinality of wide list queries. The floor is set by the
	// unpaginated catalog lists (brands/ingredients): a seeded catalog
	// page resolves ~65k fields, so 100k still rejects pathological
	// nested queries without breaking legitimate pages.
	GraphQLMaxCost int `envconfig:"GRAPHQL_MAX_COST" default:"100000"`
	// DatabaseStatementTimeout is the PostgreSQL statement_timeout applied
	// to every pooled connection so queries abandoned by a cancelled
	// request still die server-side. It should exceed GraphQLTimeout so
	// the deadline hits the resolver context first.
	DatabaseStatementTimeout time.Duration `envconfig:"DATABASE_STATEMENT_TIMEOUT" default:"15s"`
	// HTTP server timeouts. Without them the server is exposed to
	// slowloris-style connection exhaustion.
	HTTPReadHeaderTimeout time.Duration `envconfig:"HTTP_READ_HEADER_TIMEOUT" default:"5s"`
	HTTPReadTimeout       time.Duration `envconfig:"HTTP_READ_TIMEOUT" default:"15s"`
	HTTPWriteTimeout      time.Duration `envconfig:"HTTP_WRITE_TIMEOUT" default:"30s"`
	HTTPIdleTimeout       time.Duration `envconfig:"HTTP_IDLE_TIMEOUT" default:"60s"`
	// OCRServiceURL is the base URL of the OCR microservice.
	OCRServiceURL string `envconfig:"OCR_SERVICE_URL" default:"http://ocr:8000"`
	// OCRTimeout caps the call to the OCR service.
	OCRTimeout time.Duration `envconfig:"OCR_TIMEOUT" default:"20s"`
	// NutritionPhotoMaxBytes is the maximum decoded image size accepted by
	// submitItemNutritionPhoto.
	NutritionPhotoMaxBytes int `envconfig:"NUTRITION_PHOTO_MAX_BYTES" default:"6291456"`
	// RecipeScanMaxBytes is the maximum decoded recipe scan (image or PDF)
	// accepted by submitRecipeScan.
	RecipeScanMaxBytes int `envconfig:"RECIPE_SCAN_MAX_BYTES" default:"20971520"`
	// OllamaURL is the base URL of the local Ollama API. Empty disables the
	// recipe OCR import feature entirely.
	OllamaURL string `envconfig:"OLLAMA_URL" default:""`
	// OllamaModel is the model used for structured recipe extraction.
	OllamaModel string `envconfig:"OLLAMA_MODEL" default:"qwen2.5:7b-instruct"`
	// OllamaTemperature is the sampling temperature for extraction. Keep it
	// low (0.0-0.2) because the task is extraction, not generation.
	OllamaTemperature float64 `envconfig:"OLLAMA_TEMPERATURE" default:"0.1"`
	// OllamaNumCtx is the context window size in tokens.
	OllamaNumCtx int `envconfig:"OLLAMA_NUM_CTX" default:"8192"`
	// OllamaVisionModel is an optional vision model that can be used instead
	// of the OCR + extraction pipeline. Empty means the vision path is not
	// used.
	OllamaVisionModel string `envconfig:"OLLAMA_VISION_MODEL" default:""`
	// AIProvider selects the assistant LLM backend: "ollama" or "mock".
	// Empty disables all AI assistant features.
	AIProvider string `envconfig:"AI_PROVIDER" default:""`
	// AIModel overrides the model used for assistant calls; empty falls
	// back to OllamaModel.
	AIModel string `envconfig:"AI_MODEL" default:""`
	// AITemperature is the sampling temperature for assistant generation.
	// Slightly higher than extraction because suggestions benefit from a
	// little creativity.
	AITemperature float64 `envconfig:"AI_TEMPERATURE" default:"0.2"`
	// AINumCtx is the assistant context window in tokens.
	AINumCtx int `envconfig:"AI_NUM_CTX" default:"8192"`
	// AITimeout caps a single assistant provider call.
	AITimeout time.Duration `envconfig:"AI_TIMEOUT" default:"60s"`
	// AIMaxToolRounds bounds how many tool-call round trips one assistant
	// request may take before the loop gives up.
	AIMaxToolRounds int `envconfig:"AI_MAX_TOOL_ROUNDS" default:"5"`
	// SessionSecret signs LENA-issued access tokens (HS256) for the
	// refresh-token session flow. Empty disables session issuing — OIDC-only
	// mode, matching the pre-session behaviour.
	SessionSecret string `envconfig:"SESSION_SECRET" default:""`
	// SessionAccessTTL is the access-token lifetime; clients refresh on
	// expiry rather than re-signing in.
	SessionAccessTTL time.Duration `envconfig:"SESSION_ACCESS_TTL" default:"15m"`
	// SessionRefreshTTL is the sliding refresh-token lifetime; each
	// successful rotation extends it.
	SessionRefreshTTL time.Duration `envconfig:"SESSION_REFRESH_TTL" default:"720h"`
	// DiscordClientID/Secret configure Discord OAuth2 sign-in. Empty
	// disables the provider — /auth/session/discord and discord link
	// requests return 503. The secret never leaves the server: the
	// authorization-code exchange happens here, not in clients.
	DiscordClientID     string `envconfig:"DISCORD_CLIENT_ID" default:""`
	DiscordClientSecret string `envconfig:"DISCORD_CLIENT_SECRET" default:""`
	// DiscordRedirectURI must exactly match a redirect registered in the
	// Discord app (e.g. http://localhost/auth/discord/callback); it is
	// echoed in the token exchange.
	DiscordRedirectURI string `envconfig:"DISCORD_REDIRECT_URI" default:""`
	// ImportInbox is the directory where source images/PDFs are placed before
	// the importer processes them. The new submitRecipeScan mutation also
	// writes uploaded admin scans here.
	ImportInbox string `envconfig:"IMPORT_INBOX" default:"./import/inbox"`
	// ImportAutoAcceptConfidence is the fuzzy-match score (0.0-1.0) at which
	// an OCR'd ingredient is automatically mapped to a catalog item.
	ImportAutoAcceptConfidence float64 `envconfig:"IMPORT_AUTO_ACCEPT_CONFIDENCE" default:"0.92"`
	// ImportReviewThreshold is the minimum fuzzy-match score that will be
	// shown to the admin for manual review. Anything below this is considered
	// unmatched.
	ImportReviewThreshold float64 `envconfig:"IMPORT_REVIEW_THRESHOLD" default:"0.75"`
	// ImportWorkerConcurrency controls how many recipe imports are processed
	// in parallel. Defaults to 1 to avoid GPU contention.
	ImportWorkerConcurrency int `envconfig:"IMPORT_WORKER_CONCURRENCY" default:"1"`
	// ImportStageTimeout caps each pipeline stage (OCR, draft, mapping) so a
	// stuck worker cannot hold a job forever.
	ImportStageTimeout time.Duration `envconfig:"IMPORT_STAGE_TIMEOUT" default:"2m"`
	// OCRConfidenceThreshold is the minimum per-word confidence (0-100) the
	// importer will accept before flagging a page for re-scan.
	OCRConfidenceThreshold int `envconfig:"OCR_CONFIDENCE_THRESHOLD" default:"50"`
	// ProfanityExtraTerms is a comma-separated list of additional English
	// terms the profanity detector should flag beyond the built-in deny-list.
	ProfanityExtraTerms string `envconfig:"PROFANITY_EXTRA_TERMS" default:""`
	// IdempotencyEnabled turns on mutation dedup: Idempotency-Key replays
	// and the byte-identical payload fallback.
	IdempotencyEnabled bool `envconfig:"IDEMPOTENCY_ENABLED" default:"true"`
	// IdempotencyKeyTTL bounds how long a completed mutation response is
	// kept for replay under an explicit Idempotency-Key.
	IdempotencyKeyTTL time.Duration `envconfig:"IDEMPOTENCY_KEY_TTL" default:"24h"`
	// IdempotencyAutoTTL is the fallback dedup window for byte-identical
	// mutation payloads that arrive without a key.
	IdempotencyAutoTTL time.Duration `envconfig:"IDEMPOTENCY_AUTO_TTL" default:"30s"`
	// IdempotencyInFlightTTL bounds how long a claimed-but-unfinished
	// request may hold its key before a duplicate reclaims it.
	IdempotencyInFlightTTL time.Duration `envconfig:"IDEMPOTENCY_IN_FLIGHT_TTL" default:"60s"`
	// IdempotencyWaitTimeout bounds how long a duplicate waits on an
	// in-flight twin before returning IDEMPOTENCY_IN_FLIGHT.
	IdempotencyWaitTimeout time.Duration `envconfig:"IDEMPOTENCY_WAIT_TIMEOUT" default:"30s"`
	// NotificationSweepInterval is how often the reminder sweep runs; the
	// sweep itself is cheap (three indexed queries) so an hourly cadence
	// keeps reminders timely without waking the DB constantly.
	NotificationSweepInterval time.Duration `envconfig:"NOTIFY_SWEEP_INTERVAL" default:"1h"`
	// NotificationHour is the server-local hour reminders become due; meals
	// are date-granular so "N days before" pins to this hour.
	NotificationHour int `envconfig:"NOTIFY_HOUR" default:"8"`
	// NotificationExpiryDays is how far ahead of userprefs.household_item
	// expires_at the expiry reminder fires.
	NotificationExpiryDays int `envconfig:"NOTIFY_EXPIRY_DAYS" default:"3"`
	// AnalyticsDecayInterval is how often analytics.selection_score is
	// rebuilt from the interaction-event log.
	AnalyticsDecayInterval time.Duration `envconfig:"ANALYTICS_DECAY_INTERVAL" default:"6h"`
	// AnalyticsHalfLifeDays is the engagement half-life in days — an event
	// contributes half its weight after this long.
	AnalyticsHalfLifeDays float64 `envconfig:"ANALYTICS_HALF_LIFE_DAYS" default:"90"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("lena", &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &cfg, nil
}

// ValidateServer checks the fields required by the LENA2 API server.
// The ocrimport CLI reuses Load but does not need these values.
func (c *Config) ValidateServer() error {
	var missing []string
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.AuthAudiences == "" {
		missing = append(missing, "AUTH_AUDIENCES")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required server configuration: %s", strings.Join(missing, ", "))
	}

	if strings.Contains(c.AuthAudiences, "dummy") && !strings.Contains(c.AuthIssuers, "testissuer") {
		return fmt.Errorf("AUTH_AUDIENCES contains the placeholder value 'dummy'; set a real audience or configure a test issuer")
	}

	return nil
}
