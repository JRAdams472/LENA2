import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor, act, within } from "@testing-library/react";
import { usePathname, useRouter } from "next/navigation";
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
const mockedUseRouter = useRouter as jest.Mock;
const mockPush = jest.fn();

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
    mockedUseRouter.mockReturnValue({ push: mockPush });
    mockPush.mockReset();
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
              {
                id: "4",
                kind: "ITEM_EXPIRING",
                title: "Milk expires soon",
                body: "Milk expires Oct 2 — use it or add a replacement to your grocery list.",
                itemId: "9",
                createdAt: "2026-09-20T10:00:00Z",
                actor: null,
              },
              {
                id: "5",
                kind: "PROTEIN_DEFROST",
                title: "Defrost protein for Oct 3",
                body: "Roast calls for 10.0 lb of protein.",
                recipeId: "7",
                createdAt: "2026-09-20T10:00:00Z",
                actor: null,
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

  it("pins Household and Profile in a bottom secondary zone", async () => {
    signIn();
    renderLayout();

    await waitFor(() =>
      expect(screen.getByTestId("page-content")).toBeInTheDocument()
    );

    const nav = screen.getAllByLabelText("secondary navigation")[0];
    expect(within(nav).getByText("Household")).toBeInTheDocument();
    expect(within(nav).getByText("Profile")).toBeInTheDocument();

    // The admin section appears once /me resolves.
    expect(await within(nav).findByText("Administration")).toBeInTheDocument();
    expect(await within(nav).findByText("Users")).toBeInTheDocument();

    // The zone renders below the main navigation list.
    const mainNav = screen.getAllByLabelText("main navigation")[0];
    expect(
      mainNav.compareDocumentPosition(nav) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();
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
    expect(screen.getAllByText(/ago|just now/).length).toBeGreaterThan(0);

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

  it("renders reminder notifications with server text and actions", async () => {
    signIn();
    renderLayout();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "notifications" }));
    });

    // Server-rendered title/body are used for scheduled reminders.
    expect(await screen.findByText("Milk expires soon")).toBeInTheDocument();
    expect(
      screen.getByText(/use it or add a replacement/)
    ).toBeInTheDocument();
    expect(screen.getByText("Defrost protein for Oct 3")).toBeInTheDocument();

    // The expiry row offers the replacement action.
    mockFetch.mockResolvedValueOnce(
      gql({ addItemToCurrentGroceryList: { id: "11" } })
    );
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Add to list" }));
    });
    expect(
      mockFetch.mock.calls.some(([, init]) =>
        (init as RequestInit).body
          ?.toString()
          .includes("addItemToCurrentGroceryList")
      )
    ).toBe(true);
    await waitFor(() => expect(mockPush).toHaveBeenCalledWith("/grocery-lists"));
  });

  it("deep-links reminder notifications and exposes settings", async () => {
    signIn();
    renderLayout();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "notifications" }));
    });
    await screen.findByText("Defrost protein for Oct 3");

    // recipeId deep-links to the recipe page.
    await act(async () => {
      fireEvent.click(screen.getByText("Defrost protein for Oct 3"));
    });
    expect(mockPush).toHaveBeenCalledWith("/recipes/7");

    // Settings entry navigates to the preferences page.
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "notifications" }));
    });
    fireEvent.click(await screen.findByText("Notification settings"));
    expect(mockPush).toHaveBeenCalledWith("/notifications");
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
