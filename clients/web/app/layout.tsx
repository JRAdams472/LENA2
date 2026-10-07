import type { Metadata } from "next";
import { ReactNode } from "react";
import localFont from "next/font/local";
import { AppRouterCacheProvider } from "@mui/material-nextjs/v15-appRouter";
import InitColorSchemeScript from "@mui/material/InitColorSchemeScript";
import Providers from "./providers";
import AdminLayout from "./components/AdminLayout";

// Nunito is self-hosted from the same TTFs the mobile app bundles
// (clients/mobile/assets/fonts) — no Google Fonts fetch at build or runtime.
const nunito = localFont({
  src: [
    { path: "./fonts/Nunito-Regular.ttf", weight: "400" },
    { path: "./fonts/Nunito-Medium.ttf", weight: "500" },
    { path: "./fonts/Nunito-SemiBold.ttf", weight: "600" },
    { path: "./fonts/Nunito-Bold.ttf", weight: "700" },
  ],
  variable: "--font-nunito",
  display: "swap",
});

export const metadata: Metadata = {
  title: "LENA Admin",
  description: "LENA Inventory and Wine Admin",
};

// Nonce-based CSP requires every page to render per-request — a static
// shell has no request nonce to stamp on script tags (see proxy.ts).
export const dynamic = "force-dynamic";

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" className={nunito.variable}>
      <body>
        {/* Sets the resolved light/dark class before paint — no flash on
            reload; 'class' matches cssVariables.colorSchemeSelector. */}
        <InitColorSchemeScript attribute="class" defaultMode="system" />
        <AppRouterCacheProvider>
          <Providers>
            <AdminLayout>{children}</AdminLayout>
          </Providers>
        </AppRouterCacheProvider>
      </body>
    </html>
  );
}
