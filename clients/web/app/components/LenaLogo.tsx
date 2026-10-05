"use client";

import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import { useTheme } from "@mui/material/styles";

interface LenaLogoProps {
  size?: number;
  iconColor?: string;
  textColor?: string;
  showWordmark?: boolean;
}

export default function LenaLogo({
  size = 28,
  iconColor,
  textColor,
  showWordmark = true,
}: LenaLogoProps) {
  const theme = useTheme();
  const resolvedIconColor = iconColor ?? theme.palette.primary.dark;
  const resolvedTextColor = textColor ?? theme.palette.text.primary;
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
      <Box
        component="svg"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth={2.5}
        strokeLinecap="round"
        strokeLinejoin="round"
        sx={{ width: size, height: size, color: resolvedIconColor, flexShrink: 0 }}
      >
        <path d="M2 20h20" />
        <path d="M20 16A8 8 0 0 0 4 16v0" />
        <path d="M12 4v4" />
        <circle cx="12" cy="3" r="1" fill="currentColor" />
      </Box>
      {showWordmark && (
        <Typography
          component="span"
          sx={{
            fontWeight: 600,
            letterSpacing: "0.05em",
            fontSize: size * 0.72,
            color: resolvedTextColor,
            lineHeight: 1,
          }}
        >
          LENA
        </Typography>
      )}
    </Box>
  );
}
