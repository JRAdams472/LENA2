package bff

import (
	"context"
	"strconv"

	"github.com/graph-gophers/graphql-go"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

// canonicalView reads the items/steps `view` arg — "canonical" bypasses
// the household delta, "effective" (the schema default) serves it.
func canonicalView(view string) (bool, error) {
	switch view {
	case "", "effective":
		return false, nil
	case "canonical":
		return true, nil
	default:
		return false, badInputf("invalid recipe view %q", view)
	}
}

// deltaFor returns the household's delta for this recipe — from the
// preloaded children when available, else a single-recipe load. The lazy
// path is what makes delta application reach the AI lazy-load resolvers.
func (r *recipeResolver) deltaFor(ctx context.Context) (*recipe.RecipeDelta, error) {
	if r.rc != nil {
		return r.rc.deltas[r.recipe.RecipeID], nil
	}
	return r.rec.GetRecipeDelta(ctx, r.recipe.RecipeID, r.user.HouseholdID)
}

// lazyItems loads items when rc missed the preload — delta-applied unless
// the caller asked for the canonical view.
func (r *recipeResolver) lazyItems(ctx context.Context, canonical bool) ([]recipe.RecipeItem, error) {
	items, err := r.rec.ListRecipeItems(ctx, r.recipe.RecipeID)
	if err != nil || canonical {
		return items, err
	}
	d, err := r.deltaFor(ctx)
	if err != nil || d == nil {
		return items, err
	}
	return recipe.ApplyDelta(items, nil, d).Items, nil
}

// lazySteps mirrors lazyItems for the step list.
func (r *recipeResolver) lazySteps(ctx context.Context, canonical bool) ([]recipe.RecipeStep, error) {
	steps, err := r.rec.ListRecipeSteps(ctx, r.recipe.RecipeID)
	if err != nil || canonical {
		return steps, err
	}
	d, err := r.deltaFor(ctx)
	if err != nil || d == nil {
		return steps, err
	}
	return recipe.ApplyDelta(nil, steps, d).Steps, nil
}

// HouseholdDelta resolves the household's tweak set for this recipe —
// null when the recipe is untouched canonical for the household.
func (r *recipeResolver) HouseholdDelta(ctx context.Context) (*recipeDeltaResolver, error) {
	d, err := r.deltaFor(ctx)
	if err != nil || d == nil {
		return nil, err
	}
	return &recipeDeltaResolver{delta: d, rec: r.recipe, rc: r.rc, inv: r.inv, as: r.as}, nil
}

// recipeDeltaResolver resolves RecipeDelta fields.
type recipeDeltaResolver struct {
	delta *recipe.RecipeDelta
	rec   recipe.Recipe
	rc    *recipeChildren
	inv   ItemReader
	as    *allergySource
}

func (d *recipeDeltaResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(d.delta.RecipeDeltaID, 10))
}

// Stale fires when the canonical recipe was edited after the delta was
// last written or acknowledged.
func (d *recipeDeltaResolver) Stale() bool {
	return d.delta.Stale(d.rec.UpdatedAt)
}

func (d *recipeDeltaResolver) OrphanedItemCount() int32 {
	var n int32
	for _, di := range d.delta.Items {
		if di.Orphaned() || (di.Kind == recipe.DeltaItemSubstitute && di.ItemID == nil && di.IngredientID == nil) {
			n++
		}
	}
	return n
}

func (d *recipeDeltaResolver) OrphanedStepCount() int32 {
	var n int32
	for _, ds := range d.delta.Steps {
		if ds.Orphaned() {
			n++
		}
	}
	return n
}

func (d *recipeDeltaResolver) UpdatedAt() *graphql.Time {
	if d.delta.UpdatedAt == nil {
		return nil
	}
	t := graphqlTime(*d.delta.UpdatedAt)
	return &t
}

func (d *recipeDeltaResolver) Items() []*recipeDeltaItemResolver {
	out := make([]*recipeDeltaItemResolver, len(d.delta.Items))
	for i := range d.delta.Items {
		out[i] = &recipeDeltaItemResolver{d: d.delta.Items[i], rc: d.rc, inv: d.inv, as: d.as}
	}
	return out
}

