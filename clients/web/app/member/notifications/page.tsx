"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import CircularProgress from "@mui/material/CircularProgress";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemText from "@mui/material/ListItemText";
import Menu from "@mui/material/Menu";
import MenuItem from "@mui/material/MenuItem";
import Paper from "@mui/material/Paper";
import Switch from "@mui/material/Switch";
import Typography from "@mui/material/Typography";
import { api, ApiError } from "@/lib/api";
import {
  notificationHref,
  notificationText,
  timeAgo,
  unreadNotificationIds,
} from "@/lib/notifications";
import {
  HouseholdNotification,
  NotificationCategoryPreference,
} from "@/lib/types";

// Mute presets offered by the per-category menu, in milliseconds.
const MUTE_OPTIONS = [
  { label: "1 hour", ms: 60 * 60 * 1000 },
  { label: "8 hours", ms: 8 * 60 * 60 * 1000 },
  { label: "1 day", ms: 24 * 60 * 60 * 1000 },
  { label: "1 week", ms: 7 * 24 * 60 * 60 * 1000 },
];

function prefStatusText(
  isAll: boolean,
  enabled: boolean,
  pushEnabled: boolean
): string {
  if (isAll) {
    return pushEnabled
      ? "Pushes every category — and pauses everything while muted."
      : "Pause every notification for a while — nothing is delivered until it expires.";
  }
  const feed = enabled ? "Delivered" : "Turned off";
  return pushEnabled ? `${feed} · pushed` : feed;
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

  const pushToggle = useMutation({
    mutationFn: (enabled: boolean) =>
      api.setNotificationCategoryPushEnabled(pref.category, enabled),
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
          <Button
            size="small"
            aria-haspopup="menu"
            title="Pause notifications for a while"
            onClick={(e) => setMenuAnchor(e.currentTarget)}
          >
            Mute
          </Button>
          <Box sx={{ textAlign: "center" }}>
            <Switch
              checked={pref.pushEnabled}
              onChange={(e) => pushToggle.mutate(e.target.checked)}
              title="Push this category to your devices"
              slotProps={{
                input: { "aria-label": `Push ${pref.label}` },
              }}
            />
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{ display: "block" }}
            >
              Push
            </Typography>
          </Box>
          {isAll ? (
            // _all has no feed toggle — global muting is time-bounded.
            // Spacer keeps the Push column aligned across rows.
            <Box sx={{ width: 58 }} />
          ) : (
            <Box sx={{ textAlign: "center" }}>
              <Switch
                edge="end"
                checked={pref.enabled}
                onChange={(e) => toggle.mutate(e.target.checked)}
                title="Turn feed delivery on or off"
                slotProps={{ input: { "aria-label": `Enable ${pref.label}` } }}
              />
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ display: "block" }}
              >
                Feed
              </Typography>
            </Box>
          )}
        </Box>
      }
    >
      <ListItemText
        primary={isAll ? `${pref.label} (global mute)` : pref.label}
        secondary={
          <>
            {prefStatusText(isAll, pref.enabled, pref.pushEnabled)}
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

// Notification history — the bell dropdown only shows the recent slice;
// this feed lists more of it with per-item read state. Clicking a row
// marks just that notification read and follows its deep-link.
function NotificationFeed() {
  const router = useRouter();
  const queryClient = useQueryClient();
  // Locally-marked reads until the API returns per-item readAt.
  const [readIds, setReadIds] = useState<ReadonlySet<number>>(
    () => new Set()
  );

  const feedQuery = useQuery({
    queryKey: ["myNotifications"],
    queryFn: () => api.getMyNotifications(50),
  });
  const unreadQuery = useQuery({
    queryKey: ["unreadNotificationCount"],
    queryFn: () => api.getUnreadNotificationCount(),
  });

  const items = feedQuery.data ?? [];
  // Count unknown → every unmarked row gets the dot rather than none.
  const unreadSet = unreadNotificationIds(
    items,
    unreadQuery.data ?? items.length,
    readIds
  );
  const isUnread = (n: HouseholdNotification) =>
    unreadSet.has(n.notificationID);

  const refreshUnread = () =>
    queryClient.invalidateQueries({ queryKey: ["unreadNotificationCount"] });

  const markRead = useMutation({
    mutationFn: (id: number) => api.markNotificationRead(id),
    onSuccess: refreshUnread,
  });

  const markAll = useMutation({
    mutationFn: () => api.markAllNotificationsRead(),
    onSuccess: () => {
      setReadIds((prev) => {
        const next = new Set(prev);
        (feedQuery.data ?? []).forEach((n) => next.add(n.notificationID));
        return next;
      });
      refreshUnread();
      queryClient.invalidateQueries({ queryKey: ["myNotifications"] });
    },
  });

  const open = (n: HouseholdNotification) => {
    if (isUnread(n)) {
      setReadIds((prev) => new Set(prev).add(n.notificationID));
      markRead.mutate(n.notificationID);
    }
    router.push(notificationHref(n));
  };

  const hasUnread = unreadSet.size > 0;

  return (
    <Paper sx={{ maxWidth: 640, p: 3, width: "100%" }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          mb: 1,
        }}
      >
        <Typography variant="h5">Notifications</Typography>
        <Button
          size="small"
          onClick={() => markAll.mutate()}
          disabled={!hasUnread || markAll.isPending}
        >
          Mark all read
        </Button>
      </Box>
      {feedQuery.isLoading && (
        <Box sx={{ display: "flex", justifyContent: "center", p: 3 }}>
          <CircularProgress size={24} />
        </Box>
      )}
      {feedQuery.isError && (
        <Alert severity="error">Failed to load notifications.</Alert>
      )}
      {feedQuery.isSuccess && items.length === 0 && (
        <Typography variant="body2" color="text.secondary">
          No notifications yet — household activity, reminders, and expiring
          items will show up here.
        </Typography>
      )}
      <List dense>
        {items.map((n) => (
          <ListItem key={n.notificationID} disablePadding>
            <ListItemButton onClick={() => open(n)}>
              <ListItemText
                primary={
                  <Box
                    component="span"
                    sx={{ display: "flex", alignItems: "center", gap: 1 }}
                  >
                    {isUnread(n) && (
                      <Box
                        component="span"
                        data-testid={`unread-dot-${n.notificationID}`}
                        sx={{
                          width: 8,
                          height: 8,
                          borderRadius: "50%",
                          bgcolor: "primary.main",
                          flexShrink: 0,
                        }}
                      />
                    )}
                    {notificationText(n)}
                  </Box>
                }
                secondary={
                  <>
                    {n.body}
                    {n.body ? <br /> : null}
                    {timeAgo(n.createdAt)}
                  </>
                }
              />
            </ListItemButton>
          </ListItem>
        ))}
      </List>
    </Paper>
  );
}

export default function NotificationsPage() {
  const prefsQuery = useQuery({
    queryKey: ["notification-prefs"],
    queryFn: () => api.getMyNotificationPreferences(),
  });

  const prefs = prefsQuery.data ?? [];

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 3,
        alignItems: "center",
      }}
    >
      <NotificationFeed />
      <Paper sx={{ maxWidth: 640, p: 3, width: "100%" }}>
        <Typography variant="h5" gutterBottom>
          Notification settings
        </Typography>
      {prefsQuery.isLoading && (
        <Box sx={{ display: "flex", justifyContent: "center", p: 3 }}>
          <CircularProgress size={24} />
        </Box>
      )}
      {prefsQuery.isError && (
        <Alert severity="error">Failed to load notification settings.</Alert>
      )}
      {prefsQuery.isSuccess && (
        <>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Choose which notifications reach your feed — and which get pushed to
        your devices. Muting pauses a category for a while without turning it
        off; the global mute pauses everything.
      </Typography>
      <List>
        {prefs.map((p) => (
          <PrefRow key={p.category} pref={p} />
        ))}
      </List>
        </>
      )}
      </Paper>
    </Box>
  );
}
