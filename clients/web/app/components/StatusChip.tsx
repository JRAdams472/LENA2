"use client";

import Chip, { ChipProps } from "@mui/material/Chip";
import { useTheme } from "@mui/material/styles";
import { paletteFor } from "@/lib/themeVars";

export type StatusTone = "primary" | "success" | "error" | "neutral";

interface StatusChipProps extends Omit<ChipProps, "color" | "variant"> {
  tone?: StatusTone;
}

// design.md §6 — status markers are soft chips: 12% tinted background,
// darkened-tone label, hairline. Use for state labels (active, role,
// reason), not for actions or navigation.
export default function StatusChip({
  tone = "neutral",
  sx,
  ...props
}: StatusChipProps) {
  const theme = useTheme();
  const palette = paletteFor(theme);
  const token =
    tone === "neutral"
      ? palette.text.secondary
      : palette[tone].main;
  const label =
    tone === "neutral"
      ? palette.text.secondary
      : tone === "primary"
        ? palette.primary.dark
        : palette[tone].dark;
  return (
    <Chip
      size="small"
      {...props}
      sx={{
        bgcolor: `color-mix(in srgb, ${token} 12%, transparent)`,
        color: label,
        fontWeight: 500,
        "& .MuiChip-label": { px: 1.25 },
        ...sx,
      }}
    />
  );
}