func (d *recipeDeltaResolver) Steps() []*recipeDeltaStepResolver {
	out := make([]*recipeDeltaStepResolver, len(d.delta.Steps))
	for i := range d.delta.Steps {
		out[i] = &recipeDeltaStepResolver{d: d.delta.Steps[i]}
	}
	return out
}

// recipeDeltaItemResolver resolves RecipeDeltaItem fields. Item/ingredient
// refs resolve from the batch-loaded maps (delta refs already flow into
// the load via effective lines); orphan refs may be absent and resolve
// null.
type recipeDeltaItemResolver struct {
	d   recipe.DeltaItem
	rc  *recipeChildren
	inv ItemReader
	as  *allergySource
}

func (r *recipeDeltaItemResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.d.DeltaItemID, 10))
}

func (r *recipeDeltaItemResolver) RecipeItemID() *graphql.ID {
	if r.d.RecipeItemID == nil {
		return nil
	}
	id := graphql.ID(strconv.FormatInt(*r.d.RecipeItemID, 10))
	return &id
}

func (r *recipeDeltaItemResolver) Kind() string { return r.d.Kind }

func (r *recipeDeltaItemResolver) Item(ctx context.Context) (*itemResolver, error) {
	if r.d.ItemID == nil {
		return nil, nil
	}
	if r.rc != nil && r.rc.items != nil {
		if it, ok := r.rc.items[*r.d.ItemID]; ok {
			return &itemResolver{inv: r.inv, it: it, ch: r.rc.itemChildren, as: firstSource(asOfRecipeChildren(r.rc), r.as)}, nil
		}
		return nil, nil
	}
	it, err := r.inv.GetItemByID(ctx, *r.d.ItemID)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.inv, it: it, as: r.as}, nil
}

func (r *recipeDeltaItemResolver) Ingredient(ctx context.Context) (*ingredientResolver, error) {
	if r.d.IngredientID == nil {
		return nil, nil
	}
	if r.rc != nil && r.rc.ingredients != nil {
		if in, ok := r.rc.ingredients[*r.d.IngredientID]; ok {
			return &ingredientResolver{inv: r.inv, in: in, as: firstSource(asOfRecipeChildren(r.rc), r.as)}, nil
		}
		return nil, nil
	}
	in, err := r.inv.GetIngredientByID(ctx, *r.d.IngredientID)
	if err != nil {
		return nil, err
	}
	return &ingredientResolver{inv: r.inv, in: in, as: r.as}, nil
}

func (r *recipeDeltaItemResolver) Quantity() *float64 { return r.d.Quantity }

func (r *recipeDeltaItemResolver) UnitID() *graphql.ID {
	if r.d.UnitID == nil {
		return nil
	}
	id := graphql.ID(strconv.FormatInt(*r.d.UnitID, 10))
	return &id
}

func (r *recipeDeltaItemResolver) Unit(ctx context.Context) (*string, error) {
	var units map[int64]inventory.Unit
	if r.rc != nil {
		units = r.rc.units
	}
	return unitNamePtr(ctx, r.inv, units, r.d.UnitID)
}

func (r *recipeDeltaItemResolver) Section() *string     { return r.d.SectionName }
func (r *recipeDeltaItemResolver) DisplayOrder() *int32 { return r.d.DisplayOrder }
func (r *recipeDeltaItemResolver) Notes() *string       { return r.d.Notes }
func (r *recipeDeltaItemResolver) IsOptional() *bool    { return r.d.IsOptional }
func (r *recipeDeltaItemResolver) Orphaned() bool {
	return r.d.Orphaned() || (r.d.Kind == recipe.DeltaItemSubstitute && r.d.ItemID == nil && r.d.IngredientID == nil)
}

// recipeDeltaStepResolver resolves RecipeDeltaStep fields.
type recipeDeltaStepResolver struct{ d recipe.DeltaStep }

func (r *recipeDeltaStepResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.d.DeltaStepID, 10))
}

func (r *recipeDeltaStepResolver) StepID() *graphql.ID {
	if r.d.StepID == nil {
		return nil
	}
	id := graphql.ID(strconv.FormatInt(*r.d.StepID, 10))
	return &id
}

