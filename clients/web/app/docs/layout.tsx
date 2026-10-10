import { ReactNode } from "react";
import Box from "@mui/material/Box";
import MarketingShell from "@/app/components/marketing/MarketingShell";
import DocsNav from "@/app/components/marketing/DocsNav";

export default function DocsLayout({ children }: { children: ReactNode }) {
  return (
    <MarketingShell>
      <Box sx={{ display: "flex", alignItems: "stretch" }}>
        <DocsNav />
        <Box
          sx={{
            flexGrow: 1,
            minWidth: 0,
            px: { xs: 2, md: 4 },
            py: { xs: 3, md: 5 },
            maxWidth: 1120,
          }}
        >
          {children}
        </Box>
      </Box>
    </MarketingShell>
  );
}
