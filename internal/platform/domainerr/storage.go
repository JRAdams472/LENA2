package domainerr

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// FromStorage translates common sqlc/pgx errors into the shared domain
// contract. A nil error stays nil. pgx.ErrNoRows becomes ErrNotFound,
// Postgres unique/exclusion violations (SQLSTATE 23505) become ErrConflict
// with the constraint name for context, and everything else is passed
// through unchanged so higher layers can inspect or wrap it further.
func FromStorage(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("%w: %s", ErrConflict, pgErr.ConstraintName)
	}
	return err
}
