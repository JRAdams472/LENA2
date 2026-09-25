// Package testutil provides shared helpers for Go unit and integration tests.
package testutil

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
)

func ensureWindowsDockerHost() {
	if runtime.GOOS == "windows" && os.Getenv("DOCKER_HOST") == "" {
		_ = os.Setenv("DOCKER_HOST", "npipe:////./pipe/docker_engine")
	}
}

// NewTestDB starts a PostgreSQL 16 container, applies all migrations, and
// returns a connection pool and a terminate callback. Callers are responsible
// for calling the returned cleanup function.
func NewTestDB(t *testing.T, ctx context.Context) (*pgxpool.Pool, func(), error) {
	ensureWindowsDockerHost()
	// postgres logs "ready to accept connections" twice: once during initdb
	// bootstrap and once after the real startup. Wait for the second
	// occurrence plus the port, otherwise the first connection hits EOF.
	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("lena"),
		postgres.WithUsername("lena"),
		postgres.WithPassword("change-me"),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
				wait.ForListeningPort("5432/tcp"),
			),
		),
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

// MustUser upserts a test user, ensures a default household, and returns
// the resulting user ID. The household ID equals the user ID — the same
// convention the 0028 migration backfill used — unless a serially created
// household already holds that ID, in which case a fresh serial household
// is assigned instead.
func MustUser(ctx context.Context, t *testing.T, pool *pgxpool.Pool, email string) int64 {
	svc := identity.NewService(pool)
	u, err := svc.UpsertUser(ctx, "test-provider", email, email, "Test User")
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	// Mirror the authenticator's default-household path: create the row,
	// then assign it only while household_id is still NULL so an already
	// joined household is never overwritten. Explicit IDs bypass the
	// BIGSERIAL sequence, so it is bumped afterwards — otherwise
	// CreateHousehold draws a colliding household_id. The reverse can also
	// happen: a serially created household may already hold the user ID, in
	// which case the user gets a serial household of their own rather than
	// silently joining someone else's.
	householdID := u.UserID
	tag, err := pool.Exec(ctx,
		`INSERT INTO household.households (household_id, created_by) VALUES ($1, $2)
		 ON CONFLICT (household_id) DO NOTHING`, u.UserID, email)
	if err != nil {
		t.Fatalf("create test household: %v", err)
	}
	if tag.RowsAffected() == 0 {
		if err := pool.QueryRow(ctx,
			`INSERT INTO household.households (created_by) VALUES ($1) RETURNING household_id`,
			email).Scan(&householdID); err != nil {
			t.Fatalf("create fallback test household: %v", err)
		}
	}
	if _, err := pool.Exec(ctx,
		`SELECT setval('household.households_household_id_seq',
			(SELECT COALESCE(MAX(household_id), 1) FROM household.households))`); err != nil {
		t.Fatalf("sync household sequence: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE identity.users SET household_id = $2, household_role = 'owner'
		 WHERE user_id = $1 AND household_id IS NULL`, u.UserID, householdID); err != nil {
		t.Fatalf("assign test household: %v", err)
	}
	return u.UserID
}

// JoinHousehold moves a test user into an existing household row as a
// member, leaving their default household empty. For multi-member
// household tests.
func JoinHousehold(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID, householdID int64) {
	t.Helper()
	tag, err := pool.Exec(ctx,
		`UPDATE identity.users SET household_id = $2, household_role = 'member' WHERE user_id = $1`, userID, householdID)
	if err != nil {
		t.Fatalf("join test household: %v", err)
	}
	if tag.RowsAffected() == 0 {
		t.Fatalf("join test household: user %d not found", userID)
	}
}

// WithUser returns a context carrying a currentuser.User for resolver
// tests. HouseholdID defaults to the user ID, matching MustUser's
// default-household convention.
func WithUser(ctx context.Context, userID int64, email string) context.Context {
	return WithHousehold(ctx, userID, userID, email)
}

// WithHousehold returns a context carrying a currentuser.User whose
// household scope differs from the user ID — for multi-member household
// tests. HouseholdRole defaults to 'owner', matching the
// default-household convention.
func WithHousehold(ctx context.Context, userID, householdID int64, email string) context.Context {
	return WithHouseholdRole(ctx, userID, householdID, "owner", email)
}

// WithHouseholdRole is WithHousehold with an explicit household role —
// for permission-matrix tests (e.g. a plain member attempting an
// owner-only mutation).
func WithHouseholdRole(ctx context.Context, userID, householdID int64, role, email string) context.Context {
	return currentuser.WithUser(ctx, currentuser.User{
		UserID:        userID,
		Provider:      "test-provider",
		Email:         email,
		HouseholdID:   householdID,
		HouseholdRole: role,
	})
}

// WithAdmin returns a context carrying an admin currentuser.User; catalog
// mutations require this after the role-based authorization change.
func WithAdmin(ctx context.Context, userID int64, email string) context.Context {
	return currentuser.WithUser(ctx, currentuser.User{
		UserID:      userID,
		Provider:    "test-provider",
		Email:       email,
		IsAdmin:     true,
		HouseholdID: userID,
	})
}

func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get caller file")
	}
	// internal/testutil/testutil.go -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}

var (
	copySyntaxRE = regexp.MustCompile(`^\\copy\s+(\S+)\s+\(([^)]+)\)\s+FROM\s+'([^']+)'\s+WITH\s+\(([^)]+)\)\s*;?`)
	copyLineRE   = regexp.MustCompile(`(?m)^\\copy\s+.*?;`)
)

