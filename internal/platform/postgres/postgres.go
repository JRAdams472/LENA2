// Package postgres provides platform plumbing for the LENA2 service.
package postgres

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens and verifies a pgx connection pool. Every query is
// instrumented with an OpenTelemetry span; when no tracer provider is
// configured the tracer is a no-op. statementTimeout is applied as the
// PostgreSQL statement_timeout on every pooled connection so queries
// abandoned by a cancelled request still die server-side; zero leaves the
// server default in place.
func NewPool(ctx context.Context, databaseURL string, statementTimeout time.Duration) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid database url: %w", err)
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()
	if statementTimeout > 0 {
		cfg.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(statementTimeout.Milliseconds(), 10)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}
