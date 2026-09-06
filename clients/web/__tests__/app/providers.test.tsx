import "@testing-library/jest-dom";
import { render, screen } from "@testing-library/react";
import Providers from "@/app/providers";

jest.mock("@react-oauth/google", () => ({
  GoogleLogin: () => <button data-testid="google-login">Sign in with Google</button>,
  googleLogout: jest.fn(),
  GoogleOAuthProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

describe("providers", () => {
  it("renders children with the provider stack", () => {
    // The env var is set in jest.setup.js so Providers should not throw.
    render(
      <Providers>
        <div data-testid="child">hello</div>
      </Providers>
    );
    expect(screen.getByTestId("child")).toBeInTheDocument();
  });
});
