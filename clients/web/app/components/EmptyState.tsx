"use client";

import { ReactNode, createElement } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Paper from "@mui/material/Paper";
import { useTheme } from "@mui/material/styles";
import { SvgIconComponent } from "@mui/icons-material";
import { paletteFor } from "@/lib/themeVars";

interface EmptyStateProps {
  icon?: SvgIconComponent;
  title: string;
  description?: string;
  action?: ReactNode;
  // Compact variant for inside an existing card (no own Paper shell).
  compact?: boolean;
}

// design.md §6 — the shared empty surface: 40px icon at 35% accent,
// title + one supporting line, optional CTA.
export default function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  compact = false,
}: EmptyStateProps) {
  const theme = useTheme();
  const content = (
    <>
      {Icon &&
        // Icons are memo() objects, not functions — createElement, not <Icon/>.
        createElement(Icon, {
          sx: {
            fontSize: 40,
            color: `color-mix(in srgb, ${paletteFor(theme).primary.main} 35%, transparent)`,
            mb: 1,
          },
        })}
      <Typography variant="subtitle1" gutterBottom>
        {title}
      </Typography>
      {description && (
        <Typography variant="body2" color="text.secondary" sx={{ mb: action ? 2 : 0 }}>
          {description}
        </Typography>
      )}
      {action}
    </>
  );

  if (compact) {
    return (
      <Box sx={{ py: 3, textAlign: "center" }}>{content}</Box>
    );
  }
  return (
    <Paper variant="outlined" sx={{ p: 4, textAlign: "center" }}>
      {content}
    </Paper>
  );
}
