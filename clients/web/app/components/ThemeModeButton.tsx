"use client";

import IconButton from "@mui/material/IconButton";
import Tooltip from "@mui/material/Tooltip";
import DarkModeOutlinedIcon from "@mui/icons-material/DarkModeOutlined";
import LightModeOutlinedIcon from "@mui/icons-material/LightModeOutlined";
import { useColorScheme } from "@mui/material/styles";

// Quick light↔dark flip in the AppBar. The full three-way choice
// (incl. "System") lives on /profile; the icon reflects the resolved
// mode so 'system' users still see what they'll get.
export default function ThemeModeButton() {
  const { mode, systemMode, setMode } = useColorScheme();
  const resolved =
    mode === "system" || mode === undefined ? (systemMode ?? "light") : mode;
  const next = resolved === "dark" ? "light" : "dark";
  return (
    <Tooltip title={`Switch to ${next} mode`}>
      <IconButton
        color="inherit"
        aria-label={`Switch to ${next} mode`}
        onClick={() => setMode(next)}
      >
        {resolved === "dark" ? (
          <LightModeOutlinedIcon />
        ) : (
          <DarkModeOutlinedIcon />
        )}
      </IconButton>
    </Tooltip>
  );
}
