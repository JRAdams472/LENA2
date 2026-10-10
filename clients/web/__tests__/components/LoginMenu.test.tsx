import "@testing-library/jest-dom";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import LoginMenu from "@/app/components/marketing/LoginMenu";

const mockReplace = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: jest.fn(() => ({ replace: mockReplace, push: jest.fn() })),
  usePathname: jest.fn(() => "/"),
}));

const mockUseAuth = jest.fn();
jest.mock("../../app/auth/AuthProvider", () => ({
  useAuth: () => mockUseAuth(),
}));

const mockEnabledProviders = jest.fn(() => [] as string[]);
jest.mock("../../lib/oauth", () => ({
  enabledProviders: () => mockEnabledProviders(),
  startOAuthSignIn: jest.fn(() => Promise.resolve()),
}));

jest.mock("@react-oauth/google", () => ({
  GoogleLogin: () => <button data-testid="google-login">Sign in with Google</button>,
  googleLogout: jest.fn(),
  GoogleOAuthProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

beforeEach(() => {
  mockReplace.mockReset();
  mockUseAuth.mockReturnValue({ isAuthenticated: false, isRestoring: false });
  mockEnabledProviders.mockReturnValue([]);
});

describe("login menu", () => {
  it("opens the popover with the Google sign-in button", async () => {
    render(<LoginMenu />);
    await userEvent.click(screen.getByRole("button", { name: /log in/i }));
    expect(
      await screen.findByTestId("google-login")
    ).toBeInTheDocument();
    expect(screen.getByText(/welcome back/i)).toBeInTheDocument();
  });

  it("routes to /member when auth flips to signed-in", async () => {
    const { rerender } = render(<LoginMenu />);
    mockUseAuth.mockReturnValue({ isAuthenticated: true, isRestoring: false });
    rerender(<LoginMenu />);
    await waitFor(() => expect(mockReplace).toHaveBeenCalledWith("/member"));
  });

  it("does not redirect a member who was already signed in on mount", () => {
    mockUseAuth.mockReturnValue({ isAuthenticated: true, isRestoring: false });
    render(<LoginMenu />);
    expect(mockReplace).not.toHaveBeenCalled();
  });
});
