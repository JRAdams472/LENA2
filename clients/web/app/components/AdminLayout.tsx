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
import Link from "next/link";
import { usePathname } from "next/navigation";
import Button from "@mui/material/Button";
import { useAuth } from "@/app/auth/AuthProvider";
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
  children?: NavItem[];
}

const NAVIGATION: { label: string; href?: string; children?: NavItem[] }[] = [
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
    children: [{ label: "Recipes", href: "/recipes" }],
  },
  {
    label: "Meal Planning",
    children: [
      { label: "Weekly Plan", href: "/meal-plans" },
      { label: "Grocery Lists", href: "/grocery-lists" },
    ],
  },
];

function isActive(pathname: string, href: string): boolean {
  return pathname === href;
}

function isGroupActive(pathname: string, children: NavItem[]): boolean {
  return children.some((child) => isActive(pathname, child.href));
}

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const theme = useTheme();
  const pathname = usePathname() ?? "";
  const { user, signOut } = useAuth();
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
        {NAVIGATION.map((group) => {
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
