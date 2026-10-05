// Package recipe — this file holds household-scoped recipe deltas:
// structured per-line/per-step tweaks layered on the shared canonical
// recipe, no forking required. One delta per (recipe, household); reads
// apply it over the canonical rows so every consumer sees the
// household's version.
package recipe

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/recipe/sqlc"
)

// Delta item kinds.
const (
	DeltaItemSubstitute = "substitute" // replace the line's item/ingredient refs
	DeltaItemAdjust     = "adjust"     // sparse patch — NULL columns keep base
	DeltaItemRemove     = "remove"     // drop the line
	DeltaItemAdd        = "add"        // new line, no base anchor
)

// Delta step kinds.
const (
	DeltaStepReplace = "replace" // sparse patch on the base step
	DeltaStepRemove  = "remove"  // drop the step
	DeltaStepAdd     = "add"     // new step at step_number
)

// RecipeDelta is a household's tweak set for one canonical recipe.
// BaseUpdatedAt snapshots the recipe's updated_at (or created_at when
// never edited) at the last delta write or acknowledge — reads compare it
// to the recipe's current updated_at to flag drift.
type RecipeDelta struct {
	RecipeDeltaID int64
	RecipeID      int64
	HouseholdID   int64
	BaseUpdatedAt time.Time
	UpdatedAt     *time.Time
	Items         []DeltaItem
	Steps         []DeltaStep
}

// Stale reports whether the canonical recipe was edited after the delta
// was last written or acknowledged.
func (d RecipeDelta) Stale(recipeUpdatedAt *time.Time) bool {
	return recipeUpdatedAt != nil && recipeUpdatedAt.After(d.BaseUpdatedAt)
}

// DeltaItem is one line change in a household delta. RecipeItemID anchors
// to a base recipe line; it is NULL for added lines and for rows orphaned
// by a canonical edit (the anchor SET NULLs when the line is deleted).
type DeltaItem struct {
	DeltaItemID  int64
	RecipeItemID *int64
	Kind         string
	ItemID       *int64
	IngredientID *int64
	Quantity     *float64
	UnitID       *int64
	SectionName  *string
	DisplayOrder *int32
	Notes        *string
	IsOptional   *bool
}

// Orphaned reports whether the row lost its base anchor — apply skips it.
func (d DeltaItem) Orphaned() bool {
	return d.Kind != DeltaItemAdd && d.RecipeItemID == nil
}

// DeltaStep is one step change in a household delta. StepID anchors to a
// base step with the same orphan semantics as DeltaItem.RecipeItemID.
type DeltaStep struct {
	DeltaStepID         int64
	StepID              *int64
	Kind                string
	StepNumber          *int32
	Instruction         *string
	DurationMinutes     *int32
	StepType            *string
	IsPassive           *bool
	DependsOnStepNumber *int32
	Appliance           *string
}

// Orphaned reports whether the row lost its base anchor — apply skips it.
func (d DeltaStep) Orphaned() bool {
	return d.Kind != DeltaStepAdd && d.StepID == nil
}

// DeltaApplyResult is the effective recipe view after applying a delta.
// Orphan counts surface rows whose anchors were deleted by a canonical
// edit — callers can flag them rather than silently dropping them.
type DeltaApplyResult struct {
	Items         []RecipeItem
	Steps         []RecipeStep
	OrphanedItems int
	OrphanedSteps int
}

// ApplyDelta layers a household delta over the canonical items/steps and
// returns the effective lists. A nil delta returns the inputs unchanged.
// Effective lines keep their base identity (RecipeItemID/StepID) plus a
// DeltaKind marker so callers can badge changed lines; added lines carry
// ID 0 and DeltaKind "add". Step numbers in the effective list are
// renumbered 1..N after merges — dependency numbers in added steps are
// interpreted against canonical numbering.
func ApplyDelta(items []RecipeItem, steps []RecipeStep, delta *RecipeDelta) DeltaApplyResult {
	if delta == nil {
		return DeltaApplyResult{Items: items, Steps: steps}
	}
	return DeltaApplyResult{
		Items:         applyItemDelta(items, delta.Items),
		Steps:         applyStepDelta(steps, delta.Steps),
		OrphanedItems: countOrphanedItems(delta.Items),
		OrphanedSteps: countOrphanedSteps(delta.Steps),
	}
}

