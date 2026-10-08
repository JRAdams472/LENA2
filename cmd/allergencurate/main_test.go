package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/llm"
)

func TestProposeBatch(t *testing.T) {
	ingredients := []ingredientRow{
		{ID: 1, Name: "peanut butter"},
		{ID: 2, Name: "worcestershire sauce"},
		{ID: 3, Name: "carrots"},
	}
	known := []string{"peanuts", "fish", "gluten"}

	t.Run("flags allergens, marks new ones, and collects unresolved", func(t *testing.T) {
		prov := llm.NewMockProvider()
		prov.Handler = func(req llm.Request) (llm.Response, error) {
			assert.True(t, req.JSONMode, "propose must request JSON output")
			assert.Contains(t, req.Messages[0].Content, "worcestershire sauce")
			assert.Contains(t, req.Messages[0].Content, "peanuts")
			return llm.Response{Message: llm.Message{
				Content: `{"mappings":[
					{"ingredient_id":1,"allergens":[{"name":"Peanuts","kind":"contains"}],"confidence":"high","notes":"peanut spread"},
					{"ingredient_id":2,"allergens":[{"name":"fish","kind":"contains"},{"name":"Histamine","kind":"CONTAINS"}],"confidence":"medium","notes":"anchovies; histamine not in registry"}
				],"unresolved":[
					{"ingredient_id":3,"reason":"no allergens"}
				]}`},
			}, nil
		}

		ms, us, err := proposeBatch(context.Background(), prov, known, ingredients)
		require.NoError(t, err)
		require.Len(t, ms, 2)
		// Known allergen (case-insensitive normalization) — not new.
		assert.Equal(t, "peanuts", ms[0].Allergens[0].Name)
		assert.Equal(t, "contains", ms[0].Allergens[0].Kind)
		assert.False(t, ms[0].Allergens[0].NewAllergen)
		// Hidden source: fish in worcestershire; novel allergen flagged for review.
		assert.Equal(t, "fish", ms[1].Allergens[0].Name)
		assert.Equal(t, "histamine", ms[1].Allergens[1].Name)
		assert.True(t, ms[1].Allergens[1].NewAllergen)
		require.Len(t, us, 1)
		assert.Equal(t, int64(3), us[0].IngredientID)
		assert.Equal(t, "carrots", us[0].IngredientName)
	})

	t.Run("drops empty and duplicate flags and normalizes kind", func(t *testing.T) {
		prov := llm.NewMockProvider()
		prov.Handler = func(llm.Request) (llm.Response, error) {
			return llm.Response{Message: llm.Message{
				Content: `{"mappings":[
					{"ingredient_id":1,"allergens":[
						{"name":"peanuts","kind":"contains"},
						{"name":"Peanuts","kind":"may_contain"},
						{"name":"  ","kind":"contains"},
						{"name":"dust","kind":"weird"}
					],"confidence":"high"}
				]}`},
			}, nil
		}

		ms, _, err := proposeBatch(context.Background(), prov, known, ingredients)
		require.NoError(t, err)
		require.Len(t, ms, 1)
		require.Len(t, ms[0].Allergens, 2, "duplicate normalized name and empty name are dropped")
		assert.Equal(t, "peanuts", ms[0].Allergens[0].Name)
		// Invalid kind coerces to contains — safest classification wins.
		assert.Equal(t, "contains", ms[0].Allergens[1].Kind)
	})

	t.Run("skips mappings with no usable flags and propagates errors", func(t *testing.T) {
		prov := llm.NewMockProvider()
		prov.Handler = func(llm.Request) (llm.Response, error) {
			return llm.Response{Message: llm.Message{
				Content: `{"mappings":[{"ingredient_id":1,"allergens":[{"name":"","kind":"contains"}]}]}`},
			}, nil
		}
		ms, _, err := proposeBatch(context.Background(), prov, known, ingredients)
		require.NoError(t, err)
		assert.Empty(t, ms)

		boom := errors.New("ollama down")
		prov.Handler = func(llm.Request) (llm.Response, error) { return llm.Response{}, boom }
		_, _, err = proposeBatch(context.Background(), prov, known, ingredients)
		assert.ErrorIs(t, err, boom)

		prov.Handler = func(llm.Request) (llm.Response, error) {
			return llm.Response{Message: llm.Message{Content: "not json"}}, nil
		}
		_, _, err = proposeBatch(context.Background(), prov, known, ingredients)
		assert.ErrorContains(t, err, "parse model reply")
	})
}

