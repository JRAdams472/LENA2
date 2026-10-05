package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/JRAdams472/LENA2/internal/ai/tools"
)

// AllergenFlagProposal is one model-proposed flag, validated against the
// recipe context and the allergen registry. Nothing is applied — the
// resolver persists proposals to the review queue.
type AllergenFlagProposal struct {
	TargetKind string `json:"targetKind"` // "ingredient" | "item"
	TargetID   int64  `json:"targetId"`
	AllergenID int64  `json:"allergenId"`
	Kind       string `json:"kind"` // "contains" | "may_contain"
	Reason     string `json:"reason"`
}

const allergenFlagSystemPrompt = `You are LENA's allergen-curation assistant. The user message contains the
allergen registry plus a recipe's ingredient lines. Each line lists the
flaggable ids — an ingredientId (the generic ingredient, household-override
aware) and/or an itemId (the branded product) — plus any allergen flags
already curated on it.

Task: propose allergen flags for lines where the name clearly implies one,
as reviewable suggestions a curator will accept or dismiss.

Rules:
- targetKind is "ingredient" or "item"; targetId is that kind's id from the line.
- Prefer ingredient flags — they apply to every brand. Use item targets
  only for product-specific additives or shared-facility risk the generic
  ingredient would not carry.
- allergenId must come from the provided registry — never invent allergens.
- kind is "contains" when the allergen is a normal component (milk in
  cheese, wheat in flour) and "may_contain" for cross-contact risk
  (shared equipment, "may contain traces").
- Skip targets already carrying that allergen flag; do not propose the
  same (target, allergen) twice.
- Only flag what the name supports — "heavy cream" implies milk; "olive
  oil" implies nothing. When unsure, do not propose.
- "reason" is one short phrase naming the line and allergen.
- Reply ONLY with a JSON object: {"flags":[{...}]} — [] if nothing applies.`

type allergenFlagResponse struct {
	Flags []AllergenFlagProposal `json:"flags"`
}

// allergenFlagsSchema constrains structured output for allergen-flag
// suggestions — served in PreparedRequest.OutputSchema.
const allergenFlagsSchema = `{"type":"object","required":["flags"],` +
	`"properties":{"flags":{"type":"array","items":{"type":"object",` +
	`"required":["targetKind","targetId","allergenId","kind","reason"],` +
	`"properties":{"targetKind":{"type":"string"},"targetId":{"type":"integer"},` +
	`"allergenId":{"type":"integer"},"kind":{"type":"string"},` +
	`"reason":{"type":"string"}}}},"additionalProperties":false}`

// SuggestAllergens runs the server-inference path for recipe allergen
// flag proposals. Returns validated proposals — persistence into the
// review queue is the caller's job.
func (s *Service) SuggestAllergens(ctx context.Context, userID, householdID, recipeID int64, maxSuggestions int) ([]AllergenFlagProposal, error) {
	if !s.Available() {
		return nil, ErrUnavailable
	}
	scope := tools.Scope{UserID: userID, HouseholdID: householdID}
	p, err := s.prepareAllergenSuggestions(ctx, scope, recipeID, maxSuggestions)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return []AllergenFlagProposal{}, nil
	}
	return runPrepared(ctx, s.provider, p)
}