func applyItemDelta(items []RecipeItem, changes []DeltaItem) []RecipeItem {
	byLine := make(map[int64]DeltaItem, len(changes))
	var adds []DeltaItem
	for _, d := range changes {
		if d.Orphaned() || (d.Kind == DeltaItemSubstitute && d.ItemID == nil && d.IngredientID == nil) {
			continue // orphan or degenerate substitute — counted by caller
		}
		if d.Kind == DeltaItemAdd {
			adds = append(adds, d)
			continue
		}
		byLine[*d.RecipeItemID] = d
	}

	out := make([]RecipeItem, 0, len(items)+len(adds))
	for _, it := range items {
		d, ok := byLine[it.RecipeItemID]
		if !ok {
			out = append(out, it)
			continue
		}
		switch d.Kind {
		case DeltaItemRemove:
			continue
		case DeltaItemSubstitute:
			it.ItemID = d.ItemID
			it.IngredientID = d.IngredientID
			patchRecipeItem(&it, d)
		case DeltaItemAdjust:
			patchRecipeItem(&it, d)
		}
		it.DeltaKind = d.Kind
		out = append(out, it)
	}
	for _, d := range adds {
		it := RecipeItem{
			ItemID:       d.ItemID,
			IngredientID: d.IngredientID,
			DeltaKind:    DeltaItemAdd,
		}
		if len(items) > 0 {
			it.RecipeID = items[0].RecipeID
		}
		out = append(out, it.withDeltaLineValues(d))
	}
	// Merge order: display_order drives position, stable ties keep base
	// lines before adds.
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].DisplayOrder < out[j].DisplayOrder
	})
	return out
}

// patchRecipeItem applies the non-nil columns of an adjust/substitute row.
// Empty strings clear free-text fields (NULL means "keep the base").
func patchRecipeItem(it *RecipeItem, d DeltaItem) {
	if d.Quantity != nil {
		it.Quantity = *d.Quantity
	}
	if d.UnitID != nil {
		it.UnitID = *d.UnitID
	}
	if d.SectionName != nil {
		it.SectionName = *d.SectionName
	}
	if d.DisplayOrder != nil {
		it.DisplayOrder = *d.DisplayOrder
	}
	if d.Notes != nil {
		it.Notes = *d.Notes
	}
	if d.IsOptional != nil {
		it.IsOptional = *d.IsOptional
	}
}

// withDeltaLineValues fills the columns an added line carries — the
// caller supplies a non-nil quantity/unit pair per validation.
func (it RecipeItem) withDeltaLineValues(d DeltaItem) RecipeItem {
	if d.Quantity != nil {
		it.Quantity = *d.Quantity
	}
	if d.UnitID != nil {
		it.UnitID = *d.UnitID
	}
	if d.SectionName != nil {
		it.SectionName = *d.SectionName
	}
	if d.DisplayOrder != nil {
		it.DisplayOrder = *d.DisplayOrder
	}
	if d.Notes != nil {
		it.Notes = *d.Notes
	}
	if d.IsOptional != nil {
		it.IsOptional = *d.IsOptional
	}
	return it
}

