import "@testing-library/jest-dom";
import { render, screen, fireEvent } from "@testing-library/react";
import { ThemeProvider, createTheme } from "@mui/material/styles";
import ThemeModeButton from "@/app/components/ThemeModeButton";

// jsdom has no matchMedia; useColorScheme reads it for the system scheme.
const matchMediaStub = (query: string) =>
  ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }) as MediaQueryList;

const theme = createTheme({
  cssVariables: { colorSchemeSelector: "class" },
  colorSchemes: { light: { palette: {} }, dark: { palette: {} } },
});

function renderButton() {
  return render(
    <ThemeProvider theme={theme} defaultMode="system">
      <ThemeModeButton />
    </ThemeProvider>
  );
}

describe("ThemeModeButton", () => {
  beforeAll(() => {
    window.matchMedia = matchMediaStub;
  });

  beforeEach(() => {
    localStorage.clear();
    document.documentElement.classList.remove("light", "dark");
  });

  it("offers dark mode first when the system scheme is light", () => {
    renderButton();
    expect(
      screen.getByRole("button", { name: /switch to dark mode/i })
    ).toBeInTheDocument();
  });

  it("flips to dark without reload and persists the choice", () => {
    renderButton();
    fireEvent.click(
      screen.getByRole("button", { name: /switch to dark mode/i })
    );
    expect(localStorage.getItem("mui-mode")).toBe("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(
      screen.getByRole("button", { name: /switch to light mode/i })
    ).toBeInTheDocument();
  });

  it("flips back to light", () => {
    renderButton();
    fireEvent.click(
      screen.getByRole("button", { name: /switch to dark mode/i })
    );
    fireEvent.click(
      screen.getByRole("button", { name: /switch to light mode/i })
    );
    expect(localStorage.getItem("mui-mode")).toBe("light");
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });
});
