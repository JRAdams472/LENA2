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
	items := []itemRow{
		{ID: 1, Name: "Green Giant Whole Kernel Corn"},
		{ID: 2, Name: "Kraft Macaroni & Cheese"},
		{ID: 3, Name: "Mystery Blend"},
	}
	known := []string{"corn", "flour", "sugar"}

	t.Run("maps, flags new ingredients, and collects unresolved", func(t *testing.T) {
		prov := llm.NewMockProvider()
		prov.Handler = func(req llm.Request) (llm.Response, error) {
			assert.True(t, req.JSONMode, "propose must request JSON output")
			assert.Contains(t, req.Messages[0].Content, "Green Giant Whole Kernel Corn")
			assert.Contains(t, req.Messages[0].Content, "corn")
			return llm.Response{Message: llm.Message{
				Content: `{"mappings":[
					{"item_id":1,"ingredient":"Corn","confidence":"high","notes":"canned corn"},
					{"item_id":2,"ingredient":"macaroni and cheese","confidence":"medium"}
				],"unresolved":[
					{"item_id":3,"reason":"mixed product, no single ingredient"}
				]}`},
			}, nil
		}

		ms, us, err := proposeBatch(context.Background(), prov, known, items)
		require.NoError(t, err)
		require.Len(t, ms, 2)
		// Known ingredient (case-insensitive normalization) — not new.
		assert.Equal(t, "corn", ms[0].Ingredient)
		assert.False(t, ms[0].NewIngredient)
		assert.Equal(t, "Green Giant Whole Kernel Corn", ms[0].ItemName)
		// Novel ingredient gets flagged for review.
		assert.True(t, ms[1].NewIngredient)
		require.Len(t, us, 1)
		assert.Equal(t, int64(3), us[0].ItemID)
		assert.Equal(t, "Mystery Blend", us[0].ItemName)
	})

	t.Run("drops duplicate and empty mappings", func(t *testing.T) {
		prov := llm.NewMockProvider()
		prov.Handler = func(llm.Request) (llm.Response, error) {
			return llm.Response{Message: llm.Message{
				Content: `{"mappings":[
					{"item_id":1,"ingredient":"corn","confidence":"high"},
					{"item_id":1,"ingredient":"flour","confidence":"low"},
					{"item_id":2,"ingredient":"  ","confidence":"high"}
				]}`},
			}, nil
		}

		ms, _, err := proposeBatch(context.Background(), prov, known, items)
		require.NoError(t, err)
		require.Len(t, ms, 1, "second mapping for the same item and empty names are dropped")
		assert.Equal(t, "corn", ms[0].Ingredient)
	})

	t.Run("propagates provider and parse errors", func(t *testing.T) {
		boom := errors.New("ollama down")
		prov := llm.NewMockProvider()
		prov.Handler = func(llm.Request) (llm.Response, error) { return llm.Response{}, boom }
		_, _, err := proposeBatch(context.Background(), prov, known, items)
		assert.ErrorIs(t, err, boom)

		prov.Handler = func(llm.Request) (llm.Response, error) {
			return llm.Response{Message: llm.Message{Content: "not json"}}, nil
		}
		_, _, err = proposeBatch(context.Background(), prov, known, items)
		assert.ErrorContains(t, err, "parse model reply")
	})
}