func (r *recipeDeltaStepResolver) Kind() string                { return r.d.Kind }
func (r *recipeDeltaStepResolver) StepNumber() *int32          { return r.d.StepNumber }
func (r *recipeDeltaStepResolver) Instruction() *string        { return r.d.Instruction }
func (r *recipeDeltaStepResolver) DurationMinutes() *int32     { return r.d.DurationMinutes }
func (r *recipeDeltaStepResolver) StepType() *string           { return r.d.StepType }
func (r *recipeDeltaStepResolver) IsPassive() *bool            { return r.d.IsPassive }
func (r *recipeDeltaStepResolver) DependsOnStepNumber() *int32 { return r.d.DependsOnStepNumber }
func (r *recipeDeltaStepResolver) Appliance() *string          { return r.d.Appliance }
func (r *recipeDeltaStepResolver) Orphaned() bool              { return r.d.Orphaned() }

// ---------- mutations (any household member) ----------

type deltaItemInput struct {
	RecipeItemID *graphql.ID
	Kind         string
	ItemID       *graphql.ID
	IngredientID *graphql.ID
	Quantity     *float64
	UnitID       *graphql.ID
	Section      *string
	DisplayOrder *int32
	Notes        *string
	IsOptional   *bool
}

type deltaStepInput struct {
	StepID              *graphql.ID
	Kind                string
	StepNumber          *int32
	Instruction         *string
	DurationMinutes     *int32
	StepType            *string
	IsPassive           *bool
	DependsOnStepNumber *int32
	Appliance           *string
}

func parseDeltaItems(inputs []deltaItemInput) ([]recipe.DeltaItem, error) {
	out := make([]recipe.DeltaItem, len(inputs))
	for i, in := range inputs {
		var err error
		out[i].Kind = in.Kind
		if out[i].RecipeItemID, err = parseOptID(in.RecipeItemID); err != nil {
			return nil, err
		}
		if out[i].ItemID, err = parseOptID(in.ItemID); err != nil {
			return nil, err
		}
		if out[i].IngredientID, err = parseOptID(in.IngredientID); err != nil {
			return nil, err
		}
		if out[i].UnitID, err = parseOptID(in.UnitID); err != nil {
			return nil, err
		}
		out[i].Quantity = in.Quantity
		out[i].SectionName = in.Section
		out[i].DisplayOrder = in.DisplayOrder
		out[i].Notes = in.Notes
		out[i].IsOptional = in.IsOptional
	}
	return out, nil
}

func parseDeltaSteps(inputs []deltaStepInput) ([]recipe.DeltaStep, error) {
	out := make([]recipe.DeltaStep, len(inputs))
	for i, in := range inputs {
		var err error
		out[i].Kind = in.Kind
		if out[i].StepID, err = parseOptID(in.StepID); err != nil {
			return nil, err
		}
		out[i].StepNumber = in.StepNumber
		out[i].Instruction = in.Instruction
		out[i].DurationMinutes = in.DurationMinutes
		out[i].StepType = in.StepType
		out[i].IsPassive = in.IsPassive
		out[i].DependsOnStepNumber = in.DependsOnStepNumber
		out[i].Appliance = in.Appliance
	}
	return out, nil
}

func parseOptID(id *graphql.ID) (*int64, error) {
	if id == nil {
		return nil, nil
	}
	v, err := parseID(string(*id))
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// SetRecipeDelta replaces the household's delta for a recipe — the change
// set is the whole desired delta. Any household member may write.
func (r *Resolver) SetRecipeDelta(ctx context.Context, args struct {
	RecipeID graphql.ID
	Items    []deltaItemInput
	Steps    []deltaStepInput
}) (*recipeDeltaResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if u.HouseholdID == 0 {
		return nil, badInputf("no household to record a delta for")
	}
	recipeID, err := parseID(string(args.RecipeID))
	if err != nil {
		return nil, err
	}
	items, err := parseDeltaItems(args.Items)
	if err != nil {
		return nil, err
	}
	steps, err := parseDeltaSteps(args.Steps)
	if err != nil {
		return nil, err
	}
	d, err := r.RecipeService.SetRecipeDelta(ctx, recipeID, u.HouseholdID, items, steps, u.Email)
	if err != nil {
		return nil, err
	}
	return r.deltaResolverFor(ctx, u, recipeID, d)
}

// ClearRecipeDelta removes the household's delta — the recipe renders
// canonical everywhere again.
func (r *Resolver) ClearRecipeDelta(ctx context.Context, args struct {
	RecipeID graphql.ID
}) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	if u.HouseholdID == 0 {
		return false, badInputf("no household to clear a delta for")
	}
	recipeID, err := parseID(string(args.RecipeID))
	if err != nil {
		return false, err
	}
	if err := r.RecipeService.ClearRecipeDelta(ctx, recipeID, u.HouseholdID, u.Email); err != nil {
		return false, err
	}
	return true, nil
}

