import "@testing-library/jest-dom";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import MarketingShell from "@/app/components/marketing/MarketingShell";

let mockPathname = "/";

jest.mock("next/navigation", () => ({
  useRouter: jest.fn(() => ({ replace: jest.fn(), push: jest.fn() })),
  usePathname: jest.fn(() => mockPathname),
}));

const mockUseAuth = jest.fn();
jest.mock("../../app/auth/AuthProvider", () => ({
  useAuth: () => mockUseAuth(),
}));

jest.mock("../../lib/oauth", () => ({
  enabledProviders: () => [],
  startOAuthSignIn: jest.fn(() => Promise.resolve()),
}));

jest.mock("@react-oauth/google", () => ({
  GoogleLogin: () => <button data-testid="google-login">Sign in with Google</button>,
  googleLogout: jest.fn(),
  GoogleOAuthProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

beforeEach(() => {
  mockPathname = "/";
  mockUseAuth.mockReset();
  mockUseAuth.mockReturnValue({ isAuthenticated: false, isRestoring: false });
});

describe("marketing shell drawer", () => {
  it("shows site links on non-docs routes", async () => {
    render(<MarketingShell>content</MarketingShell>);
    await userEvent.click(
      screen.getByRole("button", { name: /open navigation/i })
    );
    const nav = await screen.findByRole("navigation", { name: /site/i });
    expect(nav).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /docs/i })).not.toHaveLength(0);
    // Docs group subheaders only appear in the docs tree.
    expect(screen.queryByText(/web app/i)).not.toBeInTheDocument();
  });

  it("shows the docs tree plus site links on /docs routes", async () => {
    mockPathname = "/docs/meal-planning";
    render(<MarketingShell>content</MarketingShell>);
    await userEvent.click(
      screen.getByRole("button", { name: /open navigation/i })
    );
    const nav = await screen.findByRole("navigation", {
      name: /documentation/i,
    });
    expect(nav).toBeInTheDocument();
    // Docs nav groups + a live docs page link + the site section.
    expect(screen.getByText(/web app/i)).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /meal planning/i })
    ).toBeInTheDocument();
    expect(screen.getByText(/^site$/i)).toBeInTheDocument();
  });

  it("marks the current docs page as selected", async () => {
    mockPathname = "/docs/meal-planning";
    render(<MarketingShell>content</MarketingShell>);
    await userEvent.click(
      screen.getByRole("button", { name: /open navigation/i })
    );
    const link = await screen.findByRole("link", { name: /meal planning/i });
    expect(link).toHaveAttribute("aria-current", "page");
  });
});
