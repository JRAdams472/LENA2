// Package config provides platform plumbing for the LENA2 service.
package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all runtime configuration for the LENA2 application.
type Config struct {
	Port        string `envconfig:"PORT" default:"8080"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`
	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`
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
	ProtectedEmails    string `envconfig:"PROTECTED_EMAILS" default:""`
	AuthIssuers        string `envconfig:"AUTH_ISSUERS" default:"https://accounts.google.com"`
	AuthAudiences      string `envconfig:"AUTH_AUDIENCES" required:"true"`
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
	// ImportWorkDir is the local directory where the importer keeps source
	// images, OCR output, drafts, and review decisions.
	ImportWorkDir string `envconfig:"IMPORT_WORK_DIR" default:"./import/work"`
	// ImportAutoAcceptConfidence is the fuzzy-match score (0.0-1.0) at which
	// an OCR'd ingredient is automatically mapped to a catalog item.
	ImportAutoAcceptConfidence float64 `envconfig:"IMPORT_AUTO_ACCEPT_CONFIDENCE" default:"0.92"`
	// ImportReviewThreshold is the minimum fuzzy-match score that will be
	// shown to the admin for manual review. Anything below this is considered
	// unmatched.
	ImportReviewThreshold float64 `envconfig:"IMPORT_REVIEW_THRESHOLD" default:"0.75"`
	// OCREngine selects the OCR container implementation. Options:
	// tesseract | paddle | doctr.
	OCREngine string `envconfig:"OCR_ENGINE" default:"tesseract"`
	// OCRConfidenceThreshold is the minimum per-word confidence (0-100) the
	// importer will accept before flagging a page for re-scan.
	OCRConfidenceThreshold int `envconfig:"OCR_CONFIDENCE_THRESHOLD" default:"50"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("lena", &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &cfg, nil
}
