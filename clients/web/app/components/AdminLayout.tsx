"use client";

import * as React from "react";
import { alpha, styled, useTheme } from "@mui/material/styles";
import Box from "@mui/material/Box";
import AppBar from "@mui/material/AppBar";
import Toolbar from "@mui/material/Toolbar";
import Typography from "@mui/material/Typography";
import IconButton from "@mui/material/IconButton";
import Drawer from "@mui/material/Drawer";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemText from "@mui/material/ListItemText";
import Collapse from "@mui/material/Collapse";
import Divider from "@mui/material/Divider";
import MenuIcon from "@mui/icons-material/Menu";
import ExpandLess from "@mui/icons-material/ExpandLess";
import ExpandMore from "@mui/icons-material/ExpandMore";
import DashboardIcon from "@mui/icons-material/Dashboard";
import InventoryIcon from "@mui/icons-material/Inventory";
import WineBarIcon from "@mui/icons-material/WineBar";
import MenuBookIcon from "@mui/icons-material/MenuBook";
import RestaurantIcon from "@mui/icons-material/Restaurant";
import ListItemIcon from "@mui/material/ListItemIcon";
import Badge from "@mui/material/Badge";
import Menu from "@mui/material/Menu";
import MenuItem from "@mui/material/MenuItem";
import NotificationsIcon from "@mui/icons-material/Notifications";
import SettingsIcon from "@mui/icons-material/Settings";
import AccountCircleIcon from "@mui/icons-material/AccountCircle";
import LogoutIcon from "@mui/icons-material/Logout";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import Button from "@mui/material/Button";
import { api } from "@/lib/api";
import { HouseholdNotification } from "@/lib/types";
import { useAuth } from "@/app/auth/AuthProvider";
import { useMe } from "@/app/auth/useMe";
import LoginScreen from "@/app/components/LoginScreen";
import LenaLogo from "@/app/components/LenaLogo";

const DRAWER_WIDTH = 260;

// The nav Box below reserves DRAWER_WIDTH in the flex row (the permanent
// drawer's paper is position:fixed), so Main must not offset again —
// marginLeft here would double the gap.
const Main = styled("main")(({ theme }) => ({
  flexGrow: 1,
  minWidth: 0,
  padding: theme.spacing(3),
  paddingTop: theme.spacing(10),
}));

interface NavItem {
  label: string;
  href: string;
  adminOnly?: boolean;
  children?: NavItem[];
}

type NavEntry = { label: string; href?: string; adminOnly?: boolean; children?: NavItem[] };

const CORE_NAV: NavEntry[] = [
  { label: "Dashboard", href: "/" },
  { label: "Assistant", href: "/assistant" },
  {
    label: "Meal Planning",
    children: [
      { label: "Weekly Plan", href: "/meal-plans" },
      { label: "Events", href: "/events" },
      { label: "Grocery Lists", href: "/grocery-lists" },
    ],
  },
  {
    label: "Recipes",
    children: [
      { label: "Recipes", href: "/recipes" },
      { label: "Categories", href: "/recipes/categories", adminOnly: true },
      { label: "Pending Reviews", href: "/recipes/pending", adminOnly: true },
    ],
  },
  {
    label: "Inventory",
    children: [
      { label: "Items", href: "/inventory/items" },
      { label: "Brands", href: "/inventory/brands" },
      { label: "Categories", href: "/inventory/categories" },
      { label: "Food Flavors", href: "/inventory/food-flavors" },
      { label: "Food Nutrients", href: "/inventory/food-nutrients" },
      { label: "Nutrient Types", href: "/inventory/nutrient-types" },
      { label: "Flavor Profiles", href: "/inventory/flavor-profiles" },
    ],
  },
  {
    label: "Wine",
    children: [
      { label: "Bottles", href: "/wine/bottles" },
      { label: "Countries", href: "/wine/countries" },
      { label: "Regions", href: "/wine/regions" },
      { label: "Types", href: "/wine/types" },
      { label: "Vintages", href: "/wine/vintages" },
      { label: "Grape Varieties", href: "/wine/grape-varieties" },
      { label: "Wine Flavor Profiles", href: "/wine/wine-flavor-profiles" },
    ],
  },
];