func applyStepDelta(steps []RecipeStep, changes []DeltaStep) []RecipeStep {
	byStep := make(map[int64]DeltaStep, len(changes))
	var adds []DeltaStep
	for _, d := range changes {
		if d.Orphaned() {
			continue
		}
		if d.Kind == DeltaStepAdd {
			adds = append(adds, d)
			continue
		}
		byStep[*d.StepID] = d
	}

	out := make([]RecipeStep, 0, len(steps)+len(adds))
	for _, st := range steps {
		d, ok := byStep[st.StepID]
		if !ok {
			out = append(out, st)
			continue
		}
		switch d.Kind {
		case DeltaStepRemove:
			continue
		case DeltaStepReplace:
			patchRecipeStep(&st, d)
		}
		st.DeltaKind = d.Kind
		out = append(out, st)
	}
	for _, d := range adds {
		st := RecipeStep{DeltaKind: DeltaStepAdd}
		if len(steps) > 0 {
			st.RecipeID = steps[0].RecipeID
		}
		if d.StepNumber != nil {
			st.StepNumber = *d.StepNumber
		}
		if d.Instruction != nil {
			st.Instruction = *d.Instruction
		}
		st.DurationMinutes = d.DurationMinutes
		if d.StepType != nil {
			st.StepType = *d.StepType
		}
		if d.IsPassive != nil {
			st.IsPassive = *d.IsPassive
		}
		st.DependsOnStepNumber = d.DependsOnStepNumber
		if d.Appliance != nil {
			st.Appliance = *d.Appliance
		}
		out = append(out, st)
	}
	// An added step takes its step_number slot — on ties it sorts ahead of
	// the canonical step it displaces.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].StepNumber != out[j].StepNumber {
			return out[i].StepNumber < out[j].StepNumber
		}
		return out[i].DeltaKind == DeltaStepAdd && out[j].DeltaKind != DeltaStepAdd
	})
	// Renumber sequentially — merges can produce duplicate/gapped numbers,
	// and the effective list is never written back to the canonical rows.
	for i := range out {
		out[i].StepNumber = int32(i + 1)
	}
	return out
}

// patchRecipeStep applies the non-nil columns of a replace row.
func patchRecipeStep(st *RecipeStep, d DeltaStep) {
	if d.StepNumber != nil {
		st.StepNumber = *d.StepNumber
	}
	if d.Instruction != nil {
		st.Instruction = *d.Instruction
	}
	if d.DurationMinutes != nil {
		st.DurationMinutes = d.DurationMinutes
	}
	if d.StepType != nil {
		st.StepType = *d.StepType
	}
	if d.IsPassive != nil {
		st.IsPassive = *d.IsPassive
	}
	if d.DependsOnStepNumber != nil {
		st.DependsOnStepNumber = d.DependsOnStepNumber
	}
	if d.Appliance != nil {
		st.Appliance = *d.Appliance
	}
}

func countOrphanedItems(changes []DeltaItem) int {
	n := 0
	for _, d := range changes {
		if d.Orphaned() || (d.Kind == DeltaItemSubstitute && d.ItemID == nil && d.IngredientID == nil) {
			n++
		}
	}
	return n
}

func countOrphanedSteps(changes []DeltaStep) int {
	n := 0
	for _, d := range changes {
		if d.Orphaned() {
			n++
		}
	}
	return n
}

// ---------- service ----------

// GetRecipeDelta returns the household's delta for a recipe with its item
// and step changes, or nil when none exists.
func (s *Service) GetRecipeDelta(ctx context.Context, recipeID, householdID int64) (*RecipeDelta, error) {
	deltas, err := s.ListRecipeDeltas(ctx, householdID, []int64{recipeID})
	if err != nil {
		return nil, err
	}
	return deltas[recipeID], nil
}

