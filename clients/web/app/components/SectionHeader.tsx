"use client";

import { ReactNode } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";

interface SectionHeaderProps {
  title: ReactNode;
  // Trailing slot — a link, button, or chip aligned to the title row.
  action?: ReactNode;
}

// design.md §6 — card/section heading row: h6 title with an optional
// trailing action. Keeps the repeated "h6 + link/button" header rows
// visually identical across cards.
export default function SectionHeader({ title, action }: SectionHeaderProps) {
  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 2,
        mb: 1,
      }}
    >
      <Typography variant="h6">{title}</Typography>
      {action}
    </Box>
  );
}
