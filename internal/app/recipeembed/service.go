// Package recipeembed maintains recipe embeddings for semantic search:
// building the canonical embed text per recipe, refreshing on save, and
// sweeping for rows that are missing or were embedded by another model.
package recipeembed

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

// maxTextIngredients bounds how many ingredient names go into the embed
// text — beyond that the tail adds noise, not signal.
const maxTextIngredients = 60

// Store is the subset of the recipe service the embedder needs.
type Store interface {
	GetRecipeByID(ctx context.Context, recipeID int64) (recipe.Recipe, error)
	ListRecipeItemsByRecipes(ctx context.Context, recipeIDs []int64) ([]recipe.RecipeItem, error)
	ListCategoriesForRecipes(ctx context.Context, recipeIDs []int64) (map[int64][]recipe.Category, error)
	SetRecipeEmbedding(ctx context.Context, recipeID int64, embedding, model string) error
	ListEmbeddingCandidates(ctx context.Context, model string, limit int32) ([]int64, error)
}

// Namer resolves catalog names: canonical ingredient names are preferred,
// falling back to item names for recipe lines without an ingredient link.
type Namer interface {
	GetIngredientsByIDs(ctx context.Context, ingredientIDs []int64) ([]inventory.Ingredient, error)
	GetItemsByIDs(ctx context.Context, itemIDs []int64) ([]inventory.Item, error)
}

// Service refreshes recipe embeddings and sweeps for stale rows.
type Service struct {
	embedder  llm.Embedder
	model     string
	store     Store
	namer     Namer
	log       *slog.Logger
	batchSize int32
	batchGap  time.Duration
	interval  time.Duration

	lifetimeCtx    context.Context
	lifetimeCancel context.CancelFunc
}

// Option tunes Service construction (tests override the defaults).
type Option func(*Service)

// WithBatchSize sets how many candidates each sweep iteration loads.
func WithBatchSize(n int32) Option { return func(s *Service) { s.batchSize = n } }

// WithBatchGap sets the delay between embedding batches.
func WithBatchGap(d time.Duration) Option { return func(s *Service) { s.batchGap = d } }

// WithInterval sets the delay between sweep passes after the catalog is clean.
func WithInterval(d time.Duration) Option { return func(s *Service) { s.interval = d } }

// WithLogger overrides the logger (tests can silence it).
func WithLogger(l *slog.Logger) Option { return func(s *Service) { s.log = l } }

