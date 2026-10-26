package bff

import (
	"context"
	"log/slog"
	"strings"

	"github.com/graph-gophers/graphql-go"

	"github.com/JRAdams472/LENA2/internal/analytics"
)

// RecordSelection logs that the current user selected a catalog entity from
// a dropdown or search result. It is fire-and-forget: failures are logged
// and ignored so tracking cannot break the UI.
func (r *Resolver) RecordSelection(ctx context.Context, args struct {
	EntityType string
	EntityID   graphql.ID
}) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	entityType, err := validAnalyticsEntityType(args.EntityType)
	if err != nil {
		return false, err
	}
	entityID, err := parseID(string(args.EntityID))
	if err != nil {
		return false, err
	}
	if err := r.AnalyticsService.RecordEvent(ctx, analytics.Event{
		UserID:     u.UserID,
		EventType:  selectionEventType(entityType),
		EntityType: entityType,
		EntityID:   entityID,
	}, u.Email); err != nil {
		slog.Default().Error("record selection failed", "error", err)
	}
	return true, nil
}

// RecordSearch logs a free-text search. It is fire-and-forget: failures are
// logged and ignored so tracking cannot break the UI.
func (r *Resolver) RecordSearch(ctx context.Context, args struct {
	EntityType string
	Term       string
}) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	entityType, err := validAnalyticsEntityType(args.EntityType)
	if err != nil {
		return false, err
	}
	term := strings.TrimSpace(args.Term)
	if len(term) > maxAnalyticsTermBytes {
		return false, badInputf("search term exceeds %d bytes", maxAnalyticsTermBytes)
	}
	if err := r.AnalyticsService.RecordEvent(ctx, analytics.Event{
		UserID:     u.UserID,
		EventType:  searchEventType(entityType),
		EntityType: entityType,
		SearchTerm: term,
	}, u.Email); err != nil {
		slog.Default().Error("record search failed", "error", err)
	}
	return true, nil
}

// maxAnalyticsTermBytes bounds recorded search terms so a client can't
// stuff unbounded text into the analytics tables.
const maxAnalyticsTermBytes = 200

// validAnalyticsEntityType allowlists the analytics entity types; an
// unrecognized value previously fell through to the "item" event bucket,
// silently corrupting engagement data.
func validAnalyticsEntityType(entityType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(entityType)) {
	case analytics.EntityItem, analytics.EntityBrand, analytics.EntityRecipe,
		analytics.EntityBottle, analytics.EntityIngredient:
		return strings.ToLower(strings.TrimSpace(entityType)), nil
	default:
		return "", badInputf("unknown entity type %q", entityType)
	}
}

func selectionEventType(entityType string) string {
	switch strings.ToLower(strings.TrimSpace(entityType)) {
	case analytics.EntityBrand:
		return analytics.EventBrandSelected
	case analytics.EntityRecipe:
		return analytics.EventRecipeSelected
	case analytics.EntityBottle:
		return analytics.EventBottleSelected
	case analytics.EntityIngredient:
		return analytics.EventIngredientSelected
	default:
		return analytics.EventItemSelected
	}
}

func searchEventType(entityType string) string {
	switch strings.ToLower(strings.TrimSpace(entityType)) {
	case analytics.EntityRecipe:
		return analytics.EventRecipeSearched
	case analytics.EntityBrand:
		return analytics.EventBrandSearched
	case analytics.EntityBottle:
		return analytics.EventBottleSearched
	case analytics.EntityIngredient:
		return analytics.EventIngredientSearched
	default:
		return analytics.EventItemSearched
	}
}

// entityEngagement loads the pre-sorted ranking inputs for one entity type
// for the current user (personal → household → global decayed scores plus
// viewed IDs and prior search terms). A missing analytics service degrades
// to empty engagement — cold-start ordering rather than an error.
func (r *Resolver) entityEngagement(ctx context.Context, userID, householdID int64, entityType string) (analytics.EntityEngagement, error) {
	if r.AnalyticsService == nil {
		return analytics.EntityEngagement{}, nil
	}
	return r.AnalyticsService.EntityEngagementSets(ctx, userID, householdID, entityType)
}
