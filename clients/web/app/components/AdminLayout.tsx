"use client";

import * as React from "react";
import { styled, useTheme } from "@mui/material/styles";
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
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import Button from "@mui/material/Button";
import { api } from "@/lib/api";
import { HouseholdNotification } from "@/lib/types";
import { useAuth } from "@/app/auth/AuthProvider";
import { useMe } from "@/app/auth/useMe";
import LoginScreen from "@/app/components/LoginScreen";

const DRAWER_WIDTH = 260;

const Logo = styled("div")(({ theme }) => ({
  width: 36,
  height: 36,
  borderRadius: theme.shape.borderRadius,
  backgroundColor: theme.palette.primary.contrastText,
  color: theme.palette.primary.main,
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  fontWeight: 700,
  marginRight: theme.spacing(2),
}));

const Main = styled("main")(({ theme }) => ({
  flexGrow: 1,
  padding: theme.spacing(3),
  paddingTop: theme.spacing(10),
  [theme.breakpoints.up("md")]: {
    marginLeft: DRAWER_WIDTH,
    width: `calc(100% - ${DRAWER_WIDTH}px)`,
  },
}));

interface NavItem {
  label: string;
  href: string;
  adminOnly?: boolean;
  children?: NavItem[];
}

const NAVIGATION: { label: string; href?: string; adminOnly?: boolean; children?: NavItem[] }[] = [
  { label: "Dashboard", href: "/" },
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
  {
    label: "Recipes",
    children: [
      { label: "Recipes", href: "/recipes" },
      { label: "Pending Reviews", href: "/recipes/pending", adminOnly: true },
    ],
  },
  {
    label: "Meal Planning",
    children: [
      { label: "Weekly Plan", href: "/meal-plans" },
      { label: "Grocery Lists", href: "/grocery-lists" },
    ],
  },
  { label: "Users", href: "/users", adminOnly: true },
  { label: "Pending Items", href: "/items/pending", adminOnly: true },
  { label: "Household", href: "/household" },
  { label: "Profile", href: "/profile" },
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
              router.push("/household");
            }}
          >
            <ListItemText
              primary={notificationText(n)}
              secondary={timeAgo(n.createdAt)}
            />
          </MenuItem>
        ))}
      </Menu>
    </>
  );
}

function notificationText(n: HouseholdNotification): string {
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
      NAVIGATION.forEach((group) => {
        if (group.children) {
          initial[group.label] = isGroupActive(pathname, group.children);
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

  const drawerContent = (
    <Box sx={{ overflow: "auto" }}>
      <Box
        sx={{
          height: 64,
          display: { xs: "flex", md: "none" },
          alignItems: "center",
          justifyContent: "center",
          borderBottom: `1px solid ${theme.palette.divider}`,
        }}
      >
        <Typography variant="h6" noWrap>
          LENA
        </Typography>
      </Box>
      <List component="nav" aria-label="main navigation">
        {NAVIGATION.filter((item) => !item.adminOnly || isAdmin).map((group) => {
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
                    <ListItemText primary={group.label} />
                    {openGroups[group.label] ? <ExpandLess /> : <ExpandMore />}
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
                          sx={{ pl: 4 }}
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
        })}
      </List>
      <Divider />
    </Box>
  );

  return (
    <Box sx={{ display: "flex" }}>
      <AppBar
        position="fixed"
        sx={{
          width: { md: `calc(100% - ${DRAWER_WIDTH}px)` },
          ml: { md: `${DRAWER_WIDTH}px` },
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
          <Logo>L</Logo>
          <Typography variant="h6" noWrap component="div" sx={{ flexGrow: 1 }}>
            LENA
          </Typography>
          {user && (
            <>
              <NotificationBell />
              <Typography variant="body2" sx={{ mr: 2 }}>
                {user.email}
              </Typography>
              <Button color="inherit" onClick={signOut}>
                Sign out
              </Button>
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
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              borderBottom: `1px solid ${theme.palette.divider}`,
            }}
          >
            <Typography variant="h6" noWrap>
              LENA
            </Typography>
          </Box>
          {drawerContent}
        </Drawer>
      </Box>

      <Main>{children}</Main>
    </Box>
  );
}
