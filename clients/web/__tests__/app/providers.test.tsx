import "@testing-library/jest-dom";
import { render, screen, waitFor } from "@testing-library/react";
import { useQuery } from "@tanstack/react-query";
import AppBar from "@mui/material/AppBar";
import Button from "@mui/material/Button";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemIcon from "@mui/material/ListItemIcon";
import Paper from "@mui/material/Paper";
import Providers from "@/app/providers";
import { ApiError } from "@/lib/api";

jest.mock("@react-oauth/google", () => ({
  GoogleLogin: () => <button data-testid="google-login">Sign in with Google</button>,
  googleLogout: jest.fn(),
  useGoogleOneTapLogin: jest.fn(),
  GoogleOAuthProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

jest.mock("../../app/auth/SilentReAuth", () => () => null);

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

  it("applies the component style overrides", () => {
    // Mounting the themed components executes every styleOverrides
    // callback — including the dark-mode applyStyles branches.
    render(
      <Providers>
        <Paper data-testid="paper">p</Paper>
        <AppBar data-testid="appbar">a</AppBar>
        <Button>b</Button>
        <ListItemButton selected>
          <ListItemIcon>i</ListItemIcon>item
        </ListItemButton>
      </Providers>
    );
    expect(screen.getByTestId("paper")).toBeInTheDocument();
    expect(screen.getByTestId("appbar")).toBeInTheDocument();
  });

  it("does not retry client errors from the shared query client", async () => {
    const queryFn = jest.fn().mockRejectedValue(new ApiError(404, "nope"));
    function Probe() {
      useQuery({ queryKey: ["probe-404"], queryFn });
      return null;
    }
    render(
      <Providers>
        <Probe />
      </Providers>
    );
    await waitFor(() => expect(queryFn).toHaveBeenCalledTimes(1));
  });

  it("retries server errors up to the cap", async () => {
    const queryFn = jest
      .fn()
      .mockRejectedValue(new ApiError(503, "unavailable"));
    function Probe() {
      useQuery({
        queryKey: ["probe-503"],
        queryFn,
        retryDelay: 1,
      });
      return null;
    }
    render(
      <Providers>
        <Probe />
      </Providers>
    );
    await waitFor(() => expect(queryFn.mock.calls.length).toBeGreaterThan(1), {
      timeout: 3000,
    });
  });
});
