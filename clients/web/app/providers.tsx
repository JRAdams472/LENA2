"use client";

import { ThemeProvider, alpha, createTheme } from "@mui/material/styles";
import CssBaseline from "@mui/material/CssBaseline";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReactNode, useState } from "react";
import { GoogleOAuthProvider } from "@react-oauth/google";
import { ApiError } from "@/lib/api";
import { AuthProvider } from "@/app/auth/AuthProvider";
import SilentReAuth from "@/app/auth/SilentReAuth";

const GOOGLE_CLIENT_ID = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID ?? "";
const CLIENT_ID_PLACEHOLDER = "__YOUR_GOOGLE_CLIENT_ID__";

if (!GOOGLE_CLIENT_ID || GOOGLE_CLIENT_ID === CLIENT_ID_PLACEHOLDER) {
  throw new Error(
    "NEXT_PUBLIC_GOOGLE_CLIENT_ID is not configured. " +
    "Copy frontend/.env.example to frontend/.env.local and set it to your real Google OAuth web client ID."
  );
}

const theme = createTheme({
  palette: {
    primary: { main: "#7C9473", light: "#A4B79C", dark: "#5F7A57" },
    background: { default: "#FAF6EF", paper: "#FFFDF8" },
    success: { main: "#8A9A5B", dark: "#6E7B45", light: "#A9B87F" },
    info: { main: "#8A9A5B", dark: "#6E7B45", light: "#A9B87F" },
    divider: "#E5DFD3",
    text: { primary: "#3E3E34", secondary: "#6B6B5E" },
  },
  shape: { borderRadius: 10 },
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
      styleOverrides: { a: { color: "#5F7A57" } },
    },
    MuiPaper: {
      styleOverrides: {
        root: { boxShadow: "0 4px 12px rgba(62,62,52,0.06)" },
      },
    },
    MuiAppBar: {
      styleOverrides: {
        root: ({ theme }) => ({
          backgroundColor: theme.palette.primary.main,
          color: theme.palette.primary.contrastText,
          boxShadow: "none",
          borderBottom: `1px solid ${theme.palette.primary.dark}`,
        }),
      },
    },
    MuiListItemButton: {
      styleOverrides: {
        root: ({ theme }) => ({
          borderRadius: theme.shape.borderRadius,
          marginLeft: theme.spacing(1.5),
          marginRight: theme.spacing(1.5),
          padding: "10px 16px",
          "&.Mui-selected": {
            backgroundColor: alpha(theme.palette.primary.main, 0.1),
            color: theme.palette.primary.main,
            "& .MuiListItemIcon-root": {
              color: theme.palette.primary.main,
            },
            "&:hover": {
              backgroundColor: alpha(theme.palette.primary.main, 0.14),
            },
          },
        }),
      },
    },
    MuiListItemIcon: {
      styleOverrides: {
        root: ({ theme }) => ({
          color: theme.palette.text.secondary,
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

  return (
    <GoogleOAuthProvider clientId={GOOGLE_CLIENT_ID}>
      <AuthProvider>
        <QueryClientProvider client={queryClient}>
          <ThemeProvider theme={theme}>
            <CssBaseline />
            <SilentReAuth />
            {children}
          </ThemeProvider>
        </QueryClientProvider>
      </AuthProvider>
    </GoogleOAuthProvider>
  );
}
