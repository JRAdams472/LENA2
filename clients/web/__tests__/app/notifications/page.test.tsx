import "@testing-library/jest-dom";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import NotificationsPage from "@/app/member/notifications/page";
import { api } from "@/lib/api";

const mockPush = jest.fn();
jest.mock("next/navigation", () => ({
  useRouter: jest.fn(() => ({ push: mockPush })),
}));

jest.mock("../../../lib/api", () => ({
  api: {
    getMyNotificationPreferences: jest.fn(),
    getMyNotifications: jest.fn(),
    getUnreadNotificationCount: jest.fn(),
    markNotificationRead: jest.fn(),
    markAllNotificationsRead: jest.fn(),
    setNotificationCategoryEnabled: jest.fn(),
    setNotificationCategoryPushEnabled: jest.fn(),
    muteNotifications: jest.fn(),
    clearNotificationMute: jest.fn(),
  },
  ApiError: class ApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

const mockedApi = api as jest.Mocked<typeof api>;

const notifications = [
  {
    notificationID: 4,
    kind: "ITEM_EXPIRING" as const,
    actor: null,
    foodEventId: null,
    title: "Milk expires soon",
    body: "Use it or add a replacement.",
    recipeId: null,
    itemId: 9,
    createdAt: "2026-09-20T10:00:00Z",
    readAt: null,
  },
  {
    notificationID: 3,
    kind: "INVITE_RECEIVED" as const,
    actor: {
      userID: 9,
      displayName: "Mate",
      firstName: null,
      lastName: null,
    },
    foodEventId: null,
    title: null,
    body: null,
    recipeId: null,
    itemId: null,
    createdAt: "2026-09-19T10:00:00Z",
    readAt: null,
  },
];

const prefs = [
  {
    category: "_all",
    label: "All notifications",
    enabled: true,
    pushEnabled: false,
    mutedUntil: null,
  },
  {
    category: "household",
    label: "Household activity",
    enabled: true,
    pushEnabled: true,
    mutedUntil: null,
  },
  {
    category: "expiry",
    label: "Expiring pantry items",
    enabled: false,
    pushEnabled: false,
    mutedUntil: null,
  },
];

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <NotificationsPage />
    </QueryClientProvider>
  );
}

describe("NotificationsPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedApi.getMyNotificationPreferences.mockResolvedValue(prefs);
    mockedApi.getMyNotifications.mockResolvedValue(notifications);
    mockedApi.getUnreadNotificationCount.mockResolvedValue(2);
  });

  it("lists every opt-out bucket with its effective state", async () => {
    renderPage();

    expect(await screen.findByText("All notifications (global mute)")).toBeInTheDocument();
    expect(screen.getByText("Household activity")).toBeInTheDocument();
    expect(screen.getByText("Expiring pantry items")).toBeInTheDocument();
    expect(screen.getByText("Turned off")).toBeInTheDocument();
    // Household is feed-on + push-on, so it reads "Delivered · pushed".
    expect(screen.getByText("Delivered · pushed")).toBeInTheDocument();
  });

  it("toggles a category off via the switch", async () => {
    mockedApi.setNotificationCategoryEnabled.mockResolvedValue(true);
    renderPage();

    const toggle = await screen.findByLabelText("Enable Household activity");
    await userEvent.click(toggle);

    await waitFor(() =>
      expect(mockedApi.setNotificationCategoryEnabled).toHaveBeenCalledWith(
        "household",
        false
      )
    );
    expect(mockedApi.getMyNotificationPreferences).toHaveBeenCalledTimes(2);
  });

  it("toggles a category's push delivery via its own switch", async () => {
    mockedApi.setNotificationCategoryPushEnabled.mockResolvedValue(true);
    renderPage();

    const toggle = await screen.findByLabelText("Push Expiring pantry items");
    await userEvent.click(toggle);

    await waitFor(() =>
      expect(mockedApi.setNotificationCategoryPushEnabled).toHaveBeenCalledWith(
        "expiry",
        true
      )
    );
    // Feed toggle untouched — push is an independent channel.
    expect(mockedApi.setNotificationCategoryEnabled).not.toHaveBeenCalled();
  });

  it("shows a push switch on the _all row — it is the master push opt-in", async () => {
    mockedApi.setNotificationCategoryPushEnabled.mockResolvedValue(true);
    renderPage();

    const toggle = await screen.findByLabelText("Push All notifications");
    await userEvent.click(toggle);

    await waitFor(() =>
      expect(mockedApi.setNotificationCategoryPushEnabled).toHaveBeenCalledWith(
        "_all",
        true
      )
    );
  });

  it("mutes a category for a preset window", async () => {
    mockedApi.muteNotifications.mockResolvedValue(true);
    renderPage();

    // The first row (_all) is index 0; household's Mute button is second.
    const muteButtons = await screen.findAllByRole("button", { name: "Mute" });
    await userEvent.click(muteButtons[1]);
    await userEvent.click(await screen.findByText("Mute for 1 day"));

    await waitFor(() =>
      expect(mockedApi.muteNotifications).toHaveBeenCalledWith(
        "household",
        expect.any(String)
      )
    );
  });

  it("sends a null category for the global mute row", async () => {
    mockedApi.muteNotifications.mockResolvedValue(true);
    renderPage();

    const muteButtons = await screen.findAllByRole("button", { name: "Mute" });
    await userEvent.click(muteButtons[0]);
    await userEvent.click(await screen.findByText("Mute for 1 hour"));

    await waitFor(() =>
      expect(mockedApi.muteNotifications).toHaveBeenCalledWith(
        null,
        expect.any(String)
      )
    );
  });

  it("shows an active mute chip that can be cleared", async () => {
    const until = new Date(Date.now() + 3_600_000).toISOString();
    mockedApi.getMyNotificationPreferences.mockResolvedValue([
      { ...prefs[1], mutedUntil: until },
    ]);
    mockedApi.clearNotificationMute.mockResolvedValue(true);
    renderPage();

    const chip = await screen.findByText(/Muted until/);
    expect(chip).toBeInTheDocument();

    // Chip's delete icon clears the mute.
    await userEvent.click(screen.getByTestId("CancelIcon"));
    await waitFor(() =>
      expect(mockedApi.clearNotificationMute).toHaveBeenCalledWith("household")
    );
  });

  describe("notification feed", () => {
    it("lists notification history newest first with unread markers", async () => {
      renderPage();

      expect(await screen.findByText("Milk expires soon")).toBeInTheDocument();
      expect(
        screen.getByText("Mate invited you to their household")
      ).toBeInTheDocument();
      // Until the API returns per-item readAt both rows read as unread.
      expect(screen.getByTestId("unread-dot-4")).toBeInTheDocument();
      expect(screen.getByTestId("unread-dot-3")).toBeInTheDocument();
      // Newest (id 4) precedes the older invite row.
      expect(
        screen
          .getByText("Milk expires soon")
          .compareDocumentPosition(
            screen.getByText("Mate invited you to their household")
          ) & Node.DOCUMENT_POSITION_FOLLOWING
      ).toBeTruthy();
    });

    it("shows an empty-feed message instead of a bare list", async () => {
      mockedApi.getMyNotifications.mockResolvedValue([]);
      renderPage();
      expect(
        await screen.findByText(/No notifications yet/i)
      ).toBeInTheDocument();
    });

    it("marks a single notification read and follows its deep-link", async () => {
      mockedApi.markNotificationRead.mockResolvedValue(true);
      renderPage();

      await userEvent.click(await screen.findByText("Milk expires soon"));

      await waitFor(() =>
        expect(mockedApi.markNotificationRead).toHaveBeenCalledWith(4)
      );
      expect(mockPush).toHaveBeenCalledWith("/member/inventory/items");
      // The marked row loses its unread dot; the other stays.
      await waitFor(() =>
        expect(screen.queryByTestId("unread-dot-4")).not.toBeInTheDocument()
      );
      expect(screen.getByTestId("unread-dot-3")).toBeInTheDocument();
    });

    it("marks everything read via the manual action", async () => {
      mockedApi.markAllNotificationsRead.mockResolvedValue(true);
      renderPage();
      await screen.findByText("Milk expires soon");

      await userEvent.click(
        screen.getByRole("button", { name: /mark all read/i })
      );

      await waitFor(() =>
        expect(mockedApi.markAllNotificationsRead).toHaveBeenCalled()
      );
    });
  });
});
