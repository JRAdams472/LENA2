package dbtx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// txKey is the context key carrying the ambient transaction opened by a
// UnitOfWork.
type txKey struct{}

// ContextWithTx returns a context that carries tx. Services built on
// ContextExecer pick the transaction up automatically, so a UnitOfWork can
// compose calls across domain services without passing pgx types around.
func ContextWithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// TxFromContext returns the transaction stored by ContextWithTx, if any.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok && tx != nil
}

// HasTx reports whether ctx already carries a transaction.
func HasTx(ctx context.Context) bool {
	_, ok := TxFromContext(ctx)
	return ok
}

// ctxExecer routes every call to the transaction stored in ctx when one is
// present, and to inner otherwise. pgx.Tx satisfies Execer, so no adapter
// is needed for the transaction branch.
type ctxExecer struct{ inner Execer }

// ContextExecer wraps inner so queries run on the ctx-carried transaction
// when the caller is inside a UnitOfWork, and on inner otherwise.
func ContextExecer(inner Execer) Execer { return ctxExecer{inner: inner} }

func (e ctxExecer) db(ctx context.Context) Execer {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return e.inner
}

func (e ctxExecer) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return e.db(ctx).Exec(ctx, sql, args...)
}

func (e ctxExecer) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return e.db(ctx).Query(ctx, sql, args...)
}

func (e ctxExecer) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return e.db(ctx).QueryRow(ctx, sql, args...)
}

// UnitOfWork runs a callback inside a single transaction. The transaction
// travels through ctx, so every domain service called inside fn joins the
// same unit of work without the caller ever seeing pgx.Tx.
type UnitOfWork interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type unitOfWork struct{ pool Pool }

// NewUnitOfWork returns a UnitOfWork that begins transactions on pool.
// When ctx already carries a transaction, fn joins it instead of opening a
// nested one.
func NewUnitOfWork(pool Pool) UnitOfWork { return unitOfWork{pool: pool} }

func (u unitOfWork) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if HasTx(ctx) {
		return fn(ctx)
	}
	return InTx(ctx, u.pool, func(tx pgx.Tx) error {
		return fn(ContextWithTx(ctx, tx))
	})
}

// inline is a UnitOfWork that runs fn directly with no transaction. It is
// intended for tests where the participating services are mocked and no
// real pool exists.
type inline struct{}

// Inline returns a UnitOfWork that executes fn immediately without opening
// a transaction. Production resolvers must be built with NewUnitOfWork;
// Inline exists so unit tests have a single code path to exercise.
func Inline() UnitOfWork { return inline{} }

func (inline) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
