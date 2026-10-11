import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import UsersPage from "@/app/member/users/page";
import { api } from "@/lib/api";
import { User } from "@/lib/types";
// jest.mock specifiers are not rewritten by the SWC path transform, so
// the "@/..." alias cannot be used here — mock the resolved path instead.
jest.mock("../../../app/auth/useMe");
import { useMe } from "@/app/auth/useMe";

jest.mock("../../../lib/api", () => ({
  api: {
    getUsers: jest.fn(),
    getPendingUsers: jest.fn(),
    approveUser: jest.fn(),
    denyUser: jest.fn(),
    setUserRole: jest.fn(),
    setUserActive: jest.fn(),
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

const member = {
  userID: 2,
  email: "member@example.com",
  displayName: null,
  firstName: "Ada",
  lastName: "L",
  backupEmail: null,
    birthdate: null,
  role: "member" as const,
  isActive: true,
  isProtected: false,
  lastLoginAt: null,
  externalSubject: null,
  provider: null,
  isSearchable: true,
  household: null,
};

const admin = { ...member, userID: 1, email: "admin@example.com", role: "admin" as const };
const protectedAdmin = { ...admin, userID: 3, email: "boss@example.com", isProtected: true };

function meReturn(me: User | null, isAdmin: boolean) {
  return { me, isAdmin, isLoading: false, error: null, refetch: jest.fn() };
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <UsersPage />
    </QueryClientProvider>
  );
}

describe("users admin page", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  const emptyPage = {
    items: [] as User[],
    pageNumber: 1,
    pageSize: 25,
    totalCount: 0,
    totalPages: 1,
  };

  function mockPending(items: User[]) {
    mockedApi.getPendingUsers.mockResolvedValue({
      ...emptyPage,
      items,
      totalCount: items.length,
    });
  }

  it("shows Forbidden for non-admins", () => {
    mockedUseMe.mockReturnValue(meReturn(member, false));
    renderPage();
    expect(screen.getByText(/Forbidden/i)).toBeInTheDocument();
    expect(mockedApi.getUsers).not.toHaveBeenCalled();
  });

  it("lists pending access requests and approves them", async () => {
    const pendingUser = { ...member, userID: 9, email: "new@example.com", approvalStatus: "PENDING" as const };
    mockedUseMe.mockReturnValue(meReturn(admin, true));
    mockedApi.getUsers.mockResolvedValue({ ...emptyPage, items: [admin], totalCount: 1 });
    mockPending([pendingUser]);
    mockedApi.approveUser.mockResolvedValue({ ...pendingUser, approvalStatus: "APPROVED" });

    renderPage();
    await waitFor(() =>
      expect(screen.getByText("new@example.com")).toBeInTheDocument()
    );
    expect(screen.getByText(/Access requests \(1\)/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /^Approve$/ }));
    await waitFor(() =>
      expect(mockedApi.approveUser).toHaveBeenCalledWith(9)
    );
  });

  it("denies a pending user after confirmation", async () => {
    const pendingUser = { ...member, userID: 9, email: "new@example.com", approvalStatus: "PENDING" as const };
    mockedUseMe.mockReturnValue(meReturn(admin, true));
    mockedApi.getUsers.mockResolvedValue({ ...emptyPage, items: [admin], totalCount: 1 });
    mockPending([pendingUser]);
    mockedApi.denyUser.mockResolvedValue({ ...pendingUser, approvalStatus: "DENIED" });

    renderPage();
    await waitFor(() =>
      expect(screen.getByText("new@example.com")).toBeInTheDocument()
    );

    fireEvent.click(screen.getByRole("button", { name: /^Deny$/ }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: /^Deny$/ }));
    await waitFor(() =>
      expect(mockedApi.denyUser).toHaveBeenCalledWith(9)
    );
  });

  it("hides the requests section when nobody is pending", async () => {
    mockedUseMe.mockReturnValue(meReturn(admin, true));
    mockedApi.getUsers.mockResolvedValue({ ...emptyPage, items: [admin], totalCount: 1 });
    mockPending([]);

    renderPage();
    await waitFor(() =>
      expect(screen.getByText("admin@example.com")).toBeInTheDocument()
    );
    expect(screen.queryByText(/Access requests/)).not.toBeInTheDocument();
  });

  it("lists users for admins", async () => {
    mockedUseMe.mockReturnValue(meReturn(admin, true));
    mockedApi.getUsers.mockResolvedValue({
      items: [admin, member, protectedAdmin],
      pageNumber: 1,
      pageSize: 25,
      totalCount: 3,
      totalPages: 1,
    });

    renderPage();
    await waitFor(() =>
      expect(screen.getByText("member@example.com")).toBeInTheDocument()
    );
    expect(screen.getByText("boss@example.com")).toBeInTheDocument();
  });

  it("disables actions on self and protected rows", async () => {
    mockedUseMe.mockReturnValue(meReturn(admin, true));
    mockedApi.getUsers.mockResolvedValue({
      items: [admin, member, protectedAdmin],
      pageNumber: 1,
      pageSize: 25,
      totalCount: 3,
      totalPages: 1,
    });

    renderPage();
    await waitFor(() =>
      expect(screen.getByText("boss@example.com")).toBeInTheDocument()
    );

    // admin (self) and protectedAdmin rows must be disabled; member enabled.
    const rows = screen.getAllByRole("row").slice(1);
    expect(rows).toHaveLength(3);
    const selfBan = within(rows[0]).getByRole("button", { name: /ban/i });
    const memberBan = within(rows[1]).getByRole("button", { name: /ban/i });
    const protectedBan = within(rows[2]).getByRole("button", { name: /ban/i });
    expect(selfBan).toBeDisabled();
    expect(memberBan).not.toBeDisabled();
    expect(protectedBan).toBeDisabled();
  });
});