// NewService wires an embedding service. The embedder/model identify which
// vectors are current; candidates with a different embedding_model refresh.
func NewService(embedder llm.Embedder, model string, store Store, namer Namer, opts ...Option) *Service {
	s := &Service{
		embedder:  embedder,
		model:     model,
		store:     store,
		namer:     namer,
		log:       slog.Default(),
		batchSize: 32,
		batchGap:  250 * time.Millisecond,
		interval:  15 * time.Minute,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Text builds the canonical document embedded for a recipe: name,
// description, ingredient names, then category names. Keep this stable —
// changing it silently changes what stored embeddings mean.
func Text(r recipe.Recipe, ingredientNames, categoryNames []string) string {
	var b strings.Builder
	b.WriteString(r.Name)
	if d := strings.TrimSpace(r.Description); d != "" {
		b.WriteString("\n")
		b.WriteString(d)
	}
	if len(ingredientNames) > 0 {
		if len(ingredientNames) > maxTextIngredients {
			ingredientNames = ingredientNames[:maxTextIngredients]
		}
		b.WriteString("\nIngredients: ")
		b.WriteString(strings.Join(ingredientNames, ", "))
	}
	if len(categoryNames) > 0 {
		b.WriteString("\nCategories: ")
		b.WriteString(strings.Join(categoryNames, ", "))
	}
	return b.String()
}

// Refresh embeds one recipe and stores the result. Missing ingredients fall
// back to their item names. Best-effort callers should log errors, not fail
// the recipe save that triggered this.
func (s *Service) Refresh(ctx context.Context, recipeID int64) error {
	r, err := s.store.GetRecipeByID(ctx, recipeID)
	if err != nil {
		return fmt.Errorf("load recipe: %w", err)
	}
	items, err := s.store.ListRecipeItemsByRecipes(ctx, []int64{recipeID})
	if err != nil {
		return fmt.Errorf("load recipe items: %w", err)
	}
	names, err := s.ingredientNames(ctx, items)
	if err != nil {
		return err
	}
	cats, err := s.store.ListCategoriesForRecipes(ctx, []int64{recipeID})
	if err != nil {
		return fmt.Errorf("load recipe categories: %w", err)
	}
	catNames := make([]string, 0, len(cats[recipeID]))
	for _, c := range cats[recipeID] {
		catNames = append(catNames, c.Name)
	}

	vecs, err := s.embedder.Embed(ctx, []string{Text(r, names, catNames)})
	if err != nil {
		return fmt.Errorf("embed recipe: %w", err)
	}
	if len(vecs) != 1 || len(vecs[0]) == 0 {
		return fmt.Errorf("embed recipe: empty vector")
	}
	if err := s.store.SetRecipeEmbedding(ctx, recipeID, llm.VectorLiteral(vecs[0]), s.model); err != nil {
		return fmt.Errorf("store embedding: %w", err)
	}
	return nil
}

// ingredientNames resolves recipe items to display names — ingredient when
// linked, item otherwise — preserving recipe order and skipping empties.
func (s *Service) ingredientNames(ctx context.Context, items []recipe.RecipeItem) ([]string, error) {
	itemIDs := map[int64]bool{}
	ingIDs := map[int64]bool{}
	for _, ri := range items {
		if ri.IngredientID != nil {
			ingIDs[*ri.IngredientID] = true
		} else {
			itemIDs[ri.ItemID] = true
		}
	}
	names := map[int64]string{}
	if ids := keys(ingIDs); len(ids) > 0 && s.namer != nil {
		list, err := s.namer.GetIngredientsByIDs(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("load ingredient names: %w", err)
		}
		for _, g := range list {
			names[g.IngredientID] = g.Name
		}
	}
	if ids := keys(itemIDs); len(ids) > 0 && s.namer != nil {
		list, err := s.namer.GetItemsByIDs(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("load item names: %w", err)
		}
		for _, it := range list {
			names[it.ItemID] = it.Name
		}
	}
	out := make([]string, 0, len(items))
	for _, ri := range items {
		var n string
		if ri.IngredientID != nil {
			n = names[*ri.IngredientID]
		} else {
			n = names[ri.ItemID]
		}
		if n != "" {
			out = append(out, n)
		}
	}
	return out, nil
}

// Sweep embeds every stale candidate once, in batches. Returns the number of
// recipes refreshed; failures are logged and skipped (the row stays stale
// for the next pass).
func (s *Service) Sweep(ctx context.Context) (int, error) {
	total := 0
	for {
		ids, err := s.store.ListEmbeddingCandidates(ctx, s.model, s.batchSize)
		if err != nil {
			return total, fmt.Errorf("list embedding candidates: %w", err)
		}
		if len(ids) == 0 {
			return total, nil
		}
		batchOK := 0
		for _, id := range ids {
			if err := s.Refresh(ctx, id); err != nil {
				s.log.Warn("recipe embed refresh failed", "recipe_id", id, "error", err)
				continue
			}
			batchOK++
			total++
		}
		if batchOK == 0 {
			// The whole batch failed (Ollama down, dim mismatch, ...) — the
			// same rows would be listed again, so stop and let the next
			// interval retry rather than spinning.
			return total, nil
		}
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		case <-time.After(s.batchGap):
		}
	}
}

// Start runs the sweep loop on a service-lifetime context — startup
// backfill, then periodic re-sweeps — until Stop or process exit.
func (s *Service) Start() {
	s.lifetimeCtx, s.lifetimeCancel = context.WithCancel(context.Background())
	go s.run(s.lifetimeCtx)
}

// Stop cancels the sweep loop. In-flight Refresh calls finish on their own
// context timeouts.
func (s *Service) Stop() {
	if s.lifetimeCancel != nil {
		s.lifetimeCancel()
	}
}

// run performs an initial sweep then re-sweeps on a fixed interval until the
// context is cancelled.
func (s *Service) run(ctx context.Context) {
	if n, err := s.Sweep(ctx); err != nil {
		if ctx.Err() == nil {
			s.log.Warn("recipe embedding sweep failed", "embedded", n, "error", err)
		}
	} else if n > 0 {
		s.log.Info("recipe embedding backfill complete", "embedded", n)
	}
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := s.Sweep(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				s.log.Warn("recipe embedding sweep failed", "embedded", n, "error", err)
			} else if n > 0 {
				s.log.Info("recipe embedding sweep refreshed stale rows", "embedded", n)
			}
		}
	}
}

func keys(m map[int64]bool) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
