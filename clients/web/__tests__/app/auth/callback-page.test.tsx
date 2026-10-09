import "@testing-library/jest-dom";
import { render, screen, waitFor } from "@testing-library/react";
import OAuthCallbackPage from "@/app/auth/[provider]/callback/page";
import {
  oauthNonceKey,
  oauthStateKey,
  oauthVerifierKey,
} from "@/lib/oauth";
import { useAuth } from "@/app/auth/AuthProvider";
import { useParams, useRouter, useSearchParams } from "next/navigation";

const mockPush = jest.fn();
const mockSignIn = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: jest.fn(),
  useParams: jest.fn(),
  useSearchParams: jest.fn(),
}));

jest.mock("../../../app/auth/AuthProvider", () => ({
  useAuth: jest.fn(),
}));

const mockedParams = useParams as jest.Mock;
const mockedSearch = useSearchParams as jest.Mock;
const mockedRouter = useRouter as jest.Mock;
const mockedAuth = useAuth as jest.Mock;

function setup({
  provider = "discord",
  search = "",
  authenticated = false,
}: {
  provider?: string;
  search?: string;
  authenticated?: boolean;
} = {}) {
  mockedParams.mockReturnValue({ provider });
  mockedSearch.mockReturnValue(new URLSearchParams(search));
  mockedRouter.mockReturnValue({ push: mockPush });
  mockedAuth.mockReturnValue({
    signInWithProvider: mockSignIn,
    isAuthenticated: authenticated,
  });
  return render(<OAuthCallbackPage />);
}

// The effect clears these keys before validating, so a fixture helper
// writes them per test.
function seedSession(state = "st", nonce = "nc", verifier = "ver") {
  window.sessionStorage.setItem(oauthStateKey("discord"), state);
  window.sessionStorage.setItem(oauthNonceKey("discord"), nonce);
  window.sessionStorage.setItem(oauthVerifierKey("discord"), verifier);
}

describe("OAuth callback page", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    window.sessionStorage.clear();
    mockSignIn.mockResolvedValue(undefined);
  });

  it("shows an error for an unknown provider", () => {
    setup({ provider: "gitlab" });
    expect(screen.getByText("Unknown sign-in provider.")).toBeInTheDocument();
    expect(mockSignIn).not.toHaveBeenCalled();
  });

  it("reports a provider-denied redirect", async () => {
    seedSession();
    setup({ search: "error=access_denied" });
    await waitFor(() =>
      expect(
        screen.getByText("Sign-in was cancelled or denied.")
      ).toBeInTheDocument()
    );
    expect(mockSignIn).not.toHaveBeenCalled();
  });

  it("rejects a response missing the code", async () => {
    seedSession();
    setup({ search: "state=st" });
    await waitFor(() =>
      expect(
        screen.getByText("Invalid sign-in response. Please try again.")
      ).toBeInTheDocument()
    );
    expect(mockSignIn).not.toHaveBeenCalled();
  });

  it("rejects a state mismatch", async () => {
    seedSession("expected-state");
    setup({ search: "code=abc&state=forged" });
    await waitFor(() =>
      expect(
        screen.getByText("Invalid sign-in response. Please try again.")
      ).toBeInTheDocument()
    );
    expect(mockSignIn).not.toHaveBeenCalled();
  });

  it("exchanges a valid code and redirects home", async () => {
    seedSession("st", "nc", "ver");
    setup({ search: "code=abc&state=st" });
    await waitFor(() =>
      expect(mockSignIn).toHaveBeenCalledWith("discord", "abc", "nc", "ver")
    );
    await waitFor(() => expect(mockPush).toHaveBeenCalledWith("/"));
    // The one-shot PKCE artifacts are cleared from session storage.
    expect(window.sessionStorage.getItem(oauthStateKey("discord"))).toBeNull();
    expect(window.sessionStorage.getItem(oauthVerifierKey("discord"))).toBeNull();
  });

  it("surfaces an exchange failure", async () => {
    mockSignIn.mockRejectedValue(new Error("boom"));
    seedSession("st", "nc", "ver");
    setup({ search: "code=abc&state=st" });
    await waitFor(() =>
      expect(
        screen.getByText("Sign-in failed. Please try again.")
      ).toBeInTheDocument()
    );
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("redirects already-authenticated users home", async () => {
    setup({ provider: "gitlab", authenticated: true });
    await waitFor(() => expect(mockPush).toHaveBeenCalledWith("/"));
  });
});
