import "@testing-library/jest-dom";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import NotificationsPage from "@/app/notifications/page";
import { api } from "@/lib/api";

jest.mock("../../../lib/api", () => ({
  api: {
    getMyNotificationPreferences: jest.fn(),
    setNotificationCategoryEnabled: jest.fn(),
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

const prefs = [
  {
    category: "_all",
    label: "All notifications",
    enabled: true,
    mutedUntil: null,
  },
  {
    category: "household",
    label: "Household activity",
    enabled: true,
    mutedUntil: null,
  },
  {
    category: "expiry",
    label: "Expiring pantry items",
    enabled: false,
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
  });

  it("lists every opt-out bucket with its effective state", async () => {
    renderPage();

    expect(await screen.findByText("All notifications (global mute)")).toBeInTheDocument();
    expect(screen.getByText("Household activity")).toBeInTheDocument();
    expect(screen.getByText("Expiring pantry items")).toBeInTheDocument();
    expect(screen.getByText("Turned off")).toBeInTheDocument();
    expect(screen.getAllByText("Delivered").length).toBeGreaterThan(0);
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
    const until = new Date(Date.now() + 3600_000).toISOString();
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
});