// Bottom zone: collapsible captioned groups pinned to the drawer foot.
const BOTTOM_NAV: NavEntry[] = [
  {
    label: "Administration",
    adminOnly: true,
    children: [
      { label: "Users", href: "/users", adminOnly: true },
      { label: "Pending Items", href: "/items/pending", adminOnly: true },
    ],
  },
  {
    label: "Account",
    children: [
      { label: "Household", href: "/household" },
      { label: "Profile", href: "/profile" },
    ],
  },
];

function isActive(pathname: string, href: string): boolean {
  return pathname === href;
}

function isGroupActive(pathname: string, children: NavItem[]): boolean {
  return children.some((child) => isActive(pathname, child.href));
}

function timeAgo(iso: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const mins = Math.floor((Date.now() - then) / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

// NotificationBell polls the unread count every 30s while idle. Opening
// the menu fetches the feed, marks everything read, and refreshes the
// feed every 5s until closed — the recipes/pending cadence precedent.
// Deliberately not react-query: AdminLayout must render without
// QueryClientProvider (same constraint as useMe).
function NotificationBell() {
  const router = useRouter();
  const [anchor, setAnchor] = React.useState<HTMLElement | null>(null);
  const [unread, setUnread] = React.useState(0);
  const [items, setItems] = React.useState<HouseholdNotification[]>([]);

  React.useEffect(() => {
    let cancelled = false;
    const load = () =>
      api
        .getUnreadNotificationCount()
        .then((n) => !cancelled && setUnread(n))
        .catch(() => {});
    load();
    const timer = setInterval(load, 30000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, []);

  React.useEffect(() => {
    if (anchor === null) return;
    let cancelled = false;
    const load = () =>
      api
        .getMyNotifications(20)
        .then((ns) => !cancelled && setItems(ns))
        .catch(() => {});
    load();
    // Mark-read-on-open: whatever was unread when the menu opened is
    // cleared; items arriving while the menu is up stay unread until the
    // next open, so they still trigger the badge.
    api
      .markAllNotificationsRead()
      .then(() => !cancelled && setUnread(0))
      .catch(() => {});
    const timer = setInterval(load, 5000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [anchor]);

  return (
    <>
      <IconButton
        color="inherit"
        aria-label="notifications"
        onClick={(e) => setAnchor(e.currentTarget)}
        sx={{ mr: 1 }}
      >
        <Badge badgeContent={unread} color="error" data-testid="notification-badge">
          <NotificationsIcon />
        </Badge>
      </IconButton>
      <Menu
        anchorEl={anchor}
        open={anchor !== null}
        onClose={() => setAnchor(null)}
      >
        {items.length === 0 && <MenuItem disabled>No notifications</MenuItem>}
        {items.map((n) => (
          <MenuItem
            key={n.notificationID}
            onClick={() => {
              setAnchor(null);
              // Reminders deep-link to their subject; event notifications
              // to the event; household activity to the household page.
              router.push(
                n.recipeId != null
                  ? `/recipes/${n.recipeId}`
                  : n.foodEventId != null
                    ? `/events/${n.foodEventId}`
                    : n.itemId != null
                      ? "/inventory/items"
                      : "/household"
              );
            }}
          >
            <ListItemText
              primary={notificationText(n)}
              secondary={
                <>
                  {n.body}
                  {n.body ? <br /> : null}
                  {timeAgo(n.createdAt)}
                </>
              }
            />
            {n.kind === "ITEM_EXPIRING" && n.itemId != null && (
              <Button
                size="small"
                onClick={(e) => {
                  e.stopPropagation();
                  api
                    .addItemToCurrentGroceryList(n.itemId as number)
                    .then(() => {
                      setAnchor(null);
                      router.push("/grocery-lists");
                    })
                    .catch(() => {});
                }}
              >
                Add to list
              </Button>
            )}
          </MenuItem>
        ))}
        <Divider />
        <MenuItem
          onClick={() => {
            setAnchor(null);
            router.push("/notifications");
          }}
        >
          <ListItemIcon>
            <SettingsIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText primary="Notification settings" />
        </MenuItem>
      </Menu>
    </>
  );
}

function notificationText(n: HouseholdNotification): string {
  // Scheduled reminders carry server-rendered text.
  if (n.title) return n.title;
  const actor = n.actor?.displayName ?? "Someone";
  switch (n.kind) {
    case "INVITE_RECEIVED":
      return `${actor} invited you to their household`;
    case "INVITE_ACCEPTED":
      return `${actor} accepted your household invite`;
    case "INVITE_DECLINED":
      return `${actor} declined your household invite`;
    case "INVITE_CANCELLED":
      return `${actor} cancelled a household invite`;
    case "MEMBER_JOINED":
      return `${actor} joined your household`;
    case "MEMBER_LEFT":
      return `${actor} left your household`;
    case "MEMBER_REMOVED":
      return "You were removed from a household";
    case "ROLE_CHANGED":
      return `${actor} changed a household role`;
    case "HOUSEHOLD_RENAMED":
      return `${actor} renamed the household`;
    case "EVENT_CREATED":
      return `${actor} created an event`;
    case "EVENT_UPDATED":
      return `${actor} updated an event`;
    case "EVENT_DELETED":
      return `${actor} deleted an event`;
    case "PROTEIN_DEFROST":
      return "Time to defrost protein for an upcoming meal";
    case "MEAL_PREP_ADVANCE":
      return "A recipe on your meal plan needs advance prep";
    case "ITEM_EXPIRING":
      return "A pantry item is expiring soon";
    default:
      return "Notification";
  }
}

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const theme = useTheme();
  const pathname = usePathname() ?? "";
  const { user, signOut } = useAuth();
  const { isAdmin } = useMe();
  const [mobileOpen, setMobileOpen] = React.useState(false);

  const [openGroups, setOpenGroups] = React.useState<Record<string, boolean>>(
    () => {
      const initial: Record<string, boolean> = {};
      CORE_NAV.forEach((group) => {
        if (group.children) {
          initial[group.label] = isGroupActive(pathname, group.children);
        }
      });
      // Bottom-zone captions start expanded so their links stay visible.
      BOTTOM_NAV.forEach((group) => {
        if (group.children) {
          initial[group.label] = true;
        }
      });
      return initial;
    }
  );

  const toggleGroup = (label: string) => {
    setOpenGroups((prev) => ({ ...prev, [label]: !prev[label] }));
  };

  if (!user) {
    return <LoginScreen />;
  }

  const renderNavEntry = (group: NavEntry, caption = false) => {
    if (group.children) {
      const active = isGroupActive(pathname, group.children);
      const icon =
        group.label === "Inventory" ? (
          <InventoryIcon />
        ) : group.label === "Wine" ? (
          <WineBarIcon />
        ) : group.label === "Recipes" ? (
          <MenuBookIcon />
        ) : group.label === "Meal Planning" ? (
          <RestaurantIcon />
        ) : null;

      return (
        <React.Fragment key={group.label}>
          <ListItem disablePadding>
            <ListItemButton
              onClick={() => toggleGroup(group.label)}
              selected={active}
            >
              {icon && <ListItemIcon>{icon}</ListItemIcon>}
              <ListItemText
                primary={group.label}
                slotProps={
                  caption
                    ? {
                        primary: {
                          variant: "overline",
                          color: "text.secondary",
                          sx: { lineHeight: 1.5 },
                        },
                      }
                    : undefined
                }
              />
              {openGroups[group.label] ? (
                <ExpandLess fontSize={caption ? "small" : "medium"} />
              ) : (
                <ExpandMore fontSize={caption ? "small" : "medium"} />
              )}
            </ListItemButton>
          </ListItem>
          <Collapse
            in={openGroups[group.label]}
            timeout="auto"
            unmountOnExit
          >
            <List component="div" disablePadding>
              {group.children.map((child) => (
                <ListItem key={child.href} disablePadding>
                  <ListItemButton
                    component={Link}
                    href={child.href}
                    selected={isActive(pathname, child.href)}
                    onClick={() => setMobileOpen(false)}
                    sx={{ pl: caption ? 3 : 4 }}
                  >
                    <ListItemText primary={child.label} />
                  </ListItemButton>
                </ListItem>
              ))}
            </List>
          </Collapse>
        </React.Fragment>
      );
    }

    return (
      <ListItem key={group.href!} disablePadding>
        <ListItemButton
          component={Link}
          href={group.href!}
          selected={isActive(pathname, group.href!)}
          onClick={() => setMobileOpen(false)}
        >
          {group.label === "Dashboard" && (
            <ListItemIcon>
              <DashboardIcon />
            </ListItemIcon>
          )}
          <ListItemText primary={group.label} />
        </ListItemButton>
      </ListItem>
    );
  };

  const drawerContent = (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        flex: 1,
        minHeight: 0,
      }}
    >
      <Box
        sx={{
          height: 64,
          flexShrink: 0,
          display: { xs: "flex", md: "none" },
          alignItems: "center",
          justifyContent: "center",
          borderBottom: `1px solid ${theme.palette.divider}`,
        }}
      >
        <LenaLogo size={24} />
      </Box>
      <Box sx={{ flexGrow: 1, overflow: "auto" }}>
        <List component="nav" aria-label="main navigation">
          {CORE_NAV.filter((item) => !item.adminOnly || isAdmin).map((group) =>
            renderNavEntry(group)
          )}
        </List>
      </Box>
      <Divider sx={{ flexShrink: 0 }} />
      <List
        component="nav"
        aria-label="secondary navigation"
        sx={{ flexShrink: 0, py: 1 }}
      >
        {BOTTOM_NAV.filter((item) => !item.adminOnly || isAdmin).map((group) =>
          renderNavEntry(group, true)
        )}
      </List>
    </Box>
  );

  return (
    <Box sx={{ display: "flex" }}>
      <AppBar
        position="fixed"
        sx={{
          width: "100%",
          zIndex: theme.zIndex.drawer + 1,
        }}
      >
        <Toolbar>
          <IconButton
            color="inherit"
            edge="start"
            onClick={() => setMobileOpen(!mobileOpen)}
            sx={{ mr: 2, display: { md: "none" } }}
          >
            <MenuIcon />
          </IconButton>
          <Box sx={{ flexGrow: 1 }}>
            <LenaLogo size={26} iconColor="inherit" textColor="inherit" />
          </Box>
          {user && (
            <>
              <NotificationBell />
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 0.75,
                  borderRadius: 999,
                  px: 1.5,
                  py: 0.5,
                  mr: 1,
                  bgcolor: alpha(theme.palette.primary.main, 0.12),
                }}
              >
                <AccountCircleIcon fontSize="small" />
                <Typography
                  variant="body2"
                  sx={{ display: { xs: "none", sm: "block" } }}
                >
                  {user.email}
                </Typography>
              </Box>
              <IconButton
                color="inherit"
                aria-label="Sign out"
                onClick={signOut}
              >
                <LogoutIcon />
              </IconButton>
            </>
          )}
        </Toolbar>
      </AppBar>

      <Box
        component="nav"
        sx={{ width: { md: DRAWER_WIDTH }, flexShrink: { md: 0 } }}
      >
        <Drawer
          variant="temporary"
          open={mobileOpen}
          onClose={() => setMobileOpen(false)}
          ModalProps={{ keepMounted: true }}
          sx={{
            display: { xs: "block", md: "none" },
            "& .MuiDrawer-paper": {
              boxSizing: "border-box",
              width: DRAWER_WIDTH,
            },
          }}
        >
          {drawerContent}
        </Drawer>
        <Drawer
          variant="permanent"
          open
          sx={{
            display: { xs: "none", md: "block" },
            "& .MuiDrawer-paper": {
              boxSizing: "border-box",
              width: DRAWER_WIDTH,
            },
          }}
        >
          <Box
            sx={{
              height: 64,
              flexShrink: 0,
              borderBottom: `1px solid ${theme.palette.divider}`,
            }}
          />
          {drawerContent}
        </Drawer>
      </Box>

      <Main>{children}</Main>
    </Box>
  );
}
