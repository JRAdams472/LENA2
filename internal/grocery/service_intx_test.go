package grocery

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubTx is a pgx.Tx stand-in that records Exec/Commit/Rollback so tests can
// prove a tx-bound service issues its statements on the transaction. Like a
// real pgx.Tx, Rollback after Commit is a no-op.
type stubTx struct {
	execCalls  int
	closed     bool
	committed  bool
	rolledBack bool
}

func (f *stubTx) Begin(context.Context) (pgx.Tx, error) { return nil, errors.New("nested tx") }
func (f *stubTx) Commit(context.Context) error {
	f.committed = true
	f.closed = true
	return nil
}
func (f *stubTx) Rollback(context.Context) error {
	if !f.closed {
		f.rolledBack = true
		f.closed = true
	}
	return nil
}
func (f *stubTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("not implemented")
}
func (f *stubTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (f *stubTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (f *stubTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("not implemented")
}
func (f *stubTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	f.execCalls++
	return pgconn.CommandTag{}, nil
}
func (f *stubTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}
func (f *stubTx) QueryRow(context.Context, string, ...any) pgx.Row { return nil }
func (f *stubTx) Conn() *pgx.Conn                                  { return nil }

// stubPool satisfies dbtx.Pool: statements are stubbed out, Begin returns a
// fixed transaction.
type stubPool struct {
	tx *stubTx
}

func (p *stubPool) Begin(context.Context) (pgx.Tx, error) { return p.tx, nil }
func (p *stubPool) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("not implemented")
}
func (p *stubPool) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}
func (p *stubPool) QueryRow(context.Context, string, ...any) pgx.Row { return nil }

func TestServiceInTx_BindsServiceToTransaction(t *testing.T) {
	tx := &stubTx{}
	s := NewService(&stubPool{tx: tx})

	err := s.InTx(context.Background(), func(txSvc *Service) error {
		// A write through the tx-bound service must hit the transaction, not
		// the pool.
		return txSvc.DeleteGroceryListItem(context.Background(), 1)
	})
	require.NoError(t, err)
	assert.Equal(t, 1, tx.execCalls)
	assert.True(t, tx.committed)
	assert.False(t, tx.rolledBack)
}

func TestServiceInTx_RollsBackOnError(t *testing.T) {
	tx := &stubTx{}
	s := NewService(&stubPool{tx: tx})
	want := errors.New("boom")

	err := s.InTx(context.Background(), func(*Service) error { return want })
	assert.ErrorIs(t, err, want)
	assert.False(t, tx.committed)
	assert.True(t, tx.rolledBack)
}
