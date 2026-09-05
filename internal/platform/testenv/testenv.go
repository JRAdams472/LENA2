// Package testenv provides shared helpers for Go unit and integration tests.
package testenv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
)

// NewTestDB starts a PostgreSQL 16 container, applies all migrations, and
// returns a connection pool and a terminate callback. Callers are responsible
// for calling the returned cleanup function.
func NewTestDB(t *testing.T, ctx context.Context) (*pgxpool.Pool, func(), error) {
	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("lena"),
		postgres.WithUsername("lena"),
		postgres.WithPassword("change-me"),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("start postgres container: %w", err)
	}

	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate test container: %v", err)
		}
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("get container connection string: %w", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("open pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		cleanup()
		return nil, nil, fmt.Errorf("ping database: %w", err)
	}

	if err := RunMigrations(ctx, pool); err != nil {
		pool.Close()
		cleanup()
		return nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	return pool, func() {
		pool.Close()
		cleanup()
	}, nil
}

// RunMigrations applies all *.up.sql migration files and seed scripts found
// under the repo's migrations directory.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}

	upFiles, err := filepath.Glob(filepath.Join(root, "migrations", "*.up.sql"))
	if err != nil {
		return fmt.Errorf("glob migration files: %w", err)
	}
	sort.Strings(upFiles)

	for _, f := range upFiles {
		if err := execFile(ctx, pool, f); err != nil {
			return fmt.Errorf("execute %s: %w", f, err)
		}
	}

	seedFiles, err := filepath.Glob(filepath.Join(root, "migrations", "seed", "*.sql"))
	if err != nil {
		return fmt.Errorf("glob seed files: %w", err)
	}
	sort.Strings(seedFiles)

	for _, f := range seedFiles {
		if err := execFile(ctx, pool, f); err != nil {
			return fmt.Errorf("execute %s: %w", f, err)
		}
	}

	return nil
}

// MustUser upserts a test user and returns the resulting user ID.
func MustUser(ctx context.Context, t *testing.T, pool *pgxpool.Pool, email string) int64 {
	svc := identity.NewService(pool)
	u, err := svc.UpsertUser(ctx, "test-provider", email, email, "Test User")
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return u.UserID
}

// WithUser returns a context carrying a currentuser.User for resolver tests.
func WithUser(ctx context.Context, userID int64, email string) context.Context {
	return currentuser.WithUser(ctx, currentuser.User{
		UserID:   userID,
		Provider: "test-provider",
		Email:    email,
	})
}

func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get caller file")
	}
	// internal/platform/testenv/testenv.go -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..")), nil
}

func execFile(ctx context.Context, pool *pgxpool.Pool, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, string(content))
	return err
}
