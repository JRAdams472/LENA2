import {
  notificationHref,
  notificationReadAt,
  notificationText,
  timeAgo,
  unreadNotificationIds,
} from "@/lib/notifications";
import type { HouseholdNotification } from "@/lib/types";

const base: HouseholdNotification = {
  notificationID: 1,
  kind: "ITEM_EXPIRING",
  actor: null,
  foodEventId: null,
  title: null,
  body: null,
  recipeId: null,
  itemId: null,
  createdAt: "2026-09-20T10:00:00Z",
  readAt: null,
};

const n = (over: Partial<HouseholdNotification>): HouseholdNotification => ({
  ...base,
  ...over,
});

describe("notificationHref", () => {
  it("deep-links by available target", () => {
    expect(notificationHref(n({ kind: "ACCESS_REQUESTED" }))).toBe(
      "/member/users"
    );
    expect(notificationHref(n({ recipeId: 7 }))).toBe("/member/recipes/7");
    expect(notificationHref(n({ foodEventId: 3 }))).toBe("/member/events/3");
    expect(notificationHref(n({ itemId: 9 }))).toBe("/member/inventory/items");
    expect(notificationHref(n({ kind: "MEMBER_JOINED" }))).toBe(
      "/member/household"
    );
  });
});

describe("notificationText", () => {
  it("prefers the server-rendered title", () => {
    expect(
      notificationText(n({ title: "Milk expires soon", kind: "ITEM_EXPIRING" }))
    ).toBe("Milk expires soon");
  });

  it("renders event-driven kinds from kind + actor", () => {
    const actor = {
      userID: 9,
      displayName: "Mate",
      firstName: null,
      lastName: null,
    };
    expect(notificationText(n({ kind: "INVITE_RECEIVED", actor }))).toBe(
      "Mate invited you to their household"
    );
    expect(notificationText(n({ kind: "MEMBER_LEFT", actor }))).toBe(
      "Mate left your household"
    );
    expect(notificationText(n({ kind: "MEMBER_REMOVED" }))).toBe(
      "You were removed from a household"
    );
  });

  it("falls back for unknown kinds and missing actors", () => {
    expect(
      notificationText(n({ kind: "UNKNOWN_FUTURE" as never }))
    ).toBe("Notification");
    expect(notificationText(n({ kind: "MEMBER_JOINED" }))).toContain(
      "Someone"
    );
  });
});

describe("timeAgo", () => {
  it("formats recent timestamps relatively", () => {
    expect(timeAgo("not-a-date")).toBe("");
    expect(timeAgo(new Date().toISOString())).toBe("just now");
    expect(
      timeAgo(new Date(Date.now() - 5 * 60000).toISOString())
    ).toBe("5m ago");
    expect(
      timeAgo(new Date(Date.now() - 3 * 3600000).toISOString())
    ).toBe("3h ago");
    expect(
      timeAgo(new Date(Date.now() - 2 * 86400000).toISOString())
    ).toBe("2d ago");
  });
});

describe("notificationReadAt", () => {
  it("reads the API's readAt field, null while unread", () => {
    expect(notificationReadAt(n({}))).toBeNull();
    const withRead = n({ readAt: "2026-09-20T11:00:00Z" });
    expect(notificationReadAt(withRead)).toBe("2026-09-20T11:00:00Z");
  });
});

describe("unreadNotificationIds", () => {
  const items = [n({ notificationID: 4 }), n({ notificationID: 3 }), n({ notificationID: 2 })];

  it("marks the first `unreadCount` rows unread (newest-first feed)", () => {
    expect([...unreadNotificationIds(items, 2)].sort()).toEqual([3, 4]);
    expect(unreadNotificationIds(items, 0).size).toBe(0);
    expect(unreadNotificationIds(items, 5).size).toBe(3);
  });

  it("skips rows read server-side or marked this session", () => {
    const read3 = n({
      notificationID: 3,
      readAt: "2026-09-20T11:00:00Z",
    });
    expect([...unreadNotificationIds([items[0], read3, items[2]], 2)].sort()).toEqual([2, 4]);

    const local = new Set([4]);
    expect([...unreadNotificationIds(items, 2, local)]).toEqual([3, 2]);
  });
});
