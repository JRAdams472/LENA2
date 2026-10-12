import "@testing-library/jest-dom";
import { render, screen } from "@testing-library/react";
import PrivacyPage from "@/app/privacy/page";

jest.mock("next/navigation", () => ({
  useRouter: jest.fn(() => ({ replace: jest.fn(), push: jest.fn() })),
  usePathname: jest.fn(() => "/privacy"),
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

describe("public privacy page", () => {
  it("states what is collected, how it is used, and the no-sale clause", () => {
    render(<PrivacyPage />);

    expect(
      screen.getByRole("heading", { name: /privacy policy/i, level: 1 })
    ).toBeInTheDocument();

    // Collected data — names, social-login email, push tokens.
    expect(screen.getByText(/name and email/i)).toBeInTheDocument();
    expect(screen.getByText(/social sign-in provider/i)).toBeInTheDocument();
    expect(
      screen.getByText(/push notification tokens/i)
    ).toBeInTheDocument();

    // Uses — authentication and utility notifications.
    expect(screen.getByText(/authenticate you/i)).toBeInTheDocument();
    expect(
      screen.getByText(/utility notifications/i)
    ).toBeInTheDocument();

    // The clause store review checks for verbatim.
    expect(
      screen.getByText(/do not sell your data to third parties/i)
    ).toBeInTheDocument();
  });

  it("is reachable from the marketing chrome", () => {
    render(<PrivacyPage />);
    const links = screen.getAllByRole("link", { name: /^privacy$/i });
    expect(links.length).toBeGreaterThan(0);
    expect(links.every((l) => l.getAttribute("href") === "/privacy")).toBe(
      true
    );
  });
});