// prepareAllergenSuggestions assembles the recipe's flag context through
// the read-only tool registry. Returns (nil, nil) when the recipe has no
// flaggable lines — nothing to propose means no model call. The validator
// drops unknown targets/allergens, already-flagged pairs, and duplicates.
func (s *Service) prepareAllergenSuggestions(ctx context.Context, scope tools.Scope, recipeID int64, maxSuggestions int) (*prepared[[]AllergenFlagProposal], error) {
	if maxSuggestions <= 0 {
		maxSuggestions = 8
	}
	if maxSuggestions > 20 {
		maxSuggestions = 20
	}

	out, err := s.reg.Call(ctx, scope, "get_recipe_allergen_candidates",
		jsonArgs(map[string]any{"recipeIds": []int64{recipeID}}))
	if err != nil {
		return nil, err
	}
	cand, ok := out.(*tools.AllergenCandidatesOut)
	if !ok {
		return nil, fmt.Errorf("get_recipe_allergen_candidates returned %T", out)
	}
	var rec *tools.AllergenCandidateRecipe
	for i := range cand.Recipes {
		if cand.Recipes[i].ID == recipeID {
			rec = &cand.Recipes[i]
			break
		}
	}
	if rec == nil {
		return nil, errors.New("recipe not found")
	}
	if len(rec.Items) == 0 || len(cand.Allergens) == 0 {
		return nil, nil
	}

	reqBody, err := json.Marshal(cand)
	if err != nil {
		return nil, fmt.Errorf("marshal candidates: %w", err)
	}
	limit := maxSuggestions

	return &prepared[[]AllergenFlagProposal]{
		req: PreparedRequest{
			Prompt:       allergenFlagSystemPrompt,
			Context:      reqBody,
			OutputSchema: json.RawMessage(allergenFlagsSchema),
		},
		validate: func(content string) ([]AllergenFlagProposal, error) {
			var parsed allergenFlagResponse
			if err := json.Unmarshal([]byte(content), &parsed); err != nil {
				return nil, err
			}
			return filterAllergenProposals(parsed.Flags, rec, cand, limit), nil
		},
	}, nil
}

// filterAllergenProposals keeps only proposals whose target exists in the
// context, whose allergen is registered, whose kind is valid, and which
// aren't already flagged or duplicated — the same "never trust the model"
// contract every Suggest* validator enforces.
func filterAllergenProposals(props []AllergenFlagProposal, rec *tools.AllergenCandidateRecipe, cand *tools.AllergenCandidatesOut, limit int) []AllergenFlagProposal {
	registry := map[int64]bool{}
	for _, a := range cand.Allergens {
		registry[a.ID] = true
	}
	targets := flaggedTargets(rec)

	type proposalKey struct {
		kind       string
		targetID   int64
		allergenID int64
	}
	seen := map[proposalKey]bool{}
	out := make([]AllergenFlagProposal, 0, len(props))
	for _, p := range props {
		if len(out) >= limit {
			break
		}
		if !validAllergenProposal(p, registry, targets) {
			continue
		}
		key := proposalKey{p.TargetKind, p.TargetID, p.AllergenID}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return out
}

// allergenTarget identifies one flaggable row — an ingredient or an item.
type allergenTarget struct {
	kind string
	id   int64
}

// flaggedTargets indexes each flaggable target's already-flagged allergen
// ids so a proposal duplicating an existing flag can be dropped.
func flaggedTargets(rec *tools.AllergenCandidateRecipe) map[allergenTarget]map[int64]bool {
	targets := map[allergenTarget]map[int64]bool{}
	add := func(t allergenTarget, flags []tools.AllergenFlagRow) {
		if targets[t] == nil {
			targets[t] = map[int64]bool{}
		}
		for _, f := range flags {
			targets[t][f.AllergenID] = true
		}
	}
	for _, it := range rec.Items {
		if it.IngredientID != nil {
			add(allergenTarget{"ingredient", *it.IngredientID}, it.Flags)
		}
		if it.ItemID != nil {
			add(allergenTarget{"item", *it.ItemID}, it.Flags)
		}
	}
	return targets
}

// validAllergenProposal enforces the proposal contract: known target kind,
// valid flag kind, registered allergen, and a target that exists and is
// not already flagged with that allergen.
func validAllergenProposal(p AllergenFlagProposal, registry map[int64]bool, targets map[allergenTarget]map[int64]bool) bool {
	if p.TargetKind != "ingredient" && p.TargetKind != "item" {
		return false
	}
	if p.Kind != "contains" && p.Kind != "may_contain" {
		return false
	}
	if !registry[p.AllergenID] {
		return false
	}
	flagged, exists := targets[allergenTarget{p.TargetKind, p.TargetID}]
	return exists && !flagged[p.AllergenID]
}
