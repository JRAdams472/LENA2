"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import CircularProgress from "@mui/material/CircularProgress";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import Menu from "@mui/material/Menu";
import MenuItem from "@mui/material/MenuItem";
import Paper from "@mui/material/Paper";
import Switch from "@mui/material/Switch";
import Typography from "@mui/material/Typography";
import { api, ApiError } from "@/lib/api";
import { NotificationCategoryPreference } from "@/lib/types";

// Mute presets offered by the per-category menu, in milliseconds.
const MUTE_OPTIONS = [
  { label: "1 hour", ms: 60 * 60 * 1000 },
  { label: "8 hours", ms: 8 * 60 * 60 * 1000 },
  { label: "1 day", ms: 24 * 60 * 60 * 1000 },
  { label: "1 week", ms: 7 * 24 * 60 * 60 * 1000 },
];

function prefStatusText(isAll: boolean, enabled: boolean): string {
  if (isAll) {
    return "Pause every notification for a while — nothing is delivered until it expires.";
  }
  return enabled ? "Delivered" : "Turned off";
}

function fmtUntil(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function PrefRow({ pref }: { pref: NotificationCategoryPreference }) {
  const queryClient = useQueryClient();
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [error, setError] = useState<string | null>(null);
  // Render-purity: snapshot "now" once at mount rather than calling
  // new Date() inline; a mute expiring while the page is open is harmless.
  const [mountedAt] = useState(() => Date.now());
  const isAll = pref.category === "_all";
  const muted =
    pref.mutedUntil != null && Date.parse(pref.mutedUntil) > mountedAt;

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ["notification-prefs"] });

  const toggle = useMutation({
    mutationFn: (enabled: boolean) =>
      api.setNotificationCategoryEnabled(pref.category, enabled),
    onSuccess: refresh,
    onError: (e) =>
      setError(e instanceof ApiError ? e.message : "Failed to update"),
  });

  const mute = useMutation({
    mutationFn: (until: Date) =>
      api.muteNotifications(
        isAll ? null : pref.category,
        until.toISOString()
      ),
    onSuccess: () => {
      setMenuAnchor(null);
      refresh();
    },
    onError: (e) =>
      setError(e instanceof ApiError ? e.message : "Failed to mute"),
  });

  const clearMute = useMutation({
    mutationFn: () => api.clearNotificationMute(isAll ? null : pref.category),
    onSuccess: refresh,
    onError: (e) =>
      setError(e instanceof ApiError ? e.message : "Failed to unmute"),
  });

  return (
    <ListItem
      secondaryAction={
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          {muted && pref.mutedUntil && (
            <Chip
              size="small"
              color="warning"
              label={`Muted until ${fmtUntil(pref.mutedUntil)}`}
              onDelete={() => clearMute.mutate()}
            />
          )}
          <Button size="small" onClick={(e) => setMenuAnchor(e.currentTarget)}>
            Mute
          </Button>
          {!isAll && (
            <Switch
              edge="end"
              checked={pref.enabled}
              onChange={(e) => toggle.mutate(e.target.checked)}
              slotProps={{ input: { "aria-label": `Enable ${pref.label}` } }}
            />
          )}
        </Box>
      }
    >
      <ListItemText
        primary={isAll ? `${pref.label} (global mute)` : pref.label}
        secondary={
          <>
            {prefStatusText(isAll, pref.enabled)}
            {error && (
              <Typography component="span" color="error" sx={{ display: "block" }}>
                {error}
              </Typography>
            )}
          </>
        }
      />
      <Menu
        anchorEl={menuAnchor}
        open={menuAnchor !== null}
        onClose={() => setMenuAnchor(null)}
      >
        {MUTE_OPTIONS.map((o) => (
          <MenuItem
            key={o.label}
            onClick={() => mute.mutate(new Date(Date.now() + o.ms))}
          >
            Mute for {o.label}
          </MenuItem>
        ))}
      </Menu>
    </ListItem>
  );
}

export default function NotificationsPage() {
  const prefsQuery = useQuery({
    queryKey: ["notification-prefs"],
    queryFn: () => api.getMyNotificationPreferences(),
  });

  if (prefsQuery.isLoading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", mt: 8 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (prefsQuery.isError) {
    return <Alert severity="error">Failed to load notification settings.</Alert>;
  }

  const prefs = prefsQuery.data ?? [];

  return (
    <Paper sx={{ maxWidth: 640, p: 3 }}>
      <Typography variant="h5" gutterBottom>
        Notification settings
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Choose which notifications reach your feed. Muting pauses a category
        for a while without turning it off; the global mute pauses everything.
      </Typography>
      <List>
        {prefs.map((p) => (
          <PrefRow key={p.category} pref={p} />
        ))}
      </List>
    </Paper>
  );
}
