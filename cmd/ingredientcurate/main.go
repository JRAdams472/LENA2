// ingredientcurate drives the generic-ingredient curation pass described
// in docs/ingredient-layer-plan.md.
//
//	propose  — asks the configured LLM to map branded catalog items onto
//	           generic ingredients and writes a reviewable JSON artifact.
//	           Nothing is written to the database.
//	apply    — reads the reviewed artifact and applies it deterministically:
//	           get-or-create each ingredient by normalized name, link every
//	           mapped item, then backfill recipe_item / event_recipe_item
//	           ingredient_id from the item links.
//
// Database access uses the app's own config (LENA_DATABASE_URL); the LLM
// uses LENA_OLLAMA_URL / LENA_OLLAMA_MODEL like the API does.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JRAdams472/LENA2/internal/platform/config"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
)

// querier is the read side of a pgx pool, abstracted so tests can fake
// the catalog queries without a database. *pgxpool.Pool satisfies it.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// txRunner is the part of pgx.Tx apply uses; *pgx.Tx satisfies it.
type txRunner interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// mapping is one proposed item -> ingredient link in the artifact.
type mapping struct {
	ItemID        int64  `json:"item_id"`
	ItemName      string `json:"item_name"`
	Ingredient    string `json:"ingredient"`
	NewIngredient bool   `json:"new_ingredient"`
	Confidence    string `json:"confidence"` // high|medium|low
	Notes         string `json:"notes,omitempty"`
}

// unresolved records an item the model declined to map.
type unresolved struct {
	ItemID   int64  `json:"item_id"`
	ItemName string `json:"item_name"`
	Reason   string `json:"reason"`
}

// artifact is the reviewable output of propose and the input to apply.
type artifact struct {
	GeneratedAt time.Time    `json:"generated_at"`
	Model       string       `json:"model"`
	Mappings    []mapping    `json:"mappings"`
	Unresolved  []unresolved `json:"unresolved,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "propose":
		err = runPropose(os.Args[2:])
	case "apply":
		err = runApply(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		slog.Error("ingredientcurate failed", "error", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  ingredientcurate propose [-out docs/ingredient-curation.json] [-limit N] [-batch N]
  ingredientcurate apply   [-in  docs/ingredient-curation.json] [-dry-run]`)
}

func openDB(ctx context.Context) (*pgxpool.Pool, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return pgxpool.New(ctx, cfg.DatabaseURL)
}

// ---------- propose ----------

type itemRow struct {
	ID   int64
	Name string
}

func runPropose(argv []string) error {
	fs := flag.NewFlagSet("propose", flag.ContinueOnError)
	out := fs.String("out", "docs/ingredient-curation.json", "artifact path to write")
	limit := fs.Int("limit", 0, "cap on items sent to the model (0 = all unmapped recipe items)")
	batch := fs.Int("batch", 30, "items per LLM call")
	all := fs.Bool("all", false, "propose for every approved catalog item lacking a link, not just recipe-used ones")
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.OllamaURL == "" {
		return errors.New("LENA_OLLAMA_URL is not set — propose needs a model")
	}
	prov := llm.NewOllamaProvider(llm.Params{
		URL:         cfg.OllamaURL,
		Model:       cfg.OllamaModel,
		Temperature: cfg.OllamaTemperature,
		NumCtx:      cfg.OllamaNumCtx,
	})

	pool, err := openDB(ctx)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	ingredients, err := ingredientNames(ctx, pool)
	if err != nil {
		return err
	}
	items, err := unmappedItems(ctx, pool, *all, *limit)
	if err != nil {
		return err
	}
	slog.Info("proposing mappings", "items", len(items), "known_ingredients", len(ingredients), "model", cfg.OllamaModel)

	art, err := proposeAll(ctx, prov, ingredients, items, *batch, cfg.OllamaModel)
	if err != nil {
		return err
	}
	return writeArtifact(*out, art)
}

