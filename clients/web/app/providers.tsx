"use client";

import { ThemeProvider, createTheme } from "@mui/material/styles";
// Augments `Theme` so `theme.vars` is non-optional — cssVariables is always
// enabled in this app.
import "@mui/material/themeCssVarsAugmentation";
import CssBaseline from "@mui/material/CssBaseline";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReactNode, useState } from "react";
import { GoogleOAuthProvider } from "@react-oauth/google";
import { ApiError } from "@/lib/api";
import { googleOAuthConfigured } from "@/lib/oauth";
import { AuthProvider } from "@/app/auth/AuthProvider";
import SilentReAuth from "@/app/auth/SilentReAuth";

// Google OAuth is optional: without a configured client ID the GIS
// provider, one-tap re-auth, and the login button all stay off, but the
// app must still render. (This used to throw at module scope, blanking
// every route — M14.)
const GOOGLE_CLIENT_ID = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID ?? "";
const googleEnabled = googleOAuthConfigured();

const theme = createTheme({
  cssVariables: { colorSchemeSelector: "class" },
  colorSchemes: {
    light: {
      palette: {
        primary: { main: "#7C9473", light: "#A4B79C", dark: "#5F7A57" },
        background: { default: "#FAF6EF", paper: "#FFFDF8" },
        success: { main: "#8A9A5B", dark: "#6E7B45", light: "#A9B87F" },
        info: { main: "#8A9A5B", dark: "#6E7B45", light: "#A9B87F" },
        error: { main: "#B3543F" },
        divider: "#E5DFD3",
        text: { primary: "#3E3E34", secondary: "#6B6B5E" },
      },
    },
    // design.md §2 — warm-charcoal dark scheme; accents lift for contrast,
    // hairlines carry elevation instead of shadows.
    dark: {
      palette: {
        primary: {
          main: "#93A88B",
          light: "#AFC2A8",
          dark: "#7C9473",
          contrastText: "#211F1A",
        },
        background: { default: "#211F1A", paper: "#2B2922" },
        success: {
          main: "#A9B87F",
          dark: "#8A9A5B",
          light: "#C3CF9E",
          contrastText: "#211F1A",
        },
        info: {
          main: "#A9B87F",
          dark: "#8A9A5B",
          light: "#C3CF9E",
          contrastText: "#211F1A",
        },
        error: { main: "#D08A77" },
        divider: "#3A372E",
        text: { primary: "#EFEAE0", secondary: "#B5AE9D" },
      },
    },
  },
  shape: { borderRadius: 10 },
  // design.md §8 — 150ms micro / 250ms component / 400ms page entrances,
  // decelerate easing throughout.
  transitions: {
    duration: {
      shortest: 150,
      shorter: 200,
      short: 250,
      standard: 250,
      complex: 400,
      enteringScreen: 250,
      leavingScreen: 150,
    },
    easing: {
      easeOut: "cubic-bezier(0.2, 0, 0, 1)",
      easeInOut: "cubic-bezier(0.4, 0, 0.2, 1)",
      easeIn: "cubic-bezier(0.4, 0, 1, 1)",
      sharp: "cubic-bezier(0.4, 0, 0.6, 1)",
    },
  },
  // design.md §3 — Nunito is the shared family (mobile bundles the same TTFs);
  // --font-nunito is set by next/font/local in layout.tsx.
  typography: {
    fontFamily:
      "var(--font-nunito), -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
    h1: { fontSize: "1.25rem", fontWeight: 700 },
    h2: { fontSize: "1.125rem", fontWeight: 600 },
    h3: { fontSize: "0.875rem", fontWeight: 600 },
    h4: { fontSize: "1.25rem", fontWeight: 700 },
    h5: { fontSize: "1.125rem", fontWeight: 600 },
    h6: { fontSize: "0.875rem", fontWeight: 600 },
    body1: { fontSize: "0.875rem", lineHeight: 1.5 },
    body2: { fontSize: "0.875rem", lineHeight: 1.5 },
    subtitle1: { fontSize: "0.9375rem", fontWeight: 600 },
    subtitle2: { fontSize: "0.8125rem", fontWeight: 500 },
    caption: { fontSize: "0.75rem", fontWeight: 500 },
    button: { fontWeight: 600 },
  },
  components: {
    MuiCssBaseline: {
      // Raw next/link anchors otherwise render the browser default blue,
      // which clashes with the warm palette; MUI components keep their
      // own colors since class selectors beat this element rule.
      styleOverrides: (theme) => ({
        a: { color: theme.vars.palette.primary.dark },
        "::selection": {
          backgroundColor: `color-mix(in srgb, ${theme.vars.palette.primary.main} 28%, transparent)`,
        },
      }),
    },
    MuiPaper: {
      styleOverrides: {
        root: ({ theme }) => [
          { boxShadow: "0 4px 12px rgba(62,62,52,0.06)" },
          // design.md §2 — hairlines carry elevation in dark mode.
          theme.applyStyles("dark", {
            boxShadow: "none",
            border: `1px solid ${theme.vars.palette.divider}`,
          }),
        ],
      },
    },
    MuiAppBar: {
      styleOverrides: {
        root: ({ theme }) => [
          {
            backgroundColor: theme.vars.palette.primary.main,
            color: theme.vars.palette.primary.contrastText,
            boxShadow: "none",
            borderBottom: `1px solid ${theme.vars.palette.primary.dark}`,
          },
          // The signature sage band deepens in dark mode (design.md §2).
          theme.applyStyles("dark", {
            backgroundColor: "#5F7A57",
            color: "#FFFDF8",
            borderBottom: `1px solid ${theme.vars.palette.divider}`,
          }),
        ],
      },
    },
    MuiListItemButton: {
      styleOverrides: {
        root: ({ theme }) => [
          {
            borderRadius: theme.shape.borderRadius,
            marginLeft: theme.spacing(1.5),
            marginRight: theme.spacing(1.5),
            padding: "10px 16px",
            "&.Mui-selected": {
              backgroundColor: `color-mix(in srgb, ${theme.vars.palette.primary.main} 10%, transparent)`,
              color: theme.vars.palette.primary.main,
              "& .MuiListItemIcon-root": {
                color: theme.vars.palette.primary.main,
              },
              "&:hover": {
                backgroundColor: `color-mix(in srgb, ${theme.vars.palette.primary.main} 14%, transparent)`,
              },
            },
          },
        ],
      },
    },
    MuiButton: {
      styleOverrides: {
        // design.md §8 — pressed feedback settles at scale(0.98).
        root: ({ theme }) => ({
          transition: theme.transitions.create(
            ["background-color", "transform", "box-shadow", "border-color"],
            { duration: theme.transitions.duration.shortest }
          ),
          "&:active": { transform: "scale(0.98)" },
        }),
      },
    },
    MuiListItemIcon: {
      styleOverrides: {
        root: ({ theme }) => ({
          color: theme.vars.palette.text.secondary,
          minWidth: 40,
        }),
      },
    },
  },
});

function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        retry: (failureCount, error) => {
          if (error instanceof ApiError && error.status < 500) return false;
          return failureCount < 2;
        },
      },
    },
  });
}

export default function Providers({ children }: { children: ReactNode }) {
  const [queryClient] = useState(makeQueryClient);

  // SilentReAuth needs the GIS context; it mounts only when Google is
  // configured, matching the provider gate below.
  const tree = (
    <AuthProvider>
      <QueryClientProvider client={queryClient}>
        <ThemeProvider theme={theme}>
          <CssBaseline />
          {googleEnabled && <SilentReAuth />}
          {children}
        </ThemeProvider>
      </QueryClientProvider>
    </AuthProvider>
  );

  if (!googleEnabled) return tree;
  return (
    <GoogleOAuthProvider clientId={GOOGLE_CLIENT_ID}>{tree}</GoogleOAuthProvider>
  );
}
