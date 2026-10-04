package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/JRAdams472/LENA2/internal/analytics"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
)

// TasteProfiler is the analytics surface the tastes tool needs.
type TasteProfiler interface {
	TopUserSelections(ctx context.Context, userID int64, entityType string, limit int32) ([]analytics.SelectionCount, error)
	HouseholdRecipeUsage(ctx context.Context, householdID int64) (map[int64]int64, error)
	HouseholdRecipeVelocities(ctx context.Context, householdID int64, recentDays int32) ([]analytics.RecipeVelocity, error)
}

type tasteRow struct {
	RecipeID int64  `json:"recipeId"`
	Name     string `json:"name,omitempty"`
	Picks    int64  `json:"picks"`
}

type trendingRow struct {
	RecipeID    int64  `json:"recipeId"`
	Name        string `json:"name,omitempty"`
	RecentPicks int64  `json:"recentPicks"`
	TotalPicks  int64  `json:"totalPicks"`
}

// TastesOut summarises household and personal recipe preferences for
// prompts — who picks what, and what's accelerating recently.
type TastesOut struct {
	HouseholdTop []tasteRow    `json:"householdTop"`
	Trending     []trendingRow `json:"trending,omitempty"`
	UserTop      []tasteRow    `json:"userTop"`
}

// RegisterTasteTools wires get_household_tastes.
func RegisterTasteTools(reg *Registry, stats TasteProfiler, recipes RecipeLookup) {
	reg.Register(llm.ToolSpec{
		Name:        "get_household_tastes",
		Description: "Summarise the household's recipe preferences: most-picked recipes, recent trending recipes, and the caller's own favourites.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{
					"type":        "integer",
					"description": "Rows per section (default 10, max 25)",
				},
			},
		},
	}, func(ctx context.Context, scope Scope, args json.RawMessage) (any, error) {
		limit, err := parseLimitArg(args, "get_household_tastes", 10, 25)
		if err != nil {
			return nil, err
		}

		out := TastesOut{HouseholdTop: []tasteRow{}, UserTop: []tasteRow{}}
		ids := map[int64]bool{}

		if scope.HouseholdID != 0 {
			if err := householdTastes(ctx, stats, scope.HouseholdID, limit, &out, ids); err != nil {
				return nil, err
			}
		}
		if scope.UserID != 0 {
			if err := userTastes(ctx, stats, scope.UserID, limit, &out, ids); err != nil {
				return nil, err
			}
		}

		// Resolve recipe names for readable prompts.
		if len(ids) > 0 && recipes != nil {
			if err := resolveTasteNames(ctx, recipes, ids, &out); err != nil {
				return nil, err
			}
		}
		return out, nil
	})
}

// parseLimitArg extracts the optional integer `limit` from tool args,
// clamped to [def, max].
func parseLimitArg(args json.RawMessage, tool string, def, hi int) (int, error) {
	var a struct {
		Limit int `json:"limit"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &a); err != nil {
			return 0, fmt.Errorf("%s args: %w", tool, err)
		}
	}
	limit := a.Limit
	if limit <= 0 {
		limit = def
	}
	if limit > hi {
		limit = hi
	}
	return limit, nil
}

// householdTastes fills HouseholdTop and Trending from household usage and
// velocity stats, recording recipe ids into ids for name resolution.
func householdTastes(ctx context.Context, stats TasteProfiler, householdID int64, limit int, out *TastesOut, ids map[int64]bool) error {
	usage, err := stats.HouseholdRecipeUsage(ctx, householdID)
	if err != nil {
		return fmt.Errorf("household usage: %w", err)
	}
	top := make([]tasteRow, 0, len(usage))
	for id, n := range usage {
		top = append(top, tasteRow{RecipeID: id, Picks: n})
	}
	sort.Slice(top, func(i, j int) bool { return top[i].Picks > top[j].Picks })
	if len(top) > limit {
		top = top[:limit]
	}
	out.HouseholdTop = top
	for _, t := range top {
		ids[t.RecipeID] = true
	}

	vel, err := stats.HouseholdRecipeVelocities(ctx, householdID, 30)
	if err != nil {
		return fmt.Errorf("household velocity: %w", err)
	}
	sort.Slice(vel, func(i, j int) bool { return vel[i].RecentCount > vel[j].RecentCount })
	for i, v := range vel {
		if i >= limit {
			break
		}
		if v.RecentCount == 0 {
			continue
		}
		out.Trending = append(out.Trending, trendingRow{
			RecipeID:    v.RecipeID,
			RecentPicks: v.RecentCount,
			TotalPicks:  v.TotalCount,
		})
		ids[v.RecipeID] = true
	}
	return nil
}

// userTastes fills UserTop from the caller's own selection stats.
func userTastes(ctx context.Context, stats TasteProfiler, userID int64, limit int, out *TastesOut, ids map[int64]bool) error {
	//nolint:gosec // limit is clamped to <= 25 by parseLimitArg.
	user, err := stats.TopUserSelections(ctx, userID, analytics.EntityRecipe, int32(limit))
	if err != nil {
		return fmt.Errorf("user selections: %w", err)
	}
	for _, c := range user {
		out.UserTop = append(out.UserTop, tasteRow{RecipeID: c.EntityID, Picks: c.SelectCount})
		ids[c.EntityID] = true
	}
	return nil
}

// resolveTasteNames stamps recipe names onto every section row.
func resolveTasteNames(ctx context.Context, recipes RecipeLookup, ids map[int64]bool, out *TastesOut) error {
	iids := make([]int64, 0, len(ids))
	for id := range ids {
		iids = append(iids, id)
	}
	list, err := recipes.GetRecipesByIDs(ctx, iids)
	if err != nil {
		return fmt.Errorf("recipe names: %w", err)
	}
	names := map[int64]string{}
	for _, r := range list {
		names[r.RecipeID] = r.Name
	}
	for i := range out.HouseholdTop {
		out.HouseholdTop[i].Name = names[out.HouseholdTop[i].RecipeID]
	}
	for i := range out.Trending {
		out.Trending[i].Name = names[out.Trending[i].RecipeID]
	}
	for i := range out.UserTop {
		out.UserTop[i].Name = names[out.UserTop[i].RecipeID]
	}
	return nil
}
