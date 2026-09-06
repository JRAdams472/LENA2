package config

import (
	"fmt"

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
	AdminEmails        string `envconfig:"ADMIN_EMAILS" default:""`
	AuthIssuers        string `envconfig:"AUTH_ISSUERS" default:"https://accounts.google.com"`
	AuthAudiences      string `envconfig:"AUTH_AUDIENCES" required:"true"`
	CORSAllowedOrigins string `envconfig:"CORS_ALLOWED_ORIGINS" default:"http://localhost"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("lena", &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &cfg, nil
}
