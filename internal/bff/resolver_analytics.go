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
	entityID, err := parseID(string(args.EntityID))
	if err != nil {
		return false, err
	}
	if err := r.AnalyticsService.RecordEvent(ctx, analytics.Event{
		UserID:     u.UserID,
		EventType:  selectionEventType(args.EntityType),
		EntityType: strings.TrimSpace(args.EntityType),
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
	if err := r.AnalyticsService.RecordEvent(ctx, analytics.Event{
		UserID:     u.UserID,
		EventType:  searchEventType(args.EntityType),
		EntityType: strings.TrimSpace(args.EntityType),
		SearchTerm: strings.TrimSpace(args.Term),
	}, u.Email); err != nil {
		slog.Default().Error("record search failed", "error", err)
	}
	return true, nil
}

func selectionEventType(entityType string) string {
	switch strings.ToLower(strings.TrimSpace(entityType)) {
	case analytics.EntityBrand:
		return analytics.EventBrandSelected
	case analytics.EntityRecipe:
		return analytics.EventRecipeSelected
	default:
		return analytics.EventItemSelected
	}
}

func searchEventType(entityType string) string {
	switch strings.ToLower(strings.TrimSpace(entityType)) {
	case analytics.EntityRecipe:
		return analytics.EventRecipeSearched
	default:
		return analytics.EventItemSearched
	}
}
