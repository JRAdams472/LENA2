import "@testing-library/jest-dom";
import { render, screen } from "@testing-library/react";
import DisclaimerPage from "@/app/disclaimer/page";

jest.mock("next/navigation", () => ({
  useRouter: jest.fn(() => ({ replace: jest.fn(), push: jest.fn() })),
  usePathname: jest.fn(() => "/disclaimer"),
}));

jest.mock("../../../app/auth/AuthProvider", () => ({
  useAuth: () => ({ isAuthenticated: false, isRestoring: false }),
}));

jest.mock("../../../lib/oauth", () => ({
  enabledProviders: () => [],
  startOAuthSignIn: jest.fn(() => Promise.resolve()),
}));

jest.mock("@react-oauth/google", () => ({
  GoogleLogin: () => (
    <button data-testid="google-login">Sign in with Google</button>
  ),
  googleLogout: jest.fn(),
  GoogleOAuthProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

describe("public disclaimer page", () => {
  it("states the informational-only nature and the liability entity", () => {
    render(<DisclaimerPage />);

    expect(
      screen.getByRole("heading", {
        name: /medical & allergen disclaimer/i,
        level: 1,
      })
    ).toBeInTheDocument();

    // The core legal statements.
    expect(screen.getByText(/not medical, nutritional, or dietary advice/i)).toBeInTheDocument();
    expect(screen.getByText(/incomplete,/i)).toBeInTheDocument();
    expect(screen.getByText(/check the product's actual label/i)).toBeInTheDocument();
    expect(screen.getByText(/Lena's Kitchen accepts no liability/i)).toBeInTheDocument();
  });

  it("is reachable from the marketing chrome", () => {
    render(<DisclaimerPage />);
    const links = screen.getAllByRole("link", { name: /^disclaimer$/i });
    expect(links.length).toBeGreaterThan(0);
    expect(links.every((l) => l.getAttribute("href") === "/disclaimer")).toBe(
      true
    );
  });
});
