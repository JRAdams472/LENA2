// Package domainerr provides the shared, language-level error sentinels used
// by domain services to report outcomes without leaking storage-specific
// details (pgx, SQLSTATE, etc.) to callers.
package domainerr

import "errors"

// Sentinel domain errors. Use errors.Is to test for them.
var (
	// ErrNotFound indicates the requested entity does not exist or was not
	// visible to the caller (e.g., zero rows matched an UPDATE/DELETE).
	ErrNotFound = errors.New("not found")
	// ErrConflict indicates a unique or exclusion constraint was violated, or
	// a concurrent modification resulted in a duplicate.
	ErrConflict = errors.New("conflict")
	// ErrValidation indicates input failed business-level validation before
	// reaching storage.
	ErrValidation = errors.New("validation error")
	// ErrLastAdmin indicates the requested change would leave zero active
	// administrators in the system.
	ErrLastAdmin = errors.New("last active admin")
)

// ValidationError wraps ErrValidation with optional field and message detail.
// Use errors.Is(err, ErrValidation) for broad matching; the message is safe
// for client exposure.
type ValidationError struct {
	Field string
	Msg   string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Msg
	}
	return e.Field + ": " + e.Msg
}

// Unwrap returns the underlying ErrValidation sentinel so callers can use
// errors.Is(err, ErrValidation).
func (e *ValidationError) Unwrap() error { return ErrValidation }