func TestArtifactRoundTrip(t *testing.T) {
	// The artifact schema is the review contract between propose and
	// apply; a rename breaks reviewed files silently, so pin the shape.
	raw := `{
		"generated_at": "2026-10-04T00:00:00Z",
		"model": "qwen2.5:7b-instruct",
		"mappings": [{"ingredient_id": 42, "ingredient_name": "soy sauce", "allergens": [{"name": "soy", "kind": "contains"}, {"name": "wheat", "kind": "contains"}], "confidence": "high", "notes": "fermented soy and wheat"}],
		"unresolved": [{"ingredient_id": 7, "ingredient_name": "salt", "reason": "no allergens"}]
	}`
	var art artifact
	require.NoError(t, json.Unmarshal([]byte(raw), &art))
	require.Len(t, art.Mappings, 1)
	assert.Equal(t, int64(42), art.Mappings[0].IngredientID)
	require.Len(t, art.Mappings[0].Allergens, 2)
	assert.Equal(t, "wheat", art.Mappings[0].Allergens[1].Name)
	assert.Equal(t, "contains", art.Mappings[0].Allergens[1].Kind)
	require.Len(t, art.Unresolved, 1)
	assert.Equal(t, "no allergens", art.Unresolved[0].Reason)
}

// ---------- fakes for the DB seams ----------

// fakeRows is a minimal pgx.Rows returning canned row values.
type fakeRows struct {
	rows [][]any
	idx  int
	err  error
}

