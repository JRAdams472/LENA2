"use client";

import { ThemeProvider, createTheme } from "@mui/material/styles";
import CssBaseline from "@mui/material/CssBaseline";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReactNode, useState } from "react";
import { GoogleOAuthProvider } from "@react-oauth/google";
import { ApiError } from "@/lib/api";
import { AuthProvider } from "@/app/auth/AuthProvider";

const GOOGLE_CLIENT_ID = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID ?? "";
const CLIENT_ID_PLACEHOLDER = "__YOUR_GOOGLE_CLIENT_ID__";

if (!GOOGLE_CLIENT_ID || GOOGLE_CLIENT_ID === CLIENT_ID_PLACEHOLDER) {
  throw new Error(
    "NEXT_PUBLIC_GOOGLE_CLIENT_ID is not configured. " +
    "Copy frontend/.env.example to frontend/.env.local and set it to your real Google OAuth web client ID."
  );
}

const theme = createTheme();

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
    <AuthProvider>
      <GoogleOAuthProvider clientId={GOOGLE_CLIENT_ID}>
        <QueryClientProvider client={queryClient}>
          <ThemeProvider theme={theme}>
            <CssBaseline />
            {children}
          </ThemeProvider>
        </QueryClientProvider>
      </GoogleOAuthProvider>
    </AuthProvider>
  );
}
