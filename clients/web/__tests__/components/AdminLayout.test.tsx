import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
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
  localStorage.setItem("lena_id_token", makeToken(email, future));
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

describe("AdminLayout", () => {
  beforeEach(() => {
    localStorage.clear();
    mockedUsePathname.mockReturnValue("/");
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
    expect(localStorage.getItem("lena_id_token")).toBeNull();
    expect(screen.queryByTestId("page-content")).not.toBeInTheDocument();
  });
});