func (f *fakeRows) Close()                                       {}
func (f *fakeRows) Err() error                                   { return f.err }
func (f *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (f *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (f *fakeRows) Next() bool {
	f.idx++
	return f.idx <= len(f.rows)
}
func (f *fakeRows) Scan(dest ...any) error {
	row := f.rows[f.idx-1]
	if len(dest) != len(row) {
		return fmt.Errorf("scan arity %d != %d", len(dest), len(row))
	}
	for i, v := range row {
		switch d := dest[i].(type) {
		case *string:
			*d = v.(string)
		case *int64:
			*d = v.(int64)
		default:
			return fmt.Errorf("unsupported dest %T", d)
		}
	}
	return nil
}
func (f *fakeRows) Values() ([]any, error) { return f.rows[f.idx-1], nil }
func (f *fakeRows) RawValues() [][]byte    { return nil }
func (f *fakeRows) Conn() *pgx.Conn        { return nil }
func (f *fakeRows) TypeMap() *pgtype.Map   { return pgtype.NewMap() }

// fakeQuerier serves canned rows and records the SQL it was asked for.
type fakeQuerier struct {
	lastSQL string
	rows    pgx.Rows
	err     error
}

func (f *fakeQuerier) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	f.lastSQL = sql
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

// fakeRow is a one-shot pgx.Row.
type fakeRow struct{ scan func(dest ...any) error }

func (r fakeRow) Scan(dest ...any) error { return r.scan(dest...) }

// fakeTx resolves allergen names from a map and records exec/commit state.
type fakeTx struct {
	resolveIDs map[string]int64
	resolveErr error
	execErr    error
	execCalls  []string
	committed  bool
	rolledBack bool
	commitErr  error
}

func (t *fakeTx) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	name := args[0].(string)
	return fakeRow{scan: func(dest ...any) error {
		if t.resolveErr != nil {
			return t.resolveErr
		}
		id, ok := t.resolveIDs[normalizeName(name)]
		if !ok {
			return pgx.ErrNoRows
		}
		*dest[0].(*int64) = id
		return nil
	}}
}

func (t *fakeTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	t.execCalls = append(t.execCalls, sql)
	if t.execErr != nil {
		return pgconn.CommandTag{}, t.execErr
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (t *fakeTx) Commit(context.Context) error {
	t.committed = true
	return t.commitErr
}

func (t *fakeTx) Rollback(context.Context) error {
	t.rolledBack = true
	return nil
}

// ---------- query helpers ----------

func TestAllergenNames(t *testing.T) {
	q := &fakeQuerier{rows: &fakeRows{rows: [][]any{{"fish"}, {"peanuts"}}}}
	names, err := allergenNames(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, []string{"fish", "peanuts"}, names)

	boom := errors.New("db down")
	q = &fakeQuerier{err: boom}
	_, err = allergenNames(context.Background(), q)
	assert.ErrorIs(t, err, boom)

	q = &fakeQuerier{rows: &fakeRows{rows: [][]any{{"x"}}, err: boom}}
	_, err = allergenNames(context.Background(), q)
	assert.ErrorIs(t, err, boom)
}

func TestUnflaggedIngredients(t *testing.T) {
	ctx := context.Background()

	q := &fakeQuerier{rows: &fakeRows{rows: [][]any{{int64(1), "peanut butter"}}}}
	rows, err := unflaggedIngredients(ctx, q, false, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Contains(t, q.lastSQL, "NOT EXISTS", "default sweep must exclude already-flagged ingredients")
	assert.NotContains(t, q.lastSQL, "LIMIT")

	// -all widens the sweep; -limit caps the result set.
	q2 := &fakeQuerier{rows: &fakeRows{rows: [][]any{{int64(9), "x"}}}}
	_, err = unflaggedIngredients(ctx, q2, true, 5)
	require.NoError(t, err)
	assert.NotContains(t, q2.lastSQL, "NOT EXISTS")
	assert.Contains(t, q2.lastSQL, "LIMIT 5")

	boom := errors.New("db down")
	_, err = unflaggedIngredients(ctx, &fakeQuerier{err: boom}, false, 0)
	assert.ErrorIs(t, err, boom)
}

// ---------- propose orchestration ----------

func TestProposeAll(t *testing.T) {
	ingredients := []ingredientRow{{ID: 1, Name: "b"}, {ID: 2, Name: "a"}, {ID: 3, Name: "c"}}

	t.Run("batches, accumulates, sorts by ingredient name", func(t *testing.T) {
		calls := 0
		prov := llm.NewMockProvider()
		prov.Handler = func(llm.Request) (llm.Response, error) {
			calls++
			return llm.Response{Message: llm.Message{Content: `{"mappings":[{"ingredient_id":1,"allergens":[{"name":"x","kind":"contains"}],"confidence":"low"}],"unresolved":[{"ingredient_id":9,"reason":"none"}]}`}}, nil
		}
		art, err := proposeAll(context.Background(), prov, []string{"x"}, ingredients, 1, "m")
		require.NoError(t, err)
		assert.Equal(t, 3, calls, "batch=1 over 3 ingredients")
		require.Len(t, art.Mappings, 3)
		assert.Len(t, art.Unresolved, 3)

		// Single batch with real ids exercises the name sort.
		prov.Handler = func(llm.Request) (llm.Response, error) {
			return llm.Response{Message: llm.Message{Content: `{"mappings":[
				{"ingredient_id":1,"allergens":[{"name":"x","kind":"contains"}],"confidence":"low"},
				{"ingredient_id":3,"allergens":[{"name":"x","kind":"contains"}],"confidence":"low"},
				{"ingredient_id":2,"allergens":[{"name":"x","kind":"contains"}],"confidence":"low"}]}`}}, nil
		}
		art, err = proposeAll(context.Background(), prov, []string{"x"}, ingredients, 30, "m")
		require.NoError(t, err)
		require.Len(t, art.Mappings, 3)
		assert.Equal(t, "a", art.Mappings[0].IngredientName, "mappings sorted by name")
	})

	t.Run("batch error propagates with range", func(t *testing.T) {
		boom := errors.New("ollama down")
		prov := llm.NewMockProvider()
		prov.Handler = func(llm.Request) (llm.Response, error) { return llm.Response{}, boom }
		_, err := proposeAll(context.Background(), prov, nil, ingredients, 30, "m")
		assert.ErrorContains(t, err, "batch 0-3")
		assert.ErrorIs(t, err, boom)
	})
}

func TestWriteArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "art.json")
	art := artifact{Model: "m", Mappings: []mapping{{IngredientID: 1, IngredientName: "x"}}}
	require.NoError(t, writeArtifact(path, art))

	raw, err := os.ReadFile(path) // #nosec G304 -- test reads its own temp file
	require.NoError(t, err)
	var got artifact
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "m", got.Model)
	assert.Equal(t, int64(1), got.Mappings[0].IngredientID)

	err = writeArtifact(filepath.Join(t.TempDir(), "no", "dir", "art.json"), art)
	assert.Error(t, err)
}

func TestRunPropose(t *testing.T) {
	t.Run("bad flag errors", func(t *testing.T) {
		assert.Error(t, runPropose([]string{"-bogus"}))
	})
	t.Run("missing ollama url errors before any db work", func(t *testing.T) {
		t.Setenv("LENA_OLLAMA_URL", "")
		assert.ErrorContains(t, runPropose(nil), "LENA_OLLAMA_URL")
	})
}

func TestRunApplyFileErrors(t *testing.T) {
	assert.Error(t, runApply([]string{"-bogus"}))

	err := runApply([]string{"-in", filepath.Join(t.TempDir(), "missing.json")})
	assert.Error(t, err, "missing artifact file must fail")

	bad := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(bad, []byte("{nope"), 0o600))
	assert.ErrorContains(t, runApply([]string{"-in", bad}), "read artifact")
}