// ListRecipeDeltas batch-loads the household's deltas (with item and step
// changes) for a set of recipes — the preload behind every delta-aware
// read path.
func (s *Service) ListRecipeDeltas(ctx context.Context, householdID int64, recipeIDs []int64) (map[int64]*RecipeDelta, error) {
	rows, err := s.q.ListRecipeDeltas(ctx, sqlc.ListRecipeDeltasParams{
		HouseholdID: householdID, Column2: recipeIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list recipe deltas: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	deltaIDs := make([]int64, len(rows))
	out := make(map[int64]*RecipeDelta, len(rows))
	for i, r := range rows {
		deltaIDs[i] = r.RecipeDeltaID
		out[r.RecipeID] = toRecipeDelta(r)
	}
	itemRows, err := s.q.ListDeltaItemsByDeltas(ctx, deltaIDs)
	if err != nil {
		return nil, fmt.Errorf("list delta items: %w", err)
	}
	for _, r := range itemRows {
		di, err := toDeltaItem(r)
		if err != nil {
			return nil, err
		}
		if d := deltaByID(out, r.RecipeDeltaID); d != nil {
			d.Items = append(d.Items, di)
		}
	}
	stepRows, err := s.q.ListDeltaStepsByDeltas(ctx, deltaIDs)
	if err != nil {
		return nil, fmt.Errorf("list delta steps: %w", err)
	}
	for _, r := range stepRows {
		if d := deltaByID(out, r.RecipeDeltaID); d != nil {
			d.Steps = append(d.Steps, toDeltaStep(r))
		}
	}
	return out, nil
}

func deltaByID(m map[int64]*RecipeDelta, deltaID int64) *RecipeDelta {
	for _, d := range m {
		if d.RecipeDeltaID == deltaID {
			return d
		}
	}
	return nil
}

// SetRecipeDelta replaces the household's delta for a recipe in one
// transaction — the full change-set is authoritative. An empty change-set
// still records the row (and a fresh base_updated_at) so the delta
// exists; callers wanting removal use ClearRecipeDelta.
func (s *Service) SetRecipeDelta(ctx context.Context, recipeID, householdID int64, items []DeltaItem, steps []DeltaStep, by string) (RecipeDelta, error) {
	if _, err := s.q.GetRecipeByID(ctx, recipeID); err != nil {
		return RecipeDelta{}, fmt.Errorf("set recipe delta: %w", domainerr.FromStorage(err))
	}
	if err := validateDeltaItems(items); err != nil {
		return RecipeDelta{}, err
	}
	if err := validateDeltaSteps(steps); err != nil {
		return RecipeDelta{}, err
	}
	var delta RecipeDelta
	err := s.withTx(ctx, func(q sqlc.Querier) error {
		row, err := q.UpsertRecipeDelta(ctx, sqlc.UpsertRecipeDeltaParams{
			RecipeID: recipeID, HouseholdID: householdID, CreatedBy: by,
		})
		if err != nil {
			return fmt.Errorf("upsert recipe delta: %w", err)
		}
		delta = *toRecipeDelta(row)
		if err := q.ReplaceDeltaItems(ctx, row.RecipeDeltaID); err != nil {
			return fmt.Errorf("replace delta items: %w", err)
		}
		for _, d := range items {
			qty, err := deltaQuantity(d.Quantity)
			if err != nil {
				return err
			}
			if _, err := q.AddDeltaItem(ctx, sqlc.AddDeltaItemParams{
				RecipeDeltaID: row.RecipeDeltaID,
				RecipeItemID:  optInt8(d.RecipeItemID),
				Kind:          d.Kind,
				ItemID:        optInt8(d.ItemID),
				IngredientID:  optInt8(d.IngredientID),
				Quantity:      qty,
				UnitID:        optInt8(d.UnitID),
				SectionName:   optText(d.SectionName),
				DisplayOrder:  optInt4(d.DisplayOrder),
				Notes:         optText(d.Notes),
				IsOptional:    optBool(d.IsOptional),
				CreatedBy:     by,
			}); err != nil {
				return fmt.Errorf("add delta item: %w", domainerr.FromStorage(err))
			}
		}
		if err := q.ReplaceDeltaSteps(ctx, row.RecipeDeltaID); err != nil {
			return fmt.Errorf("replace delta steps: %w", err)
		}
		for _, d := range steps {
			if _, err := q.AddDeltaStep(ctx, sqlc.AddDeltaStepParams{
				RecipeDeltaID:       row.RecipeDeltaID,
				StepID:              optInt8(d.StepID),
				Kind:                d.Kind,
				StepNumber:          optInt4(d.StepNumber),
				Instruction:         optText(d.Instruction),
				DurationMinutes:     optInt4(d.DurationMinutes),
				StepType:            optText(d.StepType),
				IsPassive:           optBool(d.IsPassive),
				DependsOnStepNumber: optInt4(d.DependsOnStepNumber),
				Appliance:           optText(d.Appliance),
				CreatedBy:           by,
			}); err != nil {
				return fmt.Errorf("add delta step: %w", domainerr.FromStorage(err))
			}
		}
		return nil
	})
	if err != nil {
		return RecipeDelta{}, err
	}
	return delta, nil
}

// ClearRecipeDelta removes the household's delta for a recipe, restoring
// the canonical view everywhere.
func (s *Service) ClearRecipeDelta(ctx context.Context, recipeID, householdID int64) error {
	n, err := s.q.DeleteRecipeDelta(ctx, sqlc.DeleteRecipeDeltaParams{
		RecipeID: recipeID, HouseholdID: householdID,
	})
	if err != nil {
		return fmt.Errorf("clear recipe delta: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("clear recipe delta: %w", domainerr.ErrNotFound)
	}
	return nil
}

// AcknowledgeRecipeDelta marks the delta as reviewed against the recipe's
// current version — clears the stale flag until the next canonical edit.
func (s *Service) AcknowledgeRecipeDelta(ctx context.Context, recipeID, householdID int64, by string) error {
	n, err := s.q.AcknowledgeRecipeDelta(ctx, sqlc.AcknowledgeRecipeDeltaParams{
		RecipeID: recipeID, HouseholdID: householdID, UpdatedBy: textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("acknowledge recipe delta: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("acknowledge recipe delta: %w", domainerr.ErrNotFound)
	}
	return nil
}

// ---------- validation ----------

func validateDeltaItems(items []DeltaItem) error {
	for i, d := range items {
		field := fmt.Sprintf("itemChanges[%d]", i)
		switch d.Kind {
		case DeltaItemAdd:
			if d.RecipeItemID != nil {
				return &domainerr.ValidationError{Field: field, Msg: "added lines can't anchor a recipe line"}
			}
			if d.ItemID == nil && d.IngredientID == nil {
				return &domainerr.ValidationError{Field: field, Msg: "added lines need an item or ingredient"}
			}
			if d.Quantity == nil || *d.Quantity <= 0 {
				return &domainerr.ValidationError{Field: field, Msg: "added lines need a positive quantity"}
			}
			if d.UnitID == nil {
				return &domainerr.ValidationError{Field: field, Msg: "added lines need a unit"}
			}
		case DeltaItemSubstitute:
			if d.RecipeItemID == nil {
				return &domainerr.ValidationError{Field: field, Msg: "substitute needs a recipe line"}
			}
			if (d.ItemID == nil) == (d.IngredientID == nil) {
				return &domainerr.ValidationError{Field: field, Msg: "substitute targets exactly one item or ingredient"}
			}
		case DeltaItemAdjust:
			if d.RecipeItemID == nil {
				return &domainerr.ValidationError{Field: field, Msg: "adjust needs a recipe line"}
			}
			if d.Quantity != nil && *d.Quantity <= 0 {
				return &domainerr.ValidationError{Field: field, Msg: "quantity must be positive"}
			}
			if d.ItemID != nil || d.IngredientID != nil {
				return &domainerr.ValidationError{Field: field, Msg: "adjust cannot change the item — use substitute"}
			}
		case DeltaItemRemove:
			if d.RecipeItemID == nil {
				return &domainerr.ValidationError{Field: field, Msg: "remove needs a recipe line"}
			}
		default:
			return &domainerr.ValidationError{Field: field, Msg: fmt.Sprintf("unknown item change kind %q", d.Kind)}
		}
	}
	return nil
}

func validateDeltaSteps(steps []DeltaStep) error {
	for i, d := range steps {
		field := fmt.Sprintf("stepChanges[%d]", i)
		switch d.Kind {
		case DeltaStepAdd:
			if d.StepID != nil {
				return &domainerr.ValidationError{Field: field, Msg: "added steps can't anchor a base step"}
			}
			if d.StepNumber == nil || *d.StepNumber <= 0 {
				return &domainerr.ValidationError{Field: field, Msg: "added steps need a positive step number"}
			}
			if d.Instruction == nil || *d.Instruction == "" {
				return &domainerr.ValidationError{Field: field, Msg: "added steps need an instruction"}
			}
		case DeltaStepReplace:
			if d.StepID == nil {
				return &domainerr.ValidationError{Field: field, Msg: "replace needs a step"}
			}
		case DeltaStepRemove:
			if d.StepID == nil {
				return &domainerr.ValidationError{Field: field, Msg: "remove needs a step"}
			}
		default:
			return &domainerr.ValidationError{Field: field, Msg: fmt.Sprintf("unknown step change kind %q", d.Kind)}
		}
	}
	return nil
}

// ---------- conversion ----------

func toRecipeDelta(r sqlc.RecipeRecipeDeltum) *RecipeDelta {
	d := &RecipeDelta{
		RecipeDeltaID: r.RecipeDeltaID,
		RecipeID:      r.RecipeID,
		HouseholdID:   r.HouseholdID,
		BaseUpdatedAt: r.BaseUpdatedAt,
	}
	if r.UpdatedAt.Valid {
		v := r.UpdatedAt.Time
		d.UpdatedAt = &v
	}
	return d
}

func toDeltaItem(r sqlc.RecipeRecipeDeltaItem) (DeltaItem, error) {
	d := DeltaItem{
		DeltaItemID:  r.DeltaItemID,
		Kind:         r.Kind,
		ItemID:       int8OrNil(r.ItemID),
		IngredientID: int8OrNil(r.IngredientID),
		RecipeItemID: int8OrNil(r.RecipeItemID),
		UnitID:       int8OrNil(r.UnitID),
		DisplayOrder: int4OrNil(r.DisplayOrder),
		IsOptional:   boolOrNil(r.IsOptional),
	}
	if r.Quantity.Valid {
		v, err := r.Quantity.Float64Value()
		if err != nil {
			return DeltaItem{}, fmt.Errorf("delta item %d quantity: %w", r.DeltaItemID, err)
		}
		d.Quantity = &v.Float64
	}
	if r.SectionName.Valid {
		d.SectionName = &r.SectionName.String
	}
	if r.Notes.Valid {
		d.Notes = &r.Notes.String
	}
	return d, nil
}

func toDeltaStep(r sqlc.RecipeRecipeDeltaStep) DeltaStep {
	return DeltaStep{
		DeltaStepID:         r.DeltaStepID,
		StepID:              int8OrNil(r.StepID),
		Kind:                r.Kind,
		StepNumber:          int4OrNil(r.StepNumber),
		Instruction:         textOrNil(r.Instruction),
		DurationMinutes:     int4OrNil(r.DurationMinutes),
		StepType:            textOrNil(r.StepType),
		IsPassive:           boolOrNil(r.IsPassive),
		DependsOnStepNumber: int4OrNil(r.DependsOnStepNumber),
		Appliance:           textOrNil(r.Appliance),
	}
}

func deltaQuantity(q *float64) (pgtype.Numeric, error) {
	if q == nil {
		return pgtype.Numeric{}, nil
	}
	n, err := numericFromFloat64(*q)
	if err != nil {
		return pgtype.Numeric{}, fmt.Errorf("delta item quantity: %w", err)
	}
	return n, nil
}

func int8OrNil(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	out := v.Int64
	return &out
}

func int4OrNil(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	out := v.Int32
	return &out
}

func boolOrNil(v pgtype.Bool) *bool {
	if !v.Valid {
		return nil
	}
	out := v.Bool
	return &out
}

func textOrNil(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func optText(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func optBool(v *bool) pgtype.Bool {
	if v == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *v, Valid: true}
}
