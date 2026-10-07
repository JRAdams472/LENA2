import "@testing-library/jest-dom";
import { render, screen, waitFor, within } from "@testing-library/react";
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
    getMyHouseholds: jest.fn(),
    getHouseholdInvites: jest.fn(),
    searchHouseholdUsers: jest.fn(),
    inviteHouseholdMember: jest.fn(),
    acceptHouseholdInvite: jest.fn(),
    declineHouseholdInvite: jest.fn(),
    cancelHouseholdInvite: jest.fn(),
    leaveHousehold: jest.fn(),
    renameHousehold: jest.fn(),
    setHouseholdRole: jest.fn(),
    removeHouseholdMember: jest.fn(),
    transferHouseholdOwnership: jest.fn(),
    setActiveHousehold: jest.fn(),
    createHousehold: jest.fn(),
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
  name: null,
  myRole: "OWNER" as const,
  isActive: true,
  members: [
    {
      user: { userID: 7, displayName: "Me", firstName: null, lastName: null },
      role: "OWNER" as const,
      isMe: true,
    },
    { user: mate, role: "MEMBER" as const, isMe: false },
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
    mockedApi.getMyHouseholds.mockResolvedValue([household]);
    mockedApi.getHouseholdInvites.mockResolvedValue([]);
    mockedApi.searchHouseholdUsers.mockResolvedValue([]);
  });

  it("lists household members with role chips", async () => {
    renderPage();
    expect(await screen.findByText("Mate")).toBeInTheDocument();
    expect(screen.getByText("owner")).toBeInTheDocument();
    expect(screen.getByText("member")).toBeInTheDocument();
    expect(screen.getByText("you")).toBeInTheDocument();
    expect(screen.getByText("2 of 10 members")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /leave household/i })
    ).toBeInTheDocument();
  });

  it("lets the owner rename the household", async () => {
    mockedApi.renameHousehold.mockResolvedValue(household);
    renderPage();
    await screen.findByText("Mate");
    const input = screen.getByLabelText(/household name/i);
    await userEvent.clear(input);
    await userEvent.type(input, "Casa");
    await userEvent.click(screen.getByRole("button", { name: /^save$/i }));
    await waitFor(() =>
      expect(mockedApi.renameHousehold).toHaveBeenCalledWith("Casa")
    );
  });

  it("owner can promote, transfer, and remove via the member menu", async () => {
    renderPage();
    await screen.findByText("Mate");
    await userEvent.click(
      screen.getByRole("button", { name: /manage mate/i })
    );
    expect(await screen.findByText("Make admin")).toBeInTheDocument();
    expect(screen.getByText("Transfer ownership")).toBeInTheDocument();
    await userEvent.click(screen.getByText("Make admin"));
    await waitFor(() =>
      expect(mockedApi.setHouseholdRole).toHaveBeenCalledWith(9, "ADMIN")
    );
  });

  it("owner removes a member after confirmation", async () => {
    const confirm = jest.spyOn(window, "confirm").mockReturnValue(true);
    renderPage();
    await screen.findByText("Mate");
    await userEvent.click(
      screen.getByRole("button", { name: /manage mate/i })
    );
    await userEvent.click(await screen.findByText("Remove from household"));
    await waitFor(() =>
      expect(mockedApi.removeHouseholdMember).toHaveBeenCalledWith(9)
    );
    confirm.mockRestore();
  });

  it("hides manage controls from plain members", async () => {
    mockedApi.getMyHousehold.mockResolvedValue({
      ...household,
      myRole: "MEMBER",
      members: [
        { user: mate, role: "OWNER" as const, isMe: false },
        {
          user: { userID: 7, displayName: "Me", firstName: null, lastName: null },
          role: "MEMBER" as const,
          isMe: true,
        },
      ],
    });
    renderPage();
    await screen.findByText("Mate");
    expect(
      screen.queryByRole("button", { name: /manage/i })
    ).not.toBeInTheDocument();
    expect(
      screen.queryByLabelText(/household name/i)
    ).not.toBeInTheDocument();
  });

  it("admin can remove members but not manage admins", async () => {
    const adminMember = {
      user: { userID: 13, displayName: "Boss", firstName: null, lastName: null },
      role: "ADMIN" as const,
      isMe: false,
    };
    mockedApi.getMyHousehold.mockResolvedValue({
      ...household,
      myRole: "ADMIN",
      members: [
        adminMember,
        { user: mate, role: "MEMBER" as const, isMe: false },
        {
          user: { userID: 7, displayName: "Me", firstName: null, lastName: null },
          role: "ADMIN" as const,
          isMe: true,
        },
      ],
    });
    renderPage();
    await screen.findByText("Mate");
    expect(
      screen.getByRole("button", { name: /manage mate/i })
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /manage boss/i })
    ).not.toBeInTheDocument();
    await userEvent.click(
      screen.getByRole("button", { name: /manage mate/i })
    );
    expect(await screen.findByText("Remove from household")).toBeInTheDocument();
    expect(screen.queryByText("Make admin")).not.toBeInTheDocument();
    expect(screen.queryByText("Transfer ownership")).not.toBeInTheDocument();
  });

  it("shows the member cap notice at 10 members", async () => {
    mockedApi.getMyHousehold.mockResolvedValue({
      ...household,
      members: [
        household.members[0],
        ...Array.from({ length: 9 }, (_, i) => ({
          user: {
            userID: 20 + i,
            displayName: `Member ${i}`,
            firstName: null,
            lastName: null,
          },
          role: "MEMBER" as const,
          isMe: false,
        })),
      ],
    });
    renderPage();
    expect(await screen.findByText("10 of 10 members")).toBeInTheDocument();
    expect(
      screen.getByText(/at the 10-member limit/i)
    ).toBeInTheDocument();
  });

  it("accepts an incoming invite", async () => {
    mockedApi.getHouseholdInvites.mockResolvedValue([incomingInvite]);
    renderPage();
    await screen.findByText(/Mate invited you/);
    await userEvent.click(screen.getByRole("button", { name: /accept/i }));
    await waitFor(() =>
      expect(mockedApi.acceptHouseholdInvite).toHaveBeenCalledWith(
        55,
        undefined
      )
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

  it("lists all memberships with the active one marked", async () => {
    mockedApi.getMyHouseholds.mockResolvedValue([
      household,
      {
        householdID: 77,
        name: "Beach house",
        myRole: "MEMBER" as const,
        isActive: false,
        members: [household.members[0], { user: mate, role: "OWNER" as const, isMe: false }],
        createdAt: "2026-02-01T00:00:00Z",
      },
    ]);
    renderPage();
    expect(await screen.findByText("Beach house")).toBeInTheDocument();
    expect(screen.getByText("Unnamed household")).toBeInTheDocument();
    expect(screen.getByText("active")).toBeInTheDocument();
    expect(screen.getByText("2 members · member")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /switch to beach house/i })
    ).toBeInTheDocument();
  });

  it("switches the active household", async () => {
    mockedApi.getMyHouseholds.mockResolvedValue([
      household,
      {
        householdID: 77,
        name: "Beach house",
        myRole: "MEMBER" as const,
        isActive: false,
        members: [household.members[0]],
        createdAt: "2026-02-01T00:00:00Z",
      },
    ]);
    renderPage();
    await userEvent.click(
      await screen.findByRole("button", { name: /switch to beach house/i })
    );
    await waitFor(() =>
      expect(mockedApi.setActiveHousehold).toHaveBeenCalledWith(77)
    );
  });

  it("leaves a non-active household from the list", async () => {
    const confirm = jest.spyOn(window, "confirm").mockReturnValue(true);
    mockedApi.getMyHouseholds.mockResolvedValue([
      household,
      {
        householdID: 77,
        name: "Beach house",
        myRole: "MEMBER" as const,
        isActive: false,
        members: [household.members[0]],
        createdAt: "2026-02-01T00:00:00Z",
      },
    ]);
    renderPage();
    await userEvent.click(
      await screen.findByRole("button", { name: /leave beach house/i })
    );
    await waitFor(() =>
      expect(mockedApi.leaveHousehold).toHaveBeenCalledWith(77)
    );
    confirm.mockRestore();
  });

  it("creates a household through the dialog", async () => {
    renderPage();
    await userEvent.click(
      await screen.findByRole("button", { name: /new household/i })
    );
    const dialog = await screen.findByRole("dialog");
    await userEvent.type(
      within(dialog).getByLabelText(/household name/i),
      "Cabin"
    );
    await userEvent.click(
      within(dialog).getByRole("button", { name: /create/i })
    );
    await waitFor(() =>
      expect(mockedApi.createHousehold).toHaveBeenCalledWith("Cabin")
    );
  });

  it("offers merge when the caller solely owns a household", async () => {
    mockedApi.getMyHousehold.mockResolvedValue({
      ...household,
      members: [household.members[0]],
    });
    mockedApi.getMyHouseholds.mockResolvedValue([
      {
        ...household,
        name: "Solo home",
        members: [household.members[0]],
      },
    ]);
    mockedApi.getHouseholdInvites.mockResolvedValue([incomingInvite]);
    renderPage();
    await userEvent.click(
      await screen.findByRole("button", { name: /accept/i })
    );
    expect(
      await screen.findByText(/merge Solo home into the new household/)
    ).toBeInTheDocument();
    await userEvent.click(
      screen.getByRole("button", { name: /join and merge/i })
    );
    await waitFor(() =>
      expect(mockedApi.acceptHouseholdInvite).toHaveBeenCalledWith(55, 42)
    );
  });

  it("joins without merging when the user declines the merge", async () => {
    mockedApi.getMyHouseholds.mockResolvedValue([
      {
        ...household,
        name: "Solo home",
        members: [household.members[0]],
      },
    ]);
    mockedApi.getHouseholdInvites.mockResolvedValue([incomingInvite]);
    renderPage();
    await userEvent.click(
      await screen.findByRole("button", { name: /accept/i })
    );
    await screen.findByText(/merge Solo home into the new household/);
    await userEvent.click(
      screen.getByRole("button", { name: /just join/i })
    );
    await waitFor(() =>
      expect(mockedApi.acceptHouseholdInvite).toHaveBeenCalledWith(
        55,
        undefined
      )
    );
  });

  it("leaves the household after confirmation", async () => {
    const confirm = jest.spyOn(window, "confirm").mockReturnValue(true);
    renderPage();
    await screen.findByText("Mate");
    await userEvent.click(
      screen.getByRole("button", { name: /leave household/i })
    );
    await waitFor(() =>
      expect(mockedApi.leaveHousehold).toHaveBeenCalledWith(42)
    );
    confirm.mockRestore();
  });
});
