package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all runtime configuration for the LENA2 application.
type Config struct {
	Port               string `envconfig:"PORT" default:"8080"`
	LogLevel           string `envconfig:"LOG_LEVEL" default:"info"`
	DatabaseURL        string `envconfig:"DATABASE_URL" required:"true"`
	GoogleClientID     string `envconfig:"GOOGLE_CLIENT_ID" required:"true"`
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
