// Package dbtxtest provides fake dbtx.Pool / pgx.Tx implementations so unit
// tests can exercise a service's real InTx Begin/Commit/Rollback flow
// without a database. Like a real pgx.Tx, Rollback after Commit is a no-op.
package dbtxtest

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Tx is a pgx.Tx stand-in that records statements and lifecycle calls.
type Tx struct {
	ExecCalls  int
	Committed  bool
	RolledBack bool
	ExecErr    error
	CommitErr  error
	closed     bool
}

// Begin always fails: the fake does not support nested transactions.
func (t *Tx) Begin(context.Context) (pgx.Tx, error) { return nil, errors.New("dbtxtest: nested tx") }

// Commit marks the tx closed and records success unless CommitErr is set.
func (t *Tx) Commit(context.Context) error {
	t.closed = true
	if t.CommitErr != nil {
		return t.CommitErr
	}
	t.Committed = true
	return nil
}

// Rollback records a rollback unless the tx is already closed.
func (t *Tx) Rollback(context.Context) error {
	if !t.closed {
		t.RolledBack = true
		t.closed = true
	}
	return nil
}

// CopyFrom is not implemented.
func (t *Tx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("dbtxtest: CopyFrom not implemented")
}

// SendBatch is not implemented.
func (t *Tx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }

// LargeObjects returns an empty handle.
func (t *Tx) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }

// Prepare is not implemented.
func (t *Tx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("dbtxtest: Prepare not implemented")
}

// Exec counts the call and returns ExecErr if set.
func (t *Tx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	t.ExecCalls++
	if t.ExecErr != nil {
		return pgconn.CommandTag{}, t.ExecErr
	}
	return pgconn.CommandTag{}, nil
}

// Query is not implemented.
func (t *Tx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("dbtxtest: Query not implemented")
}

// QueryRow is not implemented.
func (t *Tx) QueryRow(context.Context, string, ...any) pgx.Row { return nil }

// Conn is not implemented.
func (t *Tx) Conn() *pgx.Conn { return nil }

// Pool satisfies dbtx.Pool: statements are stubbed out, Begin returns the
// configured Tx (or BeginErr).
type Pool struct {
	Tx       *Tx
	BeginErr error
}

// Begin returns the configured Tx, or BeginErr if set.
func (p *Pool) Begin(context.Context) (pgx.Tx, error) {
	if p.BeginErr != nil {
		return nil, p.BeginErr
	}
	return p.Tx, nil
}

// Exec is not implemented on the pool.
func (p *Pool) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("dbtxtest: Exec on pool not implemented")
}

// Query is not implemented on the pool.
func (p *Pool) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("dbtxtest: Query on pool not implemented")
}

// QueryRow is not implemented on the pool.
func (p *Pool) QueryRow(context.Context, string, ...any) pgx.Row { return nil }
