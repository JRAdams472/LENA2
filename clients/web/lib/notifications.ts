// Shared presentation helpers for notification surfaces — the header
// bell (app/components/AdminLayout) and the /member/notifications feed.

import type { HouseholdNotification } from "@/lib/types";

export function timeAgo(iso: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const mins = Math.floor((Date.now() - then) / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

// Reminders deep-link to their subject; event notifications to the
// event; household activity to the household page.
export function notificationHref(n: HouseholdNotification): string {
  if (n.kind === "ACCESS_REQUESTED") return "/member/users";
  if (n.recipeId != null) return `/member/recipes/${n.recipeId}`;
  if (n.foodEventId != null) return `/member/events/${n.foodEventId}`;
  if (n.itemId != null) return "/member/inventory/items";
  return "/member/household";
}

export function notificationText(n: HouseholdNotification): string {
  // Scheduled reminders carry server-rendered text.
  if (n.title) return n.title;
  const actor = n.actor?.displayName ?? "Someone";
  switch (n.kind) {
    case "INVITE_RECEIVED":
      return `${actor} invited you to their household`;
    case "INVITE_ACCEPTED":
      return `${actor} accepted your household invite`;
    case "INVITE_DECLINED":
      return `${actor} declined your household invite`;
    case "INVITE_CANCELLED":
      return `${actor} cancelled a household invite`;
    case "MEMBER_JOINED":
      return `${actor} joined your household`;
    case "MEMBER_LEFT":
      return `${actor} left your household`;
    case "MEMBER_REMOVED":
      return "You were removed from a household";
    case "ACCESS_REQUESTED":
      return `${actor} requested access to LENA`;
    case "ROLE_CHANGED":
      return `${actor} changed a household role`;
    case "HOUSEHOLD_RENAMED":
      return `${actor} renamed the household`;
    case "EVENT_CREATED":
      return `${actor} created an event`;
    case "EVENT_UPDATED":
      return `${actor} updated an event`;
    case "EVENT_DELETED":
      return `${actor} deleted an event`;
    case "PROTEIN_DEFROST":
      return "Time to defrost protein for an upcoming meal";
    case "MEAL_PREP_ADVANCE":
      return "A recipe on your meal plan needs advance prep";
    case "ITEM_EXPIRING":
      return "A pantry item is expiring soon";
    default:
      return "Notification";
  }
}

// Per-item read state, straight off the myNotifications row (null while
// unread). unreadNotificationIds still layers the unread-count heuristic
// and session-local marks on top — a stale cached page can lag the server.
export function notificationReadAt(n: HouseholdNotification): string | null {
  return n.readAt ?? null;
}

// Which visible rows are unread. myNotifications arrives newest-first but
// today the server only exposes an unread *count* — so the first
// `unreadCount` rows that aren't known-read (server readAt once it lands,
// or marked this session via `locallyRead`) are treated as unread. When
// `unreadCount` is unknown, pass items.length so every unmarked row shows
// the dot rather than none.
export function unreadNotificationIds(
  items: HouseholdNotification[],
  unreadCount: number,
  locallyRead: ReadonlySet<number> = new Set<number>()
): Set<number> {
  const unread = new Set<number>();
  let remaining = unreadCount;
  for (const n of items) {
    if (remaining <= 0) break;
    if (
      notificationReadAt(n) !== null ||
      locallyRead.has(n.notificationID)
    ) {
      continue;
    }
    unread.add(n.notificationID);
    remaining--;
  }
  return unread;
}