func TestArtifactRoundTrip(t *testing.T) {
	// The artifact schema is the review contract between propose and apply;
	// a rename breaks reviewed files silently, so pin the JSON shape.
	raw := `{
		"generated_at": "2026-10-06T00:00:00Z",
		"model": "qwen3:8b",
		"mappings": [{"item_id": 42, "item_name": "Kellogg's Corn Flakes", "ingredient": "corn flakes", "new_ingredient": true, "confidence": "medium", "notes": "cereal"}],
		"unresolved": [{"item_id": 7, "item_name": "Party Mix", "reason": "mixed product"}]
	}`
	var art artifact
	require.NoError(t, json.Unmarshal([]byte(raw), &art))
	require.Len(t, art.Mappings, 1)
	assert.Equal(t, int64(42), art.Mappings[0].ItemID)
	assert.Equal(t, "corn flakes", art.Mappings[0].Ingredient)
	assert.True(t, art.Mappings[0].NewIngredient)
	require.Len(t, art.Unresolved, 1)
	assert.Equal(t, "mixed product", art.Unresolved[0].Reason)
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

// fakeTx resolves ingredient names and records exec/commit state.
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
		id, ok := t.resolveIDs[name]
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

func TestIngredientNames(t *testing.T) {
	q := &fakeQuerier{rows: &fakeRows{rows: [][]any{{"corn"}, {"marinara sauce"}}}}
	names, err := ingredientNames(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, []string{"corn", "marinara sauce"}, names)

	boom := errors.New("db down")
	_, err = ingredientNames(context.Background(), &fakeQuerier{err: boom})
	assert.ErrorIs(t, err, boom)

	_, err = ingredientNames(context.Background(), &fakeQuerier{rows: &fakeRows{rows: [][]any{{"x"}}, err: boom}})
	assert.ErrorIs(t, err, boom)
}

func TestUnmappedItems(t *testing.T) {
	ctx := context.Background()

	q := &fakeQuerier{rows: &fakeRows{rows: [][]any{{int64(1), "Green Giant corn"}}}}
	items, err := unmappedItems(ctx, q, false, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Contains(t, q.lastSQL, "recipe.recipe_item", "default sweep is recipe-used items")
	assert.NotContains(t, q.lastSQL, "LIMIT")

	// -all widens the sweep; -limit caps the result set.
	q = &fakeQuerier{rows: &fakeRows{rows: [][]any{{int64(9), "x"}}}}
	_, err = unmappedItems(ctx, q, true, 7)
	require.NoError(t, err)
	assert.NotContains(t, q.lastSQL, "recipe.recipe_item")
	assert.Contains(t, q.lastSQL, "LIMIT 7")

	_, err = unmappedItems(ctx, &fakeQuerier{err: errors.New("db down")}, false, 0)
	assert.Error(t, err)
}

// ---------- propose orchestration ----------

func TestProposeAll(t *testing.T) {
	items := []itemRow{{ID: 1, Name: "b"}, {ID: 2, Name: "a"}, {ID: 3, Name: "c"}}

	calls := 0
	prov := llm.NewMockProvider()
	prov.Handler = func(llm.Request) (llm.Response, error) {
		calls++
		return llm.Response{Message: llm.Message{Content: `{"mappings":[{"item_id":1,"ingredient":"corn","confidence":"low"}],"unresolved":[{"item_id":9,"reason":"none"}]}`}}, nil
	}
	art, err := proposeAll(context.Background(), prov, []string{"corn"}, items, 1, "m")
	require.NoError(t, err)
	assert.Equal(t, 3, calls, "batch=1 over 3 items")
	assert.Len(t, art.Mappings, 3)
	assert.Len(t, art.Unresolved, 3)

	prov.Handler = func(llm.Request) (llm.Response, error) {
		return llm.Response{Message: llm.Message{Content: `{"mappings":[
			{"item_id":1,"ingredient":"corn","confidence":"low"},
			{"item_id":3,"ingredient":"corn","confidence":"low"},
			{"item_id":2,"ingredient":"corn","confidence":"low"}]}`}}, nil
	}
	art, err = proposeAll(context.Background(), prov, []string{"corn"}, items, 30, "m")
	require.NoError(t, err)
	require.Len(t, art.Mappings, 3)
	assert.Equal(t, "a", art.Mappings[0].ItemName, "mappings sorted by name")

	boom := errors.New("ollama down")
	prov.Handler = func(llm.Request) (llm.Response, error) { return llm.Response{}, boom }
	_, err = proposeAll(context.Background(), prov, nil, items, 30, "m")
	assert.ErrorContains(t, err, "batch 0-3")
	assert.ErrorIs(t, err, boom)
}

func TestWriteArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "art.json")
	art := artifact{Model: "m", Mappings: []mapping{{ItemID: 1, ItemName: "x", Ingredient: "corn"}}}
	require.NoError(t, writeArtifact(path, art))

	raw, err := os.ReadFile(path) // #nosec G304 -- test reads its own temp file
	require.NoError(t, err)
	var got artifact
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, int64(1), got.Mappings[0].ItemID)

	assert.Error(t, writeArtifact(filepath.Join(t.TempDir(), "no", "dir", "art.json"), art))
}

func TestRunPropose(t *testing.T) {
	assert.Error(t, runPropose([]string{"-bogus"}))

	t.Setenv("LENA_OLLAMA_URL", "")
	assert.ErrorContains(t, runPropose(nil), "LENA_OLLAMA_URL")
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
		{ItemID: 1, ItemName: "peanut butter", Ingredient: "peanut butter"},
		{ItemID: 2, ItemName: "corn flakes", Ingredient: "corn"},
		{ItemID: 3, ItemName: "corn flakes xxl", Ingredient: "corn"},
	}}

	t.Run("resolves once, links, backfills, commits", func(t *testing.T) {
		tx := &fakeTx{resolveIDs: map[string]int64{"peanut butter": 10, "corn": 11}}
		require.NoError(t, applyArtifact(context.Background(), tx, art, false))
		assert.True(t, tx.committed)
		assert.False(t, tx.rolledBack)
		// 3 item links + recipe backfill + event backfill; "corn" resolved once.
		assert.Len(t, tx.execCalls, 5)
	})

	t.Run("dry run rolls back", func(t *testing.T) {
		tx := &fakeTx{resolveIDs: map[string]int64{"peanut butter": 10, "corn": 11}}
		require.NoError(t, applyArtifact(context.Background(), tx, art, true))
		assert.True(t, tx.rolledBack)
		assert.False(t, tx.committed)
	})

	t.Run("unresolvable ingredient fails", func(t *testing.T) {
		tx := &fakeTx{resolveIDs: map[string]int64{}}
		err := applyArtifact(context.Background(), tx, art, false)
		assert.ErrorContains(t, err, "resolve ingredient")
	})

	t.Run("link exec error wraps item id", func(t *testing.T) {
		tx := &fakeTx{resolveIDs: map[string]int64{"peanut butter": 10}, execErr: errors.New("deadlock")}
		single := artifact{Mappings: []mapping{{ItemID: 7, Ingredient: "peanut butter"}}}
		err := applyArtifact(context.Background(), tx, single, false)
		assert.ErrorContains(t, err, "link item 7")
	})

	t.Run("backfill error propagates", func(t *testing.T) {
		tx := &fakeTx{resolveIDs: map[string]int64{}}
		err := applyArtifact(context.Background(), tx, artifact{}, false)
		require.NoError(t, err)
		tx.execErr = errors.New("deadlock")
		err = applyArtifact(context.Background(), tx, artifact{Mappings: []mapping{}}, false)
		assert.ErrorContains(t, err, "backfill recipe_item")
	})

	t.Run("commit error propagates", func(t *testing.T) {
		tx := &fakeTx{resolveIDs: map[string]int64{}, commitErr: errors.New("tx lost")}
		err := applyArtifact(context.Background(), tx, artifact{}, false)
		assert.Error(t, err)
	})
}

func TestUsageAndOpenDB(t *testing.T) {
	usage() // prints to stderr; just needs to not panic

	t.Setenv("LENA_DATABASE_URL", "bogus://nope")
	_, err := openDB(context.Background())
	assert.Error(t, err)
}