// AcknowledgeRecipeDelta marks the delta reviewed against the recipe's
// current version — clears the stale flag until the next canonical edit.
func (r *Resolver) AcknowledgeRecipeDelta(ctx context.Context, args struct {
	RecipeID graphql.ID
}) (*recipeDeltaResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if u.HouseholdID == 0 {
		return nil, badInputf("no household to acknowledge a delta for")
	}
	recipeID, err := parseID(string(args.RecipeID))
	if err != nil {
		return nil, err
	}
	if err := r.RecipeService.AcknowledgeRecipeDelta(ctx, recipeID, u.HouseholdID, u.Email); err != nil {
		return nil, err
	}
	d, err := r.RecipeService.GetRecipeDelta(ctx, recipeID, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	return r.deltaResolverFor(ctx, u, recipeID, *d)
}

// deltaResolverFor reloads the recipe + children so the returned delta
// carries a fresh stale flag and its refs resolve through the batch maps.
func (r *Resolver) deltaResolverFor(ctx context.Context, u currentuser.User, recipeID int64, d recipe.RecipeDelta) (*recipeDeltaResolver, error) {
	rec, err := r.RecipeService.GetRecipeByID(ctx, recipeID)
	if err != nil {
		return nil, err
	}
	rc, err := loadRecipeChildren(ctx, r.childLoaders(), u.UserID, u.HouseholdID, []int64{recipeID}, nil, nil)
	if err != nil {
		return nil, err
	}
	return &recipeDeltaResolver{delta: &d, rec: rec, rc: rc, inv: r.InventoryService, as: firstSource(asOfRecipeChildren(rc), r.allergySrc(u))}, nil
}

// ---------- delta event log (LEN-58) ----------

// recipeDeltaEventResolver resolves RecipeDeltaEvent fields.
type recipeDeltaEventResolver struct {
	ev recipe.RecipeDeltaEvent
}

func (e *recipeDeltaEventResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(e.ev.RecipeDeltaEventID, 10))
}

func (e *recipeDeltaEventResolver) Event() string { return e.ev.Event }
func (e *recipeDeltaEventResolver) Actor() string { return e.ev.Actor }

//nolint:gosec // tweak-set sizes are bounded well below int32
func (e *recipeDeltaEventResolver) ItemCount() int32 { return int32(e.ev.ItemCount) }

//nolint:gosec // tweak-set sizes are bounded well below int32
func (e *recipeDeltaEventResolver) StepCount() int32 { return int32(e.ev.StepCount) }

// Detail returns the serialized {"before","after"} change-set snapshots.
func (e *recipeDeltaEventResolver) Detail() string { return string(e.ev.Detail) }

func (e *recipeDeltaEventResolver) CreatedAt() graphql.Time {
	return graphqlTime(e.ev.CreatedAt)
}

// RecipeDeltaEvents lists the household's tweak history for a recipe,
// newest first.
func (r *Resolver) RecipeDeltaEvents(ctx context.Context, args struct {
	RecipeID graphql.ID
	Limit    int32
}) ([]*recipeDeltaEventResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if u.HouseholdID == 0 {
		return nil, badInputf("no household to read delta events for")
	}
	recipeID, err := parseID(string(args.RecipeID))
	if err != nil {
		return nil, err
	}
	events, err := r.RecipeService.ListRecipeDeltaEvents(ctx, recipeID, u.HouseholdID, args.Limit)
	if err != nil {
		return nil, err
	}
	out := make([]*recipeDeltaEventResolver, len(events))
	for i := range events {
		out[i] = &recipeDeltaEventResolver{ev: events[i]}
	}
	return out, nil
}
