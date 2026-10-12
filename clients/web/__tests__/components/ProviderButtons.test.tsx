import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import ProviderButtons from "@/app/components/ProviderButtons";

const mockSignIn = jest.fn();
const mockEnabledProviders = jest.fn<() => string[]>(() => []);
const mockStartOAuthSignIn = jest.fn<(p: string) => Promise<void>>(() =>
  Promise.resolve()
);

jest.mock("../../app/auth/AuthProvider", () => ({
  useAuth: () => ({ signIn: mockSignIn }),
}));

jest.mock("../../lib/oauth", () => ({
  enabledProviders: () => mockEnabledProviders(),
  startOAuthSignIn: (p: string) => mockStartOAuthSignIn(p),
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
      onClick={() => onSuccess?.({ credential: "cred" })}
      onDoubleClick={() => onError?.()}
    >
      Sign in with Google
    </button>
  ),
}));

beforeEach(() => {
  mockSignIn.mockReset();
  mockEnabledProviders.mockReset().mockReturnValue([]);
  mockStartOAuthSignIn.mockReset().mockResolvedValue(undefined);
});

describe("ProviderButtons", () => {
  it("renders only Google when no OAuth providers are configured", () => {
    render(<ProviderButtons />);
    expect(screen.getByTestId("google-login")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /continue with/i })
    ).toBeNull();
  });

  it("stacks configured providers with Apple first at equal prominence", () => {
    mockEnabledProviders.mockReturnValue(["apple", "discord", "facebook"]);
    render(<ProviderButtons />);
    const buttons = screen
      .getAllByRole("button")
      .map((b) => b.textContent);
    expect(buttons).toEqual([
      "Continue with Apple",
      "Sign in with Google",
      "Continue with Discord",
      "Continue with Facebook",
    ]);
  });

  it("starts the OAuth redirect for a provider", () => {
    mockEnabledProviders.mockReturnValue(["discord"]);
    render(<ProviderButtons />);
    fireEvent.click(screen.getByRole("button", { name: "Continue with Discord" }));
    expect(mockStartOAuthSignIn).toHaveBeenCalledWith("discord");
  });

  it("starts the Apple flow through the same redirect helper", () => {
    mockEnabledProviders.mockReturnValue(["apple"]);
    render(<ProviderButtons />);
    fireEvent.click(screen.getByRole("button", { name: "Continue with Apple" }));
    expect(mockStartOAuthSignIn).toHaveBeenCalledWith("apple");
  });

  it("shows an error when the OAuth redirect fails", async () => {
    mockEnabledProviders.mockReturnValue(["discord"]);
    mockStartOAuthSignIn.mockRejectedValue(new Error("popup blocked"));
    render(<ProviderButtons />);
    fireEvent.click(screen.getByRole("button", { name: "Continue with Discord" }));
    await waitFor(() =>
      expect(
        screen.getByText(/could not start sign-in/i)
      ).toBeInTheDocument()
    );
  });

  it("signs in with the Google credential", () => {
    render(<ProviderButtons />);
    fireEvent.click(screen.getByTestId("google-login"));
    expect(mockSignIn).toHaveBeenCalledWith("cred");
  });

  it("shows an error when Google sign-in fails", () => {
    render(<ProviderButtons />);
    fireEvent.doubleClick(screen.getByTestId("google-login"));
    expect(screen.getByText(/Google sign-in failed/i)).toBeInTheDocument();
  });
});
