import "@testing-library/jest-dom";
import { render, screen, waitFor } from "@testing-library/react";
import LandingPage from "@/app/components/marketing/LandingPage";

const mockReplace = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: jest.fn(() => ({ replace: mockReplace, push: jest.fn() })),
  usePathname: jest.fn(() => "/"),
}));

// jest.mock specifiers are not rewritten by the SWC path transform, so
// the "@/..." alias cannot be used here — mock the resolved path instead.
const mockUseAuth = jest.fn();
jest.mock("../../app/auth/AuthProvider", () => ({
  useAuth: () => mockUseAuth(),
}));

jest.mock("@react-oauth/google", () => ({
  GoogleLogin: () => <button data-testid="google-login">Sign in with Google</button>,
  googleLogout: jest.fn(),
  GoogleOAuthProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

beforeEach(() => {
  mockReplace.mockReset();
  mockUseAuth.mockReset();
});

describe("public landing page", () => {
  it("redirects authenticated visitors to /member", async () => {
    mockUseAuth.mockReturnValue({ isAuthenticated: true, isRestoring: false });
    render(<LandingPage />);
    await waitFor(() => expect(mockReplace).toHaveBeenCalledWith("/member"));
  });

  it("renders the hero for anonymous visitors", async () => {
    mockUseAuth.mockReturnValue({ isAuthenticated: false, isRestoring: false });
    render(<LandingPage />);
    expect(
      await screen.findByRole("heading", { name: /household kitchen/i })
    ).toBeInTheDocument();
    expect(mockReplace).not.toHaveBeenCalled();
  });
});
