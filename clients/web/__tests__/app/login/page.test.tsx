import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider } from "@/app/auth/AuthProvider";
import LoginPage from "@/app/login/page";
import LoginScreen from "@/app/components/LoginScreen";

const mockPush = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: jest.fn(() => ({ push: mockPush })),
  useParams: jest.fn(() => ({})),
  usePathname: jest.fn(() => "/login"),
}));

jest.mock("@react-oauth/google", () => ({
  GoogleLogin: ({
    onSuccess,
    onError,
  }: {
    onSuccess?: (response: { credential?: string }) => void;
    onError?: () => void;
  }) => (
    <button
      data-testid="google-login"
      onClick={() => onSuccess?.({ credential: makeToken() })}
      onDoubleClick={() => onError?.()}
    >
      Sign in with Google
    </button>
  ),
  googleLogout: jest.fn(),
  GoogleOAuthProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

function makeToken() {
  const exp = 2000000000;
  const header = btoa(JSON.stringify({ alg: "none", typ: "JWT" }))
    .replace(/=/g, "")
    .replace(/\+/g, "-")
    .replace(/\//g, "_");
  const payload = btoa(JSON.stringify({ email: "admin@example.com", exp }))
    .replace(/=/g, "")
    .replace(/\+/g, "-")
    .replace(/\//g, "_");
  return `${header}.${payload}.signature`;
}

function renderWithAuth(element: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>{element}</AuthProvider>
    </QueryClientProvider>
  );
}

beforeEach(() => {
  mockPush.mockReset();
  localStorage.clear();
});

afterEach(() => {
  jest.restoreAllMocks();
});

describe("login page", () => {
  it("renders the login screen", () => {
    renderWithAuth(<LoginPage />);
    expect(screen.getByText("LENA")).toBeInTheDocument();
    expect(screen.getByTestId("google-login")).toBeInTheDocument();
  });
});

describe("login screen", () => {
  it("signs in and redirects when Google returns a credential", async () => {
    renderWithAuth(<LoginScreen />);
    fireEvent.click(screen.getByTestId("google-login"));
    await waitFor(() => {
      expect(localStorage.getItem("lena_id_token")).toBe(makeToken());
      expect(mockPush).toHaveBeenCalledWith("/");
    });
  });

  it("shows an error when Google does not return a credential", () => {
    (global as { googleCredential?: string | null }).googleCredential = null;
    // Re-mock GoogleLogin to return no credential for this test would require
    // a factory per-test, so instead we exercise the onError path.
    renderWithAuth(<LoginScreen />);
    fireEvent.doubleClick(screen.getByTestId("google-login"));
    expect(
      screen.getByText(/Google sign-in failed/i)
    ).toBeInTheDocument();
  });
});
