package dbtx

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// fakeTx is a minimal pgx.Tx stand-in that records Commit/Rollback calls.
// Every other method is an unused stub required only to satisfy the
// interface; InTx never calls them.
type fakeTx struct {
	committed  bool
	rolledBack bool
	commitErr  error
}

func (f *fakeTx) Begin(context.Context) (pgx.Tx, error) { return nil, errors.New("not implemented") }

func (f *fakeTx) Commit(context.Context) error {
	f.committed = true
	return f.commitErr
}

func (f *fakeTx) Rollback(context.Context) error {
	f.rolledBack = true
	return nil
}

func (f *fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("not implemented")
}
func (f *fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (f *fakeTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (f *fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("not implemented")
}
func (f *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeTx) QueryRow(context.Context, string, ...any) pgx.Row { return nil }
func (f *fakeTx) Conn() *pgx.Conn                                  { return nil }

// fakeBeginner returns a fixed fakeTx (or a fixed error) from Begin.
type fakeBeginner struct {
	tx        *fakeTx
	beginErr  error
	beginCall int
}

func (f *fakeBeginner) Begin(context.Context) (pgx.Tx, error) {
	f.beginCall++
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return f.tx, nil
}

func TestInTxCommitsOnSuccess(t *testing.T) {
	tx := &fakeTx{}
	b := &fakeBeginner{tx: tx}

	called := false
	err := InTx(context.Background(), b, func(pgx.Tx) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("fn was not called")
	}
	if !tx.committed {
		t.Fatal("expected commit")
	}
	// The deferred Rollback still runs after a successful Commit; on a
	// real pgx.Tx this is a documented safe no-op (ErrTxClosed, ignored).
}

func TestInTxRollsBackAndPropagatesFnError(t *testing.T) {
	tx := &fakeTx{}
	b := &fakeBeginner{tx: tx}
	wantErr := errors.New("boom")

	err := InTx(context.Background(), b, func(pgx.Tx) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("got %v, want %v", err, wantErr)
	}
	if tx.committed {
		t.Fatal("did not expect commit after fn error")
	}
	if !tx.rolledBack {
		t.Fatal("expected rollback after fn error")
	}
}

func TestInTxBeginError(t *testing.T) {
	beginErr := errors.New("connection refused")
	b := &fakeBeginner{beginErr: beginErr}

	called := false
	err := InTx(context.Background(), b, func(pgx.Tx) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if called {
		t.Fatal("fn should not be called when Begin fails")
	}
}

func TestInTxCommitError(t *testing.T) {
	commitErr := errors.New("commit failed")
	tx := &fakeTx{commitErr: commitErr}
	b := &fakeBeginner{tx: tx}

	err := InTx(context.Background(), b, func(pgx.Tx) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !tx.rolledBack {
		t.Fatal("expected the deferred rollback to still run after a failed commit")
	}
}
