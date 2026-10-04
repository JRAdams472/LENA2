import type { Metadata } from "next";
import { ReactNode } from "react";
import { AppRouterCacheProvider } from "@mui/material-nextjs/v15-appRouter";
import Providers from "./providers";
import AdminLayout from "./components/AdminLayout";

export const metadata: Metadata = {
  title: "LENA Admin",
  description: "LENA Inventory and Wine Admin",
};

// Nonce-based CSP requires every page to render per-request — a static
// shell has no request nonce to stamp on script tags (see proxy.ts).
export const dynamic = "force-dynamic";

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>
        <AppRouterCacheProvider>
          <Providers>
            <AdminLayout>{children}</AdminLayout>
          </Providers>
        </AppRouterCacheProvider>
      </body>
    </html>
  );
}
