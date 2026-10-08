// allergencurate drives the ingredient -> allergen seeding pass described
// in the LEN-16 allergy plan.
//
//	propose  — asks the configured LLM which registry allergens each
//	           generic ingredient contains (or may contain) and writes a
//	           reviewable JSON artifact. Nothing is written to the
//	           database.
//	apply    — reads the reviewed artifact and applies it
//	           deterministically: resolves allergen names against the
//	           seeded registry and upserts inventory.ingredient_allergen
//	           rows. Unknown allergen names fail loudly — the registry is
//	           admin-managed, so review must add them first.
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

// allergenFlag is one proposed allergen on an ingredient.
type allergenFlag struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"` // contains|may_contain
	NewAllergen bool   `json:"new_allergen,omitempty"`
}

// mapping is one proposed ingredient -> allergen set in the artifact.
type mapping struct {
	IngredientID   int64          `json:"ingredient_id"`
	IngredientName string         `json:"ingredient_name"`
	Allergens      []allergenFlag `json:"allergens"`
	Confidence     string         `json:"confidence"` // high|medium|low
	Notes          string         `json:"notes,omitempty"`
}

// unresolved records an ingredient the model declined to flag.
type unresolved struct {
	IngredientID   int64  `json:"ingredient_id"`
	IngredientName string `json:"ingredient_name"`
	Reason         string `json:"reason"`
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
		slog.Error("allergencurate failed", "error", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  allergencurate propose [-out docs/allergen-curation.json] [-limit N] [-batch N] [-all]
  allergencurate apply   [-in  docs/allergen-curation.json] [-dry-run]`)
}

func openDB(ctx context.Context) (*pgxpool.Pool, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return pgxpool.New(ctx, cfg.DatabaseURL)
}

// ---------- propose ----------

type ingredientRow struct {
	ID   int64
	Name string
}

func runPropose(argv []string) error {
	fs := flag.NewFlagSet("propose", flag.ContinueOnError)
	out := fs.String("out", "docs/allergen-curation.json", "artifact path to write")
	limit := fs.Int("limit", 0, "cap on ingredients sent to the model (0 = all)")
	batch := fs.Int("batch", 30, "ingredients per LLM call")
	all := fs.Bool("all", false, "re-propose for every active ingredient, not just ones with no allergen rows yet")
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

	allergens, err := allergenNames(ctx, pool)
	if err != nil {
		return err
	}
	ingredients, err := unflaggedIngredients(ctx, pool, *all, *limit)
	if err != nil {
		return err
	}
	slog.Info("proposing allergen flags", "ingredients", len(ingredients), "known_allergens", len(allergens), "model", cfg.OllamaModel)

	art, err := proposeAll(ctx, prov, allergens, ingredients, *batch, cfg.OllamaModel)
	if err != nil {
		return err
	}
	return writeArtifact(*out, art)
}

// proposeAll runs the batch loop over the unflagged ingredients and
// returns the sorted artifact.
func proposeAll(ctx context.Context, prov llm.Provider, allergens []string, ingredients []ingredientRow, batch int, model string) (artifact, error) {
	art := artifact{GeneratedAt: time.Now().UTC(), Model: model}
	for i := 0; i < len(ingredients); i += batch {
		end := i + batch
		if end > len(ingredients) {
			end = len(ingredients)
		}
		ms, us, err := proposeBatch(ctx, prov, allergens, ingredients[i:end])
		if err != nil {
			return art, fmt.Errorf("batch %d-%d: %w", i, end, err)
		}
		art.Mappings = append(art.Mappings, ms...)
		art.Unresolved = append(art.Unresolved, us...)
		slog.Info("batch complete", "range", fmt.Sprintf("%d-%d", i, end), "mapped", len(art.Mappings), "unresolved", len(art.Unresolved))
	}
	sort.Slice(art.Mappings, func(i, j int) bool { return art.Mappings[i].IngredientName < art.Mappings[j].IngredientName })
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

func allergenNames(ctx context.Context, pool querier) ([]string, error) {
	rows, err := pool.Query(ctx, `SELECT name FROM inventory.allergen WHERE is_active ORDER BY name`)
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

// normalizeName matches the idx_allergen_name_norm expression.
func normalizeName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func unflaggedIngredients(ctx context.Context, pool querier, all bool, limit int) ([]ingredientRow, error) {
	// Ingredients with no allergen rows are the ones that must be flagged
	// for the seed; -all widens the sweep for a re-review pass.
	q := `
		SELECT i.ingredient_id, i.name
		FROM inventory.ingredient i
		WHERE i.is_active`
	if !all {
		q += `
		  AND NOT EXISTS (
			SELECT 1 FROM inventory.ingredient_allergen ia
			WHERE ia.ingredient_id = i.ingredient_id)`
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
	var out []ingredientRow
	for rows.Next() {
		var r ingredientRow
		if err := rows.Scan(&r.ID, &r.Name); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func proposeBatch(ctx context.Context, prov llm.Provider, allergens []string, ingredients []ingredientRow) ([]mapping, []unresolved, error) {
	var sb strings.Builder
	for _, it := range ingredients {
		fmt.Fprintf(&sb, "- %d: %s\n", it.ID, it.Name)
	}
	prompt := fmt.Sprintf(`You are flagging generic food ingredients for allergens.

Known allergens (use these exact names; propose a new name only when the
ingredient carries a real allergen missing from the list):
%s

Ingredients to flag (id: name):
%s
For each ingredient, list every allergen it is an inherent source of
(kind "contains") — e.g. "peanut butter" contains "peanuts", "soy sauce"
contains "soy" and "wheat", "worcestershire sauce" contains "fish".
Use kind "may_contain" only for typical cross-contact risk (e.g.
processed oats and "gluten"). Hidden sources matter: flag sauces,
mixes, and compound ingredients for what is actually in them, not just
their namesake. When in doubt, include the flag — over-warning is
acceptable, under-warning is not.

If the ingredient has no plausible allergen, return it in "unresolved"
with reason "no allergens" so the review can confirm it was evaluated.

Reply ONLY with JSON: {"mappings":[{"ingredient_id":0,"allergens":[{"name":"allergen","kind":"contains|may_contain"}],"confidence":"high|medium|low","notes":"why"}],"unresolved":[{"ingredient_id":0,"reason":"why"}]}`,
		strings.Join(allergens, ", "), sb.String())

	resp, err := prov.Chat(ctx, llm.Request{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		JSONMode: true,
	})
	if err != nil {
		return nil, nil, err
	}
	return parseBatchReply(resp.Message.Content, allergens, ingredients)
}

// batchReply is the model's JSON response for one propose batch.
type batchReply struct {
	Mappings []struct {
		IngredientID int64 `json:"ingredient_id"`
		Allergens    []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"allergens"`
		Confidence string `json:"confidence"`
		Notes      string `json:"notes"`
	} `json:"mappings"`
	Unresolved []struct {
		IngredientID int64  `json:"ingredient_id"`
		Reason       string `json:"reason"`
	} `json:"unresolved"`
}

