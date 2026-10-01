// Package vector provides the nullable pgvector column type sqlc generates
// for `vector` columns. pgvector-go's pgvector.Vector cannot scan NULL,
// which every un-embedded recipe row is, so the schema maps `vector` to
// this type via sqlc overrides.
package vector

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"

	"github.com/pgvector/pgvector-go"
)

// Vector is a nullable pgvector value: Valid=false represents SQL NULL.
type Vector struct {
	V     []float32
	Valid bool
}

// Scan implements sql.Scanner for text-format vector values ("[1,2,3]")
// and NULL.
func (v *Vector) Scan(src any) error {
	switch s := src.(type) {
	case nil:
		v.V, v.Valid = nil, false
		return nil
	case string:
		return v.parse(s)
	case []byte:
		return v.parse(string(s))
	case pgvector.Vector:
		// With pgxvector.RegisterTypes, pgx decodes vector columns into
		// pgvector.Vector before Scanner fallback, so the source arrives
		// already decoded rather than as the text literal.
		v.V, v.Valid = s.Slice(), true
		return nil
	default:
		return fmt.Errorf("vector: unsupported scan source %T", src)
	}
}

// Value implements driver.Valuer.
func (v Vector) Value() (driver.Value, error) {
	if !v.Valid {
		return nil, nil
	}
	return Literal(v.V), nil
}

func (v *Vector) parse(s string) error {
	f, err := Parse(s)
	if err != nil {
		return err
	}
	v.V, v.Valid = f, true
	return nil
}

// Literal formats an embedding as the Postgres vector literal "[0.5,-1.2]".
// Queries pass this through ::vector casts so sqlc parameters stay strings.
func Literal(f []float32) string {
	var b strings.Builder
	b.Grow(len(f)*12 + 2)
	b.WriteByte('[')
	for i, x := range f {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(x), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// Parse parses a vector literal back into floats — the inverse of Literal.
func Parse(s string) ([]float32, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return nil, fmt.Errorf("vector: invalid literal %q", s)
	}
	body := s[1 : len(s)-1]
	if body == "" {
		return []float32{}, nil
	}
	parts := strings.Split(body, ",")
	out := make([]float32, len(parts))
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, fmt.Errorf("vector: invalid literal %q: %w", s, err)
		}
		out[i] = float32(f)
	}
	return out, nil
}
