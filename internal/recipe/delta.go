// Package recipe — this file holds household-scoped recipe deltas:
// structured per-line/per-step tweaks layered on the shared canonical
// recipe, no forking required. One delta per (recipe, household); reads
// apply it over the canonical rows so every consumer sees the
// household's version.
package recipe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
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
		out = append(out, newAddedStep(d, steps))
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

// newAddedStep builds the effective step an add row carries.
func newAddedStep(d DeltaStep, steps []RecipeStep) RecipeStep {
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
	return st
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
		// Snapshot the outgoing set before the replace — the event row
		// records both sides of the rewrite.
		prior, err := loadDeltaSet(ctx, q, recipeID, householdID)
		if err != nil {
			return fmt.Errorf("set recipe delta: %w", domainerr.FromStorage(err))
		}
		row, err := q.UpsertRecipeDelta(ctx, sqlc.UpsertRecipeDeltaParams{
			RecipeID: recipeID, HouseholdID: householdID, CreatedBy: by,
		})
		if err != nil {
			return fmt.Errorf("upsert recipe delta: %w", err)
		}
		delta = *toRecipeDelta(row)
		if err := replaceDeltaItems(ctx, q, row.RecipeDeltaID, items, by); err != nil {
			return err
		}
		if err := replaceDeltaSteps(ctx, q, row.RecipeDeltaID, steps, by); err != nil {
			return err
		}
		var before *deltaSetSnapshot
		if prior != nil {
			before = prior.snapshot()
		}
		after := (deltaSet{items: items, steps: steps}).snapshot()
		return insertDeltaEvent(ctx, q, deltaEventParams{
			recipeID: recipeID, householdID: householdID,
			deltaID: &row.RecipeDeltaID, event: DeltaEventSet, actor: by,
			before: before, after: after,
		})
	})
	if err != nil {
		return RecipeDelta{}, err
	}
	return delta, nil
}