// ---------- apply ----------

func TestApplyArtifact(t *testing.T) {
	art := artifact{Mappings: []mapping{
		{IngredientID: 1, IngredientName: "peanut butter", Allergens: []allergenFlag{
			{Name: "Peanuts", Kind: "contains"},
			{Name: "fish", Kind: "may_contain"},
		}},
		{IngredientID: 2, IngredientName: "worcestershire", Allergens: []allergenFlag{
			{Name: "Fish", Kind: "CONTAINS"},
		}},
	}}

	t.Run("resolves, upserts, commits", func(t *testing.T) {
		tx := &fakeTx{resolveIDs: map[string]int64{"peanuts": 10, "fish": 11}}
		require.NoError(t, applyArtifact(context.Background(), tx, art, false))
		assert.True(t, tx.committed)
		assert.False(t, tx.rolledBack)
		assert.Len(t, tx.execCalls, 3, "one upsert per allergen flag")
	})

	t.Run("dry run rolls back", func(t *testing.T) {
		tx := &fakeTx{resolveIDs: map[string]int64{"peanuts": 10, "fish": 11}}
		require.NoError(t, applyArtifact(context.Background(), tx, art, true))
		assert.True(t, tx.rolledBack)
		assert.False(t, tx.committed)
	})

	t.Run("unknown allergen fails loudly", func(t *testing.T) {
		tx := &fakeTx{resolveIDs: map[string]int64{}}
		err := applyArtifact(context.Background(), tx, art, false)
		assert.ErrorContains(t, err, "not in the registry")
		assert.False(t, tx.committed)
	})

	t.Run("invalid kind fails", func(t *testing.T) {
		bad := artifact{Mappings: []mapping{{IngredientID: 1, Allergens: []allergenFlag{{Name: "x", Kind: "weird"}}}}}
		err := applyArtifact(context.Background(), &fakeTx{resolveIDs: map[string]int64{"x": 1}}, bad, false)
		assert.ErrorContains(t, err, "invalid kind")
	})

	t.Run("exec error wraps ingredient id", func(t *testing.T) {
		tx := &fakeTx{resolveIDs: map[string]int64{"peanuts": 10}, execErr: errors.New("deadlock")}
		single := artifact{Mappings: []mapping{{IngredientID: 7, Allergens: []allergenFlag{{Name: "peanuts", Kind: "contains"}}}}}
		err := applyArtifact(context.Background(), tx, single, false)
		assert.ErrorContains(t, err, "flag ingredient 7")
	})

	t.Run("commit error propagates", func(t *testing.T) {
		tx := &fakeTx{resolveIDs: map[string]int64{}, commitErr: errors.New("tx lost")}
		err := applyArtifact(context.Background(), tx, artifact{}, false)
		assert.Error(t, err)
	})
}

func TestUsageAndOpenDB(t *testing.T) {
	usage() // prints to stderr; just needs to not panic

	// A malformed database URL fails pool construction, not the process.
	t.Setenv("LENA_DATABASE_URL", "bogus://nope")
	_, err := openDB(context.Background())
	assert.Error(t, err)
}