func execFile(ctx context.Context, pool *pgxpool.Pool, path string) error {
	path = filepath.Clean(path)
	// #nosec G304 -- migration paths are globbed from the repo's migrations/ directory.
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	root, err := repoRoot()
	if err != nil {
		return err
	}

	text := string(content)
	last := 0
	for _, loc := range copyLineRE.FindAllStringIndex(text, -1) {
		if loc[0] > last {
			stmt := strings.TrimSpace(text[last:loc[0]])
			if stmt != "" {
				if _, err := pool.Exec(ctx, stmt); err != nil {
					return err
				}
			}
		}
		if err := runCopy(ctx, pool, root, text[loc[0]:loc[1]]); err != nil {
			return err
		}
		last = loc[1]
	}

	if last < len(text) {
		stmt := strings.TrimSpace(text[last:])
		if stmt != "" {
			if _, err := pool.Exec(ctx, stmt); err != nil {
				return err
			}
		}
	}
	return nil
}

func runCopy(ctx context.Context, pool *pgxpool.Pool, root, stmt string) error {
	m := copySyntaxRE.FindStringSubmatch(stmt)
	if m == nil {
		return fmt.Errorf("unsupported \\copy syntax: %s", stmt)
	}

	tableName := m[1]
	columns := strings.Split(m[2], ",")
	for i := range columns {
		columns[i] = strings.TrimSpace(columns[i])
	}
	filePath := m[3]
	options := strings.ToLower(m[4])

	// Map Docker container path to repo-local path.
	if strings.HasPrefix(filePath, "/seed/") {
		filePath = filepath.Join(root, "migrations", "seed", strings.TrimPrefix(filePath, "/seed/"))
	} else if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(root, filePath)
	}

	cf, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return fmt.Errorf("open copy source %s: %w", filePath, err)
	}
	defer func() { _ = cf.Close() }()

	r := csv.NewReader(cf)
	if strings.Contains(options, "header") {
		if _, err := r.Read(); err != nil {
			return fmt.Errorf("skip csv header: %w", err)
		}
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	src := &csvCopySource{reader: r, columns: columns}
	_, err = conn.CopyFrom(ctx, pgx.Identifier{tableName}, columns, src)
	if err != nil {
		return fmt.Errorf("copy from %s: %w", filePath, err)
	}
	return nil
}

type csvCopySource struct {
	reader  *csv.Reader
	columns []string
	record  []string
	err     error
}

func (s *csvCopySource) Next() bool {
	if s.err != nil {
		return false
	}
	rec, err := s.reader.Read()
	if err != nil {
		if err == io.EOF {
			return false
		}
		s.err = err
		return false
	}
	s.record = rec
	return true
}

func (s *csvCopySource) Values() ([]interface{}, error) {
	if s.err != nil {
		return nil, s.err
	}
	if len(s.record) < len(s.columns) {
		return nil, fmt.Errorf("csv row has %d fields, expected at least %d", len(s.record), len(s.columns))
	}
	values := make([]interface{}, len(s.columns))
	for i := range s.columns {
		values[i] = strings.TrimSpace(s.record[i])
	}
	return values, nil
}

func (s *csvCopySource) Err() error { return s.err }
