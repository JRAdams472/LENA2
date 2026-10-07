import { Theme } from "@mui/material/styles";

// `theme.vars` only exists when the active theme was created with
// `cssVariables` — components rendered without our ThemeProvider (unit
// tests) get MUI's default theme and would crash on `vars.palette`.
// In the app this always returns the CSS-var palette; bare renders fall
// back to the concrete (light) palette, which is fine for assertions.
export const paletteFor = (theme: Theme) => theme.vars?.palette ?? theme.palette;
