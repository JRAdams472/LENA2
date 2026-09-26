import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { usePathname } from "next/navigation";
import AdminLayout from "@/app/components/AdminLayout";
import { AuthProvider } from "@/app/auth/AuthProvider";

jest.mock("next/navigation");
jest.mock("@react-oauth/google", () => ({
  GoogleLogin: () => <button data-testid="google-login">Sign in with Google</button>,
  googleLogout: jest.fn(),
  GoogleOAuthProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

const mockedUsePathname = usePathname as jest.Mock;

function makeToken(email: string, exp: number) {
  const header = btoa(JSON.stringify({ alg: "none", typ: "JWT" }))
    .replace(/=/g, "")
    .replace(/\+/g, "-")
    .replace(/\//g, "_");
  const payload = btoa(JSON.stringify({ email, exp }))
    .replace(/=/g, "")
    .replace(/\+/g, "-")
    .replace(/\//g, "_");
  return `${header}.${payload}.signature`;
}

function signIn(email = "admin@example.com") {
  const future = Math.floor(Date.now() / 1000) + 3600;
  sessionStorage.setItem("lena_id_token", makeToken(email, future));
}

function renderLayout() {
  return render(
    <AuthProvider>
      <AdminLayout>
        <div data-testid="page-content">Content</div>
      </AdminLayout>
    </AuthProvider>
  );
}

const mockFetch = global.fetch as jest.Mock;

function gql(data: object) {
  return {
    ok: true,
    status: 200,
    headers: { get: () => "application/json" },
    json: async () => ({ data }),
  };
}

const meResponse = {
  me: {
    id: "1",
    email: "admin@example.com",
    displayName: "Admin",
    firstName: null,
    lastName: null,
    backupEmail: null,
    role: "admin",
    isActive: true,
    isProtected: false,
    lastLoginAt: null,
    isSearchable: true,
    household: null,
  },
};

describe("AdminLayout", () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    mockedUsePathname.mockReturnValue("/");
    mockFetch.mockReset();
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("unreadNotificationCount"))
        return Promise.resolve(gql({ unreadNotificationCount: 2 }));
      if (body.query.includes("myNotifications"))
        return Promise.resolve(
          gql({
            myNotifications: [
              {
                id: "3",
                kind: "INVITE_RECEIVED",
                createdAt: "2026-09-20T10:00:00Z",
                actor: {
                  id: "9",
                  displayName: "Mate",
                  firstName: null,
                  lastName: null,
                },
              },
            ],
          })
        );
      if (body.query.includes("markAllNotificationsRead"))
        return Promise.resolve(gql({ markAllNotificationsRead: true }));
      return Promise.resolve(gql(meResponse));
    });
  });

  it("gates the app behind the login screen when signed out", () => {
    renderLayout();

    expect(screen.getByTestId("google-login")).toBeInTheDocument();
    expect(screen.queryByTestId("page-content")).not.toBeInTheDocument();
  });

  it("renders navigation groups and the signed-in user", async () => {
    signIn();
    renderLayout();

    await waitFor(() =>
      expect(screen.getByTestId("page-content")).toBeInTheDocument()
    );
    expect(screen.getAllByText("admin@example.com")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Dashboard")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Inventory")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Wine")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Meal Planning")[0]).toBeInTheDocument();
  });

  it("expands a navigation group to reveal child links", async () => {
    signIn();
    renderLayout();

    await waitFor(() =>
      expect(screen.getByTestId("page-content")).toBeInTheDocument()
    );

    expect(screen.queryAllByText("Categories")).toHaveLength(0);
    fireEvent.click(screen.getAllByText("Inventory")[0]);
    expect((await screen.findAllByText("Categories")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("Brands").length).toBeGreaterThan(0);
  });

  it("auto-expands the group containing the active route", async () => {
    signIn();
    mockedUsePathname.mockReturnValue("/inventory/categories");
    renderLayout();

    await waitFor(() =>
      expect(screen.getByTestId("page-content")).toBeInTheDocument()
    );
    expect((await screen.findAllByText("Categories")).length).toBeGreaterThan(0);
  });

  it("shows the unread badge and lists notifications in the bell menu", async () => {
    signIn();
    renderLayout();

    await waitFor(() =>
      expect(screen.getByTestId("notification-badge")).toHaveTextContent("2")
    );

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "notifications" }));
    });
    expect(
      await screen.findByText("Mate invited you to their household")
    ).toBeInTheDocument();
    expect(screen.getByText(/ago|just now/)).toBeInTheDocument();

    // Opening the menu marks everything read.
    expect(
      mockFetch.mock.calls.some(([, init]) =>
        (init as RequestInit).body
          ?.toString()
          .includes("markAllNotificationsRead")
      )
    ).toBe(true);
    // MUI keeps the last count in the DOM; the invisible class is the signal.
    expect(
      screen
        .getByTestId("notification-badge")
        .querySelector(".MuiBadge-badge")
    ).toHaveClass("MuiBadge-invisible");
  });

  it("signs out and returns to the login screen", async () => {
    signIn();
    renderLayout();

    await waitFor(() =>
      expect(screen.getByTestId("page-content")).toBeInTheDocument()
    );
    fireEvent.click(screen.getByRole("button", { name: "Sign out" }));

    await waitFor(() =>
      expect(screen.getByTestId("google-login")).toBeInTheDocument()
    );
    expect(sessionStorage.getItem("lena_id_token")).toBeNull();
    expect(localStorage.getItem("lena_id_token")).toBeNull();
    expect(screen.queryByTestId("page-content")).not.toBeInTheDocument();
  });
});