// proposeAll runs the batch loop over the unmapped items and returns the
// sorted artifact.
func proposeAll(ctx context.Context, prov llm.Provider, ingredients []string, items []itemRow, batch int, model string) (artifact, error) {
	art := artifact{GeneratedAt: time.Now().UTC(), Model: model}
	for i := 0; i < len(items); i += batch {
		end := i + batch
		if end > len(items) {
			end = len(items)
		}
		ms, us, err := proposeBatch(ctx, prov, ingredients, items[i:end])
		if err != nil {
			return art, fmt.Errorf("batch %d-%d: %w", i, end, err)
		}
		art.Mappings = append(art.Mappings, ms...)
		art.Unresolved = append(art.Unresolved, us...)
		slog.Info("batch complete", "range", fmt.Sprintf("%d-%d", i, end), "mapped", len(art.Mappings), "unresolved", len(art.Unresolved))
	}
	sort.Slice(art.Mappings, func(i, j int) bool { return art.Mappings[i].ItemName < art.Mappings[j].ItemName })
	return art, nil
}

func writeArtifact(path string, art artifact) error {
	raw, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	slog.Info("artifact written", "path", path, "mappings", len(art.Mappings), "unresolved", len(art.Unresolved))
	return nil
}

func ingredientNames(ctx context.Context, pool querier) ([]string, error) {
	rows, err := pool.Query(ctx, `SELECT name FROM inventory.ingredient WHERE is_active ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func unmappedItems(ctx context.Context, pool querier, all bool, limit int) ([]itemRow, error) {
	// Recipe-used items are the only ones that must be mapped for the
	// backfill; -all widens the sweep to the whole approved catalog.
	q := `
		SELECT DISTINCT i.item_id, i.name
		FROM inventory.item i
		WHERE i.ingredient_id IS NULL
		  AND i.status = 'approved'`
	if !all {
		q += `
		  AND i.item_id IN (SELECT DISTINCT item_id FROM recipe.recipe_item WHERE item_id IS NOT NULL)`
	}
	q += ` ORDER BY i.name`
	if limit > 0 {
		q += fmt.Sprintf(` LIMIT %d`, limit)
	}
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []itemRow
	for rows.Next() {
		var r itemRow
		if err := rows.Scan(&r.ID, &r.Name); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func proposeBatch(ctx context.Context, prov llm.Provider, ingredients []string, items []itemRow) ([]mapping, []unresolved, error) {
	var sb strings.Builder
	for _, it := range items {
		fmt.Fprintf(&sb, "- %d: %s\n", it.ID, it.Name)
	}
	prompt := fmt.Sprintf(`You are mapping branded grocery products to generic (unbranded) ingredients.

Known generic ingredients (prefer these when one fits):
%s

Products to map (id: name):
%s
For each product, choose the single generic ingredient it IS (not what it
contains — "Green Giant whole kernel corn" maps to "corn", not "salt").
If no known ingredient fits, propose a new one as a lowercase singular-ish
grocery name ("corn", "marinara sauce", "chicken breast"). If the product
is not a food ingredient (blends, mixes of several foods, non-food), mark
it unresolved.

Reply ONLY with JSON: {"mappings":[{"item_id":0,"ingredient":"name","confidence":"high|medium|low","notes":"why"}],"unresolved":[{"item_id":0,"reason":"why"}]}`,
		strings.Join(ingredients, ", "), sb.String())

	resp, err := prov.Chat(ctx, llm.Request{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		JSONMode: true,
	})
	if err != nil {
		return nil, nil, err
	}
	var parsed struct {
		Mappings []struct {
			ItemID     int64  `json:"item_id"`
			Ingredient string `json:"ingredient"`
			Confidence string `json:"confidence"`
			Notes      string `json:"notes"`
		} `json:"mappings"`
		Unresolved []struct {
			ItemID int64  `json:"item_id"`
			Reason string `json:"reason"`
		} `json:"unresolved"`
	}
	if err := json.Unmarshal([]byte(resp.Message.Content), &parsed); err != nil {
		return nil, nil, fmt.Errorf("parse model reply: %w", err)
	}
	known := make(map[string]bool, len(ingredients))
	for _, n := range ingredients {
		known[n] = true
	}
	names := make(map[int64]string, len(items))
	for _, it := range items {
		names[it.ID] = it.Name
	}
	var ms []mapping
	seen := make(map[int64]bool)
	for _, m := range parsed.Mappings {
		name := strings.ToLower(strings.TrimSpace(m.Ingredient))
		if name == "" || seen[m.ItemID] {
			continue
		}
		seen[m.ItemID] = true
		ms = append(ms, mapping{
			ItemID:        m.ItemID,
			ItemName:      names[m.ItemID],
			Ingredient:    name,
			NewIngredient: !known[name],
			Confidence:    m.Confidence,
			Notes:         m.Notes,
		})
	}
	var us []unresolved
	for _, u := range parsed.Unresolved {
		if seen[u.ItemID] {
			continue
		}
		us = append(us, unresolved{ItemID: u.ItemID, ItemName: names[u.ItemID], Reason: u.Reason})
	}
	return ms, us, nil
}

// ---------- apply ----------

func runApply(argv []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	in := fs.String("in", "docs/ingredient-curation.json", "reviewed artifact to apply")
	dryRun := fs.Bool("dry-run", false, "report what would change without writing")
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	ctx := context.Background()

	raw, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	var art artifact
	if err := json.Unmarshal(raw, &art); err != nil {
		return fmt.Errorf("read artifact: %w", err)
	}
	slog.Info("applying artifact", "generated", art.GeneratedAt, "model", art.Model, "mappings", len(art.Mappings))

	pool, err := openDB(ctx)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	return applyArtifact(ctx, tx, art, *dryRun)
}

// applyArtifact resolves ingredients, links items, backfills recipe and
// event rows, then commits — or rolls back on a dry run.
func applyArtifact(ctx context.Context, tx txRunner, art artifact, dryRun bool) error {
	// Resolve every distinct ingredient name once; creation is deduped by
	// the normalized unique index, so re-running apply is idempotent.
	ingID := make(map[string]int64)
	for _, m := range art.Mappings {
		if _, ok := ingID[m.Ingredient]; ok {
			continue
		}
		var id int64
		err := tx.QueryRow(ctx, `
			WITH ins AS (
				INSERT INTO inventory.ingredient (name, is_active, created_by)
				VALUES ($1, TRUE, 'ingredientcurate')
				ON CONFLICT DO NOTHING
				RETURNING ingredient_id
			)
			SELECT ingredient_id FROM ins
			UNION ALL
			SELECT ingredient_id FROM inventory.ingredient
			WHERE lower(btrim(regexp_replace(name, '\s+', ' ', 'g'))) =
			      lower(btrim(regexp_replace($1, '\s+', ' ', 'g')))
			LIMIT 1`, m.Ingredient).Scan(&id)
		if err != nil {
			return fmt.Errorf("resolve ingredient %q: %w", m.Ingredient, err)
		}
		ingID[m.Ingredient] = id
	}

	linked := 0
	for _, m := range art.Mappings {
		id, ok := ingID[m.Ingredient]
		if !ok {
			return fmt.Errorf("mapping for item %d references unresolvable ingredient %q", m.ItemID, m.Ingredient)
		}
		// Deterministic, idempotent: only writes when the link differs.
		tag, err := tx.Exec(ctx, `
			UPDATE inventory.item
			SET ingredient_id = $2, updated_by = 'ingredientcurate', updated_at = now()
			WHERE item_id = $1 AND ingredient_id IS DISTINCT FROM $2`, m.ItemID, id)
		if err != nil {
			return fmt.Errorf("link item %d: %w", m.ItemID, err)
		}
		linked += int(tag.RowsAffected())
	}

	// Backfill references from the item links — ingredient-only hint rows
	// and already-populated rows are untouched.
	tag, err := tx.Exec(ctx, `
		UPDATE recipe.recipe_item ri
		SET ingredient_id = i.ingredient_id
		FROM inventory.item i
		WHERE ri.item_id = i.item_id
		  AND ri.ingredient_id IS NULL
		  AND i.ingredient_id IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("backfill recipe_item: %w", err)
	}
	recipeFilled := tag.RowsAffected()

	tag, err = tx.Exec(ctx, `
		UPDATE event.event_recipe_item e
		SET ingredient_id = i.ingredient_id
		FROM inventory.item i
		WHERE e.item_id = i.item_id
		  AND e.ingredient_id IS NULL
		  AND i.ingredient_id IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("backfill event_recipe_item: %w", err)
	}
	eventFilled := tag.RowsAffected()

	slog.Info("apply complete",
		"dry_run", dryRun,
		"ingredients_resolved", len(ingID),
		"item_links_written", linked,
		"recipe_items_backfilled", recipeFilled,
		"event_recipe_items_backfilled", eventFilled)

	if dryRun {
		return tx.Rollback(ctx)
	}
	return tx.Commit(ctx)
}
