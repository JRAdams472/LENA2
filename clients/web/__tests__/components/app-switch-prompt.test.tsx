import "@testing-library/jest-dom";
import { render, screen, fireEvent } from "@testing-library/react";
import useMediaQuery from "@mui/material/useMediaQuery";
import AppSwitchPrompt from "@/app/components/AppSwitchPrompt";

jest.mock("@mui/material/useMediaQuery", () => ({
  __esModule: true,
  default: jest.fn(() => false),
}));
const mockUseMediaQuery = useMediaQuery as jest.Mock;

const KEY = "lena_app_prompt_dismissed";

beforeEach(() => {
  window.localStorage.clear();
  mockUseMediaQuery.mockReturnValue(false);
});

describe("AppSwitchPrompt", () => {
  it("renders on phone-sized viewports", () => {
    mockUseMediaQuery.mockReturnValue(true);
    render(<AppSwitchPrompt />);

    expect(screen.getByTestId("app-switch-prompt")).toBeInTheDocument();
  });

  it("stays hidden on larger viewports", () => {
    render(<AppSwitchPrompt />);

    expect(
      screen.queryByTestId("app-switch-prompt")
    ).not.toBeInTheDocument();
  });

  it("stays hidden once dismissed", () => {
    window.localStorage.setItem(KEY, "1");
    mockUseMediaQuery.mockReturnValue(true);
    render(<AppSwitchPrompt />);

    expect(
      screen.queryByTestId("app-switch-prompt")
    ).not.toBeInTheDocument();
  });

  it("remembers dismissal in localStorage and unmounts", () => {
    mockUseMediaQuery.mockReturnValue(true);
    render(<AppSwitchPrompt />);

    fireEvent.click(screen.getByRole("button", { name: /dismiss/i }));

    expect(window.localStorage.getItem(KEY)).toBe("1");
    expect(
      screen.queryByTestId("app-switch-prompt")
    ).not.toBeInTheDocument();
  });
});