// parseBatchReply converts the model's JSON into mappings and unresolved
// rows, deduplicating ingredients and normalizing allergen flags.
func parseBatchReply(content string, allergens []string, ingredients []ingredientRow) ([]mapping, []unresolved, error) {
	var parsed batchReply
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, nil, fmt.Errorf("parse model reply: %w", err)
	}
	known := make(map[string]bool, len(allergens))
	for _, n := range allergens {
		known[n] = true
	}
	names := make(map[int64]string, len(ingredients))
	for _, it := range ingredients {
		names[it.ID] = it.Name
	}
	var ms []mapping
	seen := make(map[int64]bool)
	for _, m := range parsed.Mappings {
		if seen[m.IngredientID] {
			continue
		}
		flags := convertFlags(m.Allergens, known)
		if len(flags) == 0 {
			continue
		}
		seen[m.IngredientID] = true
		ms = append(ms, mapping{
			IngredientID:   m.IngredientID,
			IngredientName: names[m.IngredientID],
			Allergens:      flags,
			Confidence:     m.Confidence,
			Notes:          m.Notes,
		})
	}
	var us []unresolved
	for _, u := range parsed.Unresolved {
		if seen[u.IngredientID] {
			continue
		}
		us = append(us, unresolved{IngredientID: u.IngredientID, IngredientName: names[u.IngredientID], Reason: u.Reason})
	}
	return ms, us, nil
}

// convertFlags normalizes one ingredient's allergen list, deduplicating
// by name and marking names absent from the known-allergen list.
func convertFlags(raw []struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}, known map[string]bool) []allergenFlag {
	var flags []allergenFlag
	flagSeen := make(map[string]bool)
	for _, a := range raw {
		name := normalizeName(a.Name)
		kind := normalizeName(a.Kind)
		if name == "" || flagSeen[name] {
			continue
		}
		if kind != "contains" && kind != "may_contain" {
			kind = "contains"
		}
		flagSeen[name] = true
		flags = append(flags, allergenFlag{Name: name, Kind: kind, NewAllergen: !known[name]})
	}
	return flags
}

// ---------- apply ----------

func runApply(argv []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	in := fs.String("in", "docs/allergen-curation.json", "reviewed artifact to apply")
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

// applyArtifact resolves allergen names, upserts the flag rows, and
// commits — or rolls back on a dry run.
func applyArtifact(ctx context.Context, tx txRunner, art artifact, dryRun bool) error {
	// Resolve allergen names against the seeded registry. Unknown names
	// fail loudly: the registry is admin-managed and a typo'd allergen
	// must never be silently created or dropped.
	allergenID := make(map[string]int64)
	resolve := func(name string) (int64, error) {
		if id, ok := allergenID[name]; ok {
			return id, nil
		}
		var id int64
		err := tx.QueryRow(ctx, `
			SELECT allergen_id FROM inventory.allergen
			WHERE lower(btrim(regexp_replace(name, '\s+', ' ', 'g'))) =
			      lower(btrim(regexp_replace($1, '\s+', ' ', 'g')))`, name).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("allergen %q is not in the registry — add it via admin before applying", name)
		}
		allergenID[name] = id
		return id, nil
	}

	written := 0
	for _, m := range art.Mappings {
		for _, a := range m.Allergens {
			name := normalizeName(a.Name)
			kind := normalizeName(a.Kind)
			if kind != "contains" && kind != "may_contain" {
				return fmt.Errorf("mapping for ingredient %d has invalid kind %q", m.IngredientID, a.Kind)
			}
			id, err := resolve(name)
			if err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `
				INSERT INTO inventory.ingredient_allergen
					(ingredient_id, allergen_id, kind, created_by)
				VALUES ($1, $2, $3, 'allergencurate')
				ON CONFLICT (ingredient_id, allergen_id) DO UPDATE
				SET kind = EXCLUDED.kind, updated_by = 'allergencurate', updated_at = now()
				WHERE inventory.ingredient_allergen.kind IS DISTINCT FROM EXCLUDED.kind`,
				m.IngredientID, id, kind)
			if err != nil {
				return fmt.Errorf("flag ingredient %d allergen %q: %w", m.IngredientID, name, err)
			}
			written += int(tag.RowsAffected())
		}
	}

	slog.Info("apply complete",
		"dry_run", dryRun,
		"allergens_resolved", len(allergenID),
		"rows_written", written)

	if dryRun {
		return tx.Rollback(ctx)
	}
	return tx.Commit(ctx)
}
