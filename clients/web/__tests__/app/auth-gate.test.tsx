import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { usePathname } from "next/navigation";
import AdminLayout from "@/app/components/AdminLayout";
import { AuthProvider } from "@/app/auth/AuthProvider";

jest.mock("next/navigation");
jest.mock("@react-oauth/google", () => ({
  GoogleLogin: () => <button data-testid="google-login">Sign in with Google</button>,
  googleLogout: jest.fn(),
  GoogleOAuthProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
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

describe("auth gate", () => {
  beforeEach(() => {
    mockedUsePathname.mockReturnValue("/");
    localStorage.clear();
  });

  it("renders the login screen when unauthenticated", () => {
    render(
      <AuthProvider>
        <AdminLayout>
          <div data-testid="app-content">App</div>
        </AdminLayout>
      </AuthProvider>
    );

    expect(screen.getByTestId("google-login")).toBeInTheDocument();
    expect(screen.queryByTestId("app-content")).not.toBeInTheDocument();
  });

  it("renders the app when authenticated", async () => {
    const future = Math.floor(Date.now() / 1000) + 3600;
    localStorage.setItem("lena_id_token", makeToken("test@example.com", future));

    render(
      <AuthProvider>
        <AdminLayout>
          <div data-testid="app-content">App</div>
        </AdminLayout>
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId("app-content")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("google-login")).not.toBeInTheDocument();
  });

  it("treats an expired token as unauthenticated", () => {
    const past = Math.floor(Date.now() / 1000) - 3600;
    localStorage.setItem("lena_id_token", makeToken("old@example.com", past));

    render(
      <AuthProvider>
        <AdminLayout>
          <div data-testid="app-content">App</div>
        </AdminLayout>
      </AuthProvider>
    );

    expect(screen.getByTestId("google-login")).toBeInTheDocument();
    expect(screen.queryByTestId("app-content")).not.toBeInTheDocument();
  });

  it("treats a malformed token as unauthenticated", () => {
    localStorage.setItem("lena_id_token", "not-a-jwt");

    render(
      <AuthProvider>
        <AdminLayout>
          <div data-testid="app-content">App</div>
        </AdminLayout>
      </AuthProvider>
    );

    expect(screen.getByTestId("google-login")).toBeInTheDocument();
    expect(screen.queryByTestId("app-content")).not.toBeInTheDocument();
  });

  it("treats a token without an email claim as unauthenticated", () => {
    const future = Math.floor(Date.now() / 1000) + 3600;
    const header = btoa(JSON.stringify({ alg: "none" }));
    const payload = btoa(JSON.stringify({ sub: "123", exp: future }));
    localStorage.setItem("lena_id_token", `${header}.${payload}.sig`);

    render(
      <AuthProvider>
        <AdminLayout>
          <div data-testid="app-content">App</div>
        </AdminLayout>
      </AuthProvider>
    );

    expect(screen.getByTestId("google-login")).toBeInTheDocument();
    expect(screen.queryByTestId("app-content")).not.toBeInTheDocument();
  });

  it("signs out via the app bar and clears the stored token", async () => {
    const future = Math.floor(Date.now() / 1000) + 3600;
    localStorage.setItem("lena_id_token", makeToken("test@example.com", future));

    render(
      <AuthProvider>
        <AdminLayout>
          <div data-testid="app-content">App</div>
        </AdminLayout>
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId("app-content")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Sign out" }));

    await waitFor(() => {
      expect(screen.getByTestId("google-login")).toBeInTheDocument();
    });
    expect(localStorage.getItem("lena_id_token")).toBeNull();
  });
});
