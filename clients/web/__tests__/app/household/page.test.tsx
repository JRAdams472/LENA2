import "@testing-library/jest-dom";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import HouseholdPage from "@/app/household/page";
import { api } from "@/lib/api";

// jest.mock specifiers are not rewritten by the SWC path transform, so
// the "@/..." alias cannot be used here — mock the resolved path instead.
jest.mock("../../../app/auth/useMe");
import { useMe } from "@/app/auth/useMe";

jest.mock("../../../lib/api", () => ({
  api: {
    getMyHousehold: jest.fn(),
    getHouseholdInvites: jest.fn(),
    searchHouseholdUsers: jest.fn(),
    inviteHouseholdMember: jest.fn(),
    acceptHouseholdInvite: jest.fn(),
    declineHouseholdInvite: jest.fn(),
    cancelHouseholdInvite: jest.fn(),
    leaveHousehold: jest.fn(),
  },
  ApiError: class ApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

const mockedUseMe = useMe as jest.Mock;
const mockedApi = api as jest.Mocked<typeof api>;

const me = {
  userID: 7,
  email: "me@example.com",
  displayName: "Me",
  firstName: null,
  lastName: null,
  backupEmail: null,
  role: "member" as const,
  isActive: true,
  isProtected: false,
  lastLoginAt: null,
  externalSubject: null,
  provider: null,
  isSearchable: true,
  household: null,
};

const mate = { userID: 9, displayName: "Mate", firstName: null, lastName: null };
const target = { userID: 11, displayName: "Target", firstName: null, lastName: null };

const household = {
  householdID: 42,
  members: [
    { userID: 7, displayName: "Me", firstName: null, lastName: null },
    mate,
  ],
  createdAt: "2026-01-01T00:00:00Z",
};

const incomingInvite = {
  inviteID: 55,
  status: "PENDING" as const,
  createdAt: "2026-09-20T10:00:00Z",
  fromUser: mate,
  toUser: { userID: 7, displayName: "Me", firstName: null, lastName: null },
};

const outgoingInvite = {
  inviteID: 56,
  status: "PENDING" as const,
  createdAt: "2026-09-20T11:00:00Z",
  fromUser: { userID: 7, displayName: "Me", firstName: null, lastName: null },
  toUser: target,
};

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <HouseholdPage />
    </QueryClientProvider>
  );
}

describe("household page", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedUseMe.mockReturnValue({
      me,
      isAdmin: false,
      isLoading: false,
      error: null,
      refetch: jest.fn(),
    });
    mockedApi.getMyHousehold.mockResolvedValue(household);
    mockedApi.getHouseholdInvites.mockResolvedValue([]);
    mockedApi.searchHouseholdUsers.mockResolvedValue([]);
  });

  it("lists household members", async () => {
    renderPage();
    expect(await screen.findByText("Mate")).toBeInTheDocument();
    expect(screen.getByText("You")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /leave household/i })
    ).toBeInTheDocument();
  });

  it("accepts an incoming invite", async () => {
    mockedApi.getHouseholdInvites.mockResolvedValue([incomingInvite]);
    renderPage();
    await screen.findByText(/Mate invited you/);
    await userEvent.click(screen.getByRole("button", { name: /accept/i }));
    await waitFor(() =>
      expect(mockedApi.acceptHouseholdInvite).toHaveBeenCalledWith(55)
    );
  });

  it("declines an incoming invite", async () => {
    mockedApi.getHouseholdInvites.mockResolvedValue([incomingInvite]);
    renderPage();
    await screen.findByText(/Mate invited you/);
    await userEvent.click(screen.getByRole("button", { name: /decline/i }));
    await waitFor(() =>
      expect(mockedApi.declineHouseholdInvite).toHaveBeenCalledWith(55)
    );
  });

  it("cancels an outgoing invite", async () => {
    mockedApi.getHouseholdInvites.mockResolvedValue([outgoingInvite]);
    renderPage();
    await screen.findByText("Target");
    await userEvent.click(screen.getByRole("button", { name: /cancel/i }));
    await waitFor(() =>
      expect(mockedApi.cancelHouseholdInvite).toHaveBeenCalledWith(56)
    );
  });

  it("searches and invites a user", async () => {
    mockedApi.searchHouseholdUsers.mockResolvedValue([target]);
    renderPage();
    await screen.findByText("Members");

    await userEvent.type(screen.getByLabelText(/name or email/i), "tar");
    await userEvent.click(screen.getByRole("button", { name: /search/i }));

    expect(await screen.findByText("Target")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /invite/i }));
    await waitFor(() =>
      expect(mockedApi.inviteHouseholdMember).toHaveBeenCalledWith(11)
    );
  });

  it("leaves the household after confirmation", async () => {
    const confirm = jest.spyOn(window, "confirm").mockReturnValue(true);
    renderPage();
    await screen.findByText("Mate");
    await userEvent.click(
      screen.getByRole("button", { name: /leave household/i })
    );
    await waitFor(() => expect(mockedApi.leaveHousehold).toHaveBeenCalled());
    confirm.mockRestore();
  });
});
