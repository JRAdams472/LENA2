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
    background: { default: "#f8fafc" },
  },
  shape: { borderRadius: 10 },
  components: {
    MuiPaper: {
      styleOverrides: {
        root: { boxShadow: "0 4px 12px rgba(0,0,0,0.05)" },
      },
    },
    MuiListItemButton: {
      styleOverrides: {
        root: ({ theme }) => ({
          borderRadius: theme.shape.borderRadius,
          marginLeft: theme.spacing(1),
          marginRight: theme.spacing(1),
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
        root: { color: "#4b5563", minWidth: 40 },
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
