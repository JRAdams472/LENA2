// Package dbtx provides a shared, interface-based seam between domain
// services and their underlying Postgres connection pool. Domain services
// depend on the Pool interface here instead of the concrete *pgxpool.Pool,
// which makes them substitutable in tests and gives every domain a single,
// tested way to run a multi-statement operation as one atomic transaction.
package dbtx

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Execer is the minimal set of methods sqlc's generated Queries type needs
// to run non-transactional queries. It is structurally identical to each
// domain's generated sqlc.DBTX interface, so a Pool value can be assigned
// directly wherever a domain expects its own DBTX type.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Beginner starts a new transaction. *pgxpool.Pool satisfies this.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Pool is what domain services depend on instead of a concrete
// *pgxpool.Pool: enough to run queries directly, and enough to start a
// transaction via InTx. *pgxpool.Pool satisfies Pool with no adapter.
type Pool interface {
	Execer
	Beginner
}

// InTx runs fn inside a new transaction started on pool. If fn returns an
// error, the transaction is rolled back and the error is returned as-is
// (already wrapped with context by the caller, if desired). If fn
// succeeds, the transaction is committed; a commit failure is returned
// wrapped with context.
func InTx(ctx context.Context, pool Beginner, fn func(tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
