import type { Metadata } from "next";
import Link from "next/link";
import Box from "@mui/material/Box";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import { DOCS_NAV } from "@/app/components/marketing/docsModel";
import { DocPage } from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Docs — LENA",
  description:
    "Every part of LENA, documented: web app, mobile app, meal planning, pantry, grocery lists, wine cellar, and the AI assistant.",
};

export default function DocsIndex() {
  return (
    <DocPage
      title="How LENA works"
      intro="Guides for every part of the app, from planning the week's dinners to scanning groceries with your phone."
    >
      {DOCS_NAV.map((group) => (
        <Box key={group.label} sx={{ mt: 4 }}>
          <Typography variant="h6" sx={{ fontWeight: 600 }} gutterBottom>
            {group.label}
          </Typography>
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
              gap: 1.5,
            }}
          >
            {group.items
              .filter((item) => item.href !== "/docs")
              .map((item) =>
                item.live ? (
                  <Paper
                    key={item.href}
                    component={Link}
                    href={item.href}
                    variant="outlined"
                    sx={{
                      p: 2,
                      textDecoration: "none",
                      color: "text.primary",
                      display: "block",
                      "&:hover": { borderColor: "primary.main" },
                    }}
                  >
                    <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
                      {item.label}
                    </Typography>
                  </Paper>
                ) : (
                  <Paper
                    key={item.href}
                    variant="outlined"
                    sx={{ p: 2, display: "block", opacity: 0.55 }}
                  >
                    <Typography variant="subtitle2" color="text.secondary">
                      {item.label}
                    </Typography>
                  </Paper>
                )
              )}
          </Box>
        </Box>
      ))}
    </DocPage>
  );
}
