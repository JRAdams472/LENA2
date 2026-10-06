package notifier

import (
	"github.com/JRAdams472/LENA2/internal/notifier/sqlc"
)

// pushTitle is the fixed notification title for pushed rows. The body
// carries the content; keeping the title stable groups pushes cleanly in
// the Android notification shade.
const pushTitle = "LENA"

// pushText renders the title/body the delivery worker sends for one outbox
// row. Rows carrying stored text (sweep reminders) use it verbatim;
// event-driven kinds render the same sentences the mobile feed renders in
// _notificationText (clients/mobile/lib/screens/household_screen.dart) so
// the copy stays consistent across surfaces.
func pushText(d sqlc.HouseholdPushDelivery, actorName string) (title, body string) {
	if d.Title.Valid && d.Body.Valid {
		return d.Title.String, d.Body.String
	}
	who := actorName
	if who == "" {
		who = "Someone"
	}
	switch d.Kind {
	case "invite_received":
		body = who + " invited you to their household"
	case "invite_accepted":
		body = who + " accepted your household invite"
	case "invite_declined":
		body = who + " declined your household invite"
	case "invite_cancelled":
		body = who + " cancelled a household invite"
	case "member_joined":
		body = who + " joined your household"
	case "member_left":
		body = who + " left your household"
	case "member_removed":
		body = "You were removed from a household"
	case "role_changed":
		body = who + " changed a household role"
	case "household_renamed":
		body = who + " renamed the household"
	case "event_created":
		body = who + " created an event"
	case "event_updated":
		body = who + " updated an event"
	case "event_deleted":
		body = who + " deleted an event"
	case KindProteinDefrost:
		body = "Protein defrost reminder"
	case KindMealPrepAdvance:
		body = "Meal prep reminder"
	case KindItemExpiring:
		body = "Pantry item expiring soon"
	default:
		body = "Household update"
	}
	return pushTitle, body
}