// replaceDeltaItems rewrites the change set's item rows inside the
// caller's transaction.
func replaceDeltaItems(ctx context.Context, q sqlc.Querier, deltaID int64, items []DeltaItem, by string) error {
	if err := q.ReplaceDeltaItems(ctx, deltaID); err != nil {
		return fmt.Errorf("replace delta items: %w", err)
	}
	for _, d := range items {
		qty, err := deltaQuantity(d.Quantity)
		if err != nil {
			return err
		}
		if _, err := q.AddDeltaItem(ctx, sqlc.AddDeltaItemParams{
			RecipeDeltaID: deltaID,
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
	return nil
}

// replaceDeltaSteps rewrites the change set's step rows inside the
// caller's transaction.
func replaceDeltaSteps(ctx context.Context, q sqlc.Querier, deltaID int64, steps []DeltaStep, by string) error {
	if err := q.ReplaceDeltaSteps(ctx, deltaID); err != nil {
		return fmt.Errorf("replace delta steps: %w", err)
	}
	for _, d := range steps {
		if _, err := q.AddDeltaStep(ctx, sqlc.AddDeltaStepParams{
			RecipeDeltaID:       deltaID,
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
}

// ClearRecipeDelta removes the household's delta for a recipe, restoring
// the canonical view everywhere.
func (s *Service) ClearRecipeDelta(ctx context.Context, recipeID, householdID int64, by string) error {
	return s.withTx(ctx, func(q sqlc.Querier) error {
		cur, err := loadDeltaSet(ctx, q, recipeID, householdID)
		if err != nil {
			return fmt.Errorf("clear recipe delta: %w", domainerr.FromStorage(err))
		}
		if cur == nil {
			return fmt.Errorf("clear recipe delta: %w", domainerr.ErrNotFound)
		}
		if err := insertDeltaEvent(ctx, q, deltaEventParams{
			recipeID: recipeID, householdID: householdID,
			deltaID: &cur.deltaID, event: DeltaEventClear, actor: by,
			before: cur.snapshot(),
		}); err != nil {
			return fmt.Errorf("clear recipe delta: %w", err)
		}
		n, err := q.DeleteRecipeDelta(ctx, sqlc.DeleteRecipeDeltaParams{
			RecipeID: recipeID, HouseholdID: householdID,
		})
		if err != nil {
			return fmt.Errorf("clear recipe delta: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("clear recipe delta: %w", domainerr.ErrNotFound)
		}
		return nil
	})
}

// AcknowledgeRecipeDelta marks the delta as reviewed against the recipe's
// current version — clears the stale flag until the next canonical edit.
func (s *Service) AcknowledgeRecipeDelta(ctx context.Context, recipeID, householdID int64, by string) error {
	return s.withTx(ctx, func(q sqlc.Querier) error {
		cur, err := loadDeltaSet(ctx, q, recipeID, householdID)
		if err != nil {
			return fmt.Errorf("acknowledge recipe delta: %w", domainerr.FromStorage(err))
		}
		if cur == nil {
			return fmt.Errorf("acknowledge recipe delta: %w", domainerr.ErrNotFound)
		}
		n, err := q.AcknowledgeRecipeDelta(ctx, sqlc.AcknowledgeRecipeDeltaParams{
			RecipeID: recipeID, HouseholdID: householdID, UpdatedBy: textOrNull(by),
		})
		if err != nil {
			return fmt.Errorf("acknowledge recipe delta: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("acknowledge recipe delta: %w", domainerr.ErrNotFound)
		}
		return insertDeltaEvent(ctx, q, deltaEventParams{
			recipeID: recipeID, householdID: householdID,
			deltaID: &cur.deltaID, event: DeltaEventAcknowledge, actor: by,
			after: cur.snapshot(),
		})
	})
}

// ---------- event log (LEN-58) ----------

// Delta event kinds — the append-only audit record of who changed a
// household tweak set and when.
const (
	DeltaEventSet         = "set"
	DeltaEventClear       = "clear"
	DeltaEventAcknowledge = "acknowledge"
)

// RecipeDeltaEvent is one audit row — a set/clear/acknowledge plus the
// serialized change-set snapshots around it. ItemCount/StepCount count
// the tweaks in the event's "after" snapshot — or "before" on a clear,
// where there is no after.
type RecipeDeltaEvent struct {
	RecipeDeltaEventID int64
	RecipeID           int64
	HouseholdID        int64
	RecipeDeltaID      *int64
	Event              string
	Actor              string
	Detail             []byte
	ItemCount          int
	StepCount          int
	CreatedAt          time.Time
}

// deltaSetSnapshot is the serialized change-set shape stored in an
// event's detail blob: {"items": [...], "steps": [...]}.
type deltaSetSnapshot struct {
	Items []deltaItemSnapshot `json:"items"`
	Steps []deltaStepSnapshot `json:"steps"`
}

type deltaItemSnapshot struct {
	DeltaItemID  int64    `json:"deltaItemId,omitempty"`
	RecipeItemID *int64   `json:"recipeItemId,omitempty"`
	Kind         string   `json:"kind"`
	ItemID       *int64   `json:"itemId,omitempty"`
	IngredientID *int64   `json:"ingredientId,omitempty"`
	Quantity     *float64 `json:"quantity,omitempty"`
	UnitID       *int64   `json:"unitId,omitempty"`
	SectionName  *string  `json:"sectionName,omitempty"`
	DisplayOrder *int32   `json:"displayOrder,omitempty"`
	Notes        *string  `json:"notes,omitempty"`
	IsOptional   *bool    `json:"isOptional,omitempty"`
}

type deltaStepSnapshot struct {
	DeltaStepID         int64   `json:"deltaStepId,omitempty"`
	StepID              *int64  `json:"stepId,omitempty"`
	Kind                string  `json:"kind"`
	StepNumber          *int32  `json:"stepNumber,omitempty"`
	Instruction         *string `json:"instruction,omitempty"`
	DurationMinutes     *int32  `json:"durationMinutes,omitempty"`
	StepType            *string `json:"stepType,omitempty"`
	IsPassive           *bool   `json:"isPassive,omitempty"`
	DependsOnStepNumber *int32  `json:"dependsOnStepNumber,omitempty"`
	Appliance           *string `json:"appliance,omitempty"`
}

// deltaEventDetail is the stored blob: {"before": snap|null, "after":
// snap|null}. Set carries both sides of the replace; clear carries the
// removed set under before; acknowledge carries the current set under
// after.
type deltaEventDetail struct {
	Before *deltaSetSnapshot `json:"before"`
	After  *deltaSetSnapshot `json:"after"`
}

// deltaSet is a loaded change set — the delta row id plus its rows.
type deltaSet struct {
	deltaID int64
	items   []DeltaItem
	steps   []DeltaStep
}

func (s deltaSet) snapshot() *deltaSetSnapshot {
	snap := &deltaSetSnapshot{
		Items: make([]deltaItemSnapshot, 0, len(s.items)),
		Steps: make([]deltaStepSnapshot, 0, len(s.steps)),
	}
	for _, d := range s.items {
		snap.Items = append(snap.Items, deltaItemSnapshot(d))
	}
	for _, d := range s.steps {
		snap.Steps = append(snap.Steps, deltaStepSnapshot(d))
	}
	return snap
}

// loadDeltaSet reads the household's delta row and its change rows inside
// the caller's transaction; nil when no delta exists.
func loadDeltaSet(ctx context.Context, q sqlc.Querier, recipeID, householdID int64) (*deltaSet, error) {
	row, err := q.GetRecipeDelta(ctx, sqlc.GetRecipeDeltaParams{
		RecipeID: recipeID, HouseholdID: householdID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	set := &deltaSet{deltaID: row.RecipeDeltaID}
	itemRows, err := q.ListDeltaItemsByDeltas(ctx, []int64{row.RecipeDeltaID})
	if err != nil {
		return nil, err
	}
	for _, r := range itemRows {
		di, err := toDeltaItem(r)
		if err != nil {
			return nil, err
		}
		set.items = append(set.items, di)
	}
	stepRows, err := q.ListDeltaStepsByDeltas(ctx, []int64{row.RecipeDeltaID})
	if err != nil {
		return nil, err
	}
	for _, r := range stepRows {
		set.steps = append(set.steps, toDeltaStep(r))
	}
	return set, nil
}

// deltaEventParams carries one audit row for insertDeltaEvent.
type deltaEventParams struct {
	recipeID, householdID int64
	deltaID               *int64
	event, actor          string
	before, after         *deltaSetSnapshot
}

// insertDeltaEvent appends one audit row inside the caller's transaction.
func insertDeltaEvent(ctx context.Context, q sqlc.Querier, p deltaEventParams) error {
	detail, err := json.Marshal(deltaEventDetail{Before: p.before, After: p.after})
	if err != nil {
		return fmt.Errorf("delta event detail: %w", err)
	}
	return q.InsertDeltaEvent(ctx, sqlc.InsertDeltaEventParams{
		RecipeID:      p.recipeID,
		HouseholdID:   p.householdID,
		RecipeDeltaID: optInt8(p.deltaID),
		Event:         p.event,
		Actor:         p.actor,
		Detail:        detail,
	})
}

// ListRecipeDeltaEvents returns the household's tweak history for a
// recipe, newest first, capped at limit.
func (s *Service) ListRecipeDeltaEvents(ctx context.Context, recipeID, householdID int64, limit int32) ([]RecipeDeltaEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.q.ListDeltaEvents(ctx, sqlc.ListDeltaEventsParams{
		RecipeID: recipeID, HouseholdID: householdID, Limit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list delta events: %w", err)
	}
	out := make([]RecipeDeltaEvent, 0, len(rows))
	for _, r := range rows {
		ev := RecipeDeltaEvent{
			RecipeDeltaEventID: r.RecipeDeltaEventID,
			RecipeID:           r.RecipeID,
			HouseholdID:        r.HouseholdID,
			RecipeDeltaID:      int8OrNil(r.RecipeDeltaID),
			Event:              r.Event,
			Actor:              r.Actor,
			Detail:             r.Detail,
			CreatedAt:          r.CreatedAt,
		}
		ev.ItemCount, ev.StepCount = deltaEventCounts(r.Detail)
		out = append(out, ev)
	}
	return out, nil
}

// deltaEventCounts extracts tweak counts from an event blob — after-side
// for set/acknowledge, before-side for a clear.
func deltaEventCounts(detail []byte) (items, steps int) {
	var d deltaEventDetail
	if err := json.Unmarshal(detail, &d); err != nil {
		return 0, 0
	}
	snap := d.After
	if snap == nil {
		snap = d.Before
	}
	if snap == nil {
		return 0, 0
	}
	return len(snap.Items), len(snap.Steps)
}

// ---------- validation ----------

func validateDeltaItems(items []DeltaItem) error {
	for i, d := range items {
		if err := validateDeltaItem(fmt.Sprintf("itemChanges[%d]", i), d); err != nil {
			return err
		}
	}
	return nil
}

func validateDeltaItem(field string, d DeltaItem) error {
	switch d.Kind {
	case DeltaItemAdd:
		return validateDeltaLineAdd(field, d)
	case DeltaItemSubstitute:
		return validateDeltaSubstitute(field, d)
	case DeltaItemAdjust:
		return validateDeltaAdjust(field, d)
	case DeltaItemRemove:
		if d.RecipeItemID == nil {
			return &domainerr.ValidationError{Field: field, Msg: "remove needs a recipe line"}
		}
		return nil
	default:
		return &domainerr.ValidationError{Field: field, Msg: fmt.Sprintf("unknown item change kind %q", d.Kind)}
	}
}

func validateDeltaLineAdd(field string, d DeltaItem) error {
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
	return nil
}

func validateDeltaSubstitute(field string, d DeltaItem) error {
	if d.RecipeItemID == nil {
		return &domainerr.ValidationError{Field: field, Msg: "substitute needs a recipe line"}
	}
	if (d.ItemID == nil) == (d.IngredientID == nil) {
		return &domainerr.ValidationError{Field: field, Msg: "substitute targets exactly one item or ingredient"}
	}
	return nil
}

func validateDeltaAdjust(field string, d DeltaItem) error {
	if d.RecipeItemID == nil {
		return &domainerr.ValidationError{Field: field, Msg: "adjust needs a recipe line"}
	}
	if d.Quantity != nil && *d.Quantity <= 0 {
		return &domainerr.ValidationError{Field: field, Msg: "quantity must be positive"}
	}
	if d.ItemID != nil || d.IngredientID != nil {
		return &domainerr.ValidationError{Field: field, Msg: "adjust cannot change the item — use substitute"}
	}
	return nil
}

func validateDeltaSteps(steps []DeltaStep) error {
	for i, d := range steps {
		if err := validateDeltaStep(fmt.Sprintf("stepChanges[%d]", i), d); err != nil {
			return err
		}
	}
	return nil
}

func validateDeltaStep(field string, d DeltaStep) error {
	switch d.Kind {
	case DeltaStepAdd:
		return validateDeltaStepAdd(field, d)
	case DeltaStepReplace:
		if d.StepID == nil {
			return &domainerr.ValidationError{Field: field, Msg: "replace needs a step"}
		}
		return nil
	case DeltaStepRemove:
		if d.StepID == nil {
			return &domainerr.ValidationError{Field: field, Msg: "remove needs a step"}
		}
		return nil
	default:
		return &domainerr.ValidationError{Field: field, Msg: fmt.Sprintf("unknown step change kind %q", d.Kind)}
	}
}

func validateDeltaStepAdd(field string, d DeltaStep) error {
	if d.StepID != nil {
		return &domainerr.ValidationError{Field: field, Msg: "added steps can't anchor a base step"}
	}
	if d.StepNumber == nil || *d.StepNumber <= 0 {
		return &domainerr.ValidationError{Field: field, Msg: "added steps need a positive step number"}
	}
	if d.Instruction == nil || *d.Instruction == "" {
		return &domainerr.ValidationError{Field: field, Msg: "added steps need an instruction"}
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
