package notifier

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"

	"github.com/JRAdams472/LENA2/internal/notifier/sqlc"
)

func TestPushText_StoredTextWins(t *testing.T) {
	d := sqlc.HouseholdPushDelivery{
		Kind:  "event_created",
		Title: pgtype.Text{String: "Milk expires soon", Valid: true},
		Body:  pgtype.Text{String: "Milk expires tomorrow", Valid: true},
	}
	title, body := pushText(d, "Alice")
	assert.Equal(t, "Milk expires soon", title)
	assert.Equal(t, "Milk expires tomorrow", body)
}

func TestPushText_EventKindsRenderWithActor(t *testing.T) {
	for _, tc := range []struct {
		kind string
		want string
	}{
		{"invite_received", "Alice invited you to their household"},
		{"invite_accepted", "Alice accepted your household invite"},
		{"invite_declined", "Alice declined your household invite"},
		{"invite_cancelled", "Alice cancelled a household invite"},
		{"member_joined", "Alice joined your household"},
		{"member_left", "Alice left your household"},
		{"member_removed", "You were removed from a household"},
		{"role_changed", "Alice changed a household role"},
		{"household_renamed", "Alice renamed the household"},
		{"event_created", "Alice created an event"},
		{"event_updated", "Alice updated an event"},
		{"event_deleted", "Alice deleted an event"},
		{KindProteinDefrost, "Protein defrost reminder"},
		{KindMealPrepAdvance, "Meal prep reminder"},
		{KindItemExpiring, "Pantry item expiring soon"},
		{"some_future_kind", "Household update"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			title, body := pushText(sqlc.HouseholdPushDelivery{Kind: tc.kind}, "Alice")
			assert.Equal(t, pushTitle, title)
			assert.Equal(t, tc.want, body)
		})
	}
}

func TestPushText_MissingActorFallsBack(t *testing.T) {
	_, body := pushText(sqlc.HouseholdPushDelivery{Kind: "member_joined"}, "")
	assert.Equal(t, "Someone joined your household", body)
}
