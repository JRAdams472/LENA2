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
		var a struct {
			Limit int `json:"limit"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &a); err != nil {
				return nil, fmt.Errorf("get_household_tastes args: %w", err)
			}
		}
		limit := a.Limit
		if limit <= 0 {
			limit = 10
		}
		if limit > 25 {
			limit = 25
		}

		out := TastesOut{HouseholdTop: []tasteRow{}, UserTop: []tasteRow{}}
		ids := map[int64]bool{}

		if scope.HouseholdID != 0 {
			usage, err := stats.HouseholdRecipeUsage(ctx, scope.HouseholdID)
			if err != nil {
				return nil, fmt.Errorf("household usage: %w", err)
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

			vel, err := stats.HouseholdRecipeVelocities(ctx, scope.HouseholdID, 30)
			if err != nil {
				return nil, fmt.Errorf("household velocity: %w", err)
			}
			sort.Slice(vel, func(i, j int) bool { return vel[i].RecentCount > vel[j].RecentCount })
			n := limit
			if len(vel) < n {
				n = len(vel)
			}
			for _, v := range vel[:n] {
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
		}

		if scope.UserID != 0 {
			user, err := stats.TopUserSelections(ctx, scope.UserID, analytics.EntityRecipe, int32(limit))
			if err != nil {
				return nil, fmt.Errorf("user selections: %w", err)
			}
			for _, c := range user {
				out.UserTop = append(out.UserTop, tasteRow{RecipeID: c.EntityID, Picks: c.SelectCount})
				ids[c.EntityID] = true
			}
		}

		// Resolve recipe names for readable prompts.
		if len(ids) > 0 && recipes != nil {
			iids := make([]int64, 0, len(ids))
			for id := range ids {
				iids = append(iids, id)
			}
			list, err := recipes.GetRecipesByIDs(ctx, iids)
			if err != nil {
				return nil, fmt.Errorf("recipe names: %w", err)
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
		}
		return out, nil
	})
}
