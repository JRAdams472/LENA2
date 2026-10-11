"use client";

import { ReactNode, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import AppBar from "@mui/material/AppBar";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Divider from "@mui/material/Divider";
import Drawer from "@mui/material/Drawer";
import IconButton from "@mui/material/IconButton";
import List from "@mui/material/List";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemText from "@mui/material/ListItemText";
import ListSubheader from "@mui/material/ListSubheader";
import Toolbar from "@mui/material/Toolbar";
import Typography from "@mui/material/Typography";
import MenuIcon from "@mui/icons-material/Menu";
import { useTheme } from "@mui/material/styles";
import { paletteFor } from "@/lib/themeVars";
import LenaLogo from "@/app/components/LenaLogo";
import ThemeModeButton from "@/app/components/ThemeModeButton";
import LoginMenu from "@/app/components/marketing/LoginMenu";
import { DocsNavList } from "@/app/components/marketing/DocsNav";

const NAV = [
  { label: "Docs", href: "/docs" },
  { label: "GitHub", href: "https://github.com/JRAdams472/LENA2", external: true },
];

const APPBAR_HEIGHT = 64;

export default function MarketingShell({ children }: { children: ReactNode }) {
  const theme = useTheme();
  const pathname = usePathname() ?? "";
  const [navOpen, setNavOpen] = useState(false);
  // The docs rail is hidden on phones — on /docs routes the drawer carries
  // the docs tree so every page stays reachable, with site links below.
  const onDocs = pathname.startsWith("/docs");

  return (
    <Box sx={{ display: "flex", flexDirection: "column", minHeight: "100vh" }}>
      {/* Signature sage band, same as the member app header */}
      <AppBar position="sticky" elevation={0}>
        <Toolbar sx={{ minHeight: APPBAR_HEIGHT, gap: 2 }}>
          <IconButton
            color="inherit"
            edge="start"
            aria-label="Open navigation"
            onClick={() => setNavOpen(true)}
            sx={{ display: { md: "none" }, mr: -1 }}
          >
            <MenuIcon />
          </IconButton>
          <Link href="/" aria-label="LENA home" style={{ display: "flex" }}>
            <LenaLogo
              size={26}
              iconColor={paletteFor(theme).primary.contrastText}
              textColor={paletteFor(theme).primary.contrastText}
            />
          </Link>
          <Box sx={{ flexGrow: 1 }} />
          <Box sx={{ display: { xs: "none", md: "flex" }, alignItems: "center", gap: 1 }}>
            {NAV.map((item) => (
              <Button
                key={item.label}
                component={Link}
                href={item.href}
                color="inherit"
                sx={{ textTransform: "none", fontWeight: 600 }}
                {...(item.external
                  ? { target: "_blank", rel: "noopener noreferrer" }
                  : {})}
              >
                {item.label}
              </Button>
            ))}
          </Box>
          <ThemeModeButton />
          <LoginMenu />
        </Toolbar>
      </AppBar>

      <Drawer
        open={navOpen}
        onClose={() => setNavOpen(false)}
        sx={{ display: { md: "none" } }}
      >
        <Box
          sx={{ width: 260, p: 1 }}
          role="navigation"
          aria-label={onDocs ? "Documentation" : "Site"}
        >
          {onDocs && <DocsNavList onNavigate={() => setNavOpen(false)} />}
          {onDocs && <Divider sx={{ my: 1 }} />}
          <List
            dense
            subheader={
              onDocs ? (
                <ListSubheader
                  sx={{ bgcolor: "transparent", fontWeight: 700, lineHeight: "40px" }}
                >
                  Site
                </ListSubheader>
              ) : undefined
            }
          >
            {NAV.map((item) => (
              <ListItemButton
                key={item.label}
                component={Link}
                href={item.href}
                onClick={() => setNavOpen(false)}
                sx={{ borderRadius: 2.5 }}
                {...(item.external
                  ? { target: "_blank", rel: "noopener noreferrer" }
                  : {})}
              >
                <ListItemText primary={item.label} />
              </ListItemButton>
            ))}
          </List>
        </Box>
      </Drawer>

      <Box component="main" sx={{ flexGrow: 1 }}>
        {children}
      </Box>

      <Box
        component="footer"
        sx={{
          borderTop: 1,
          borderColor: "divider",
          bgcolor: "background.paper",
          mt: 8,
        }}
      >
        <Box
          sx={{
            maxWidth: 1120,
            mx: "auto",
            px: { xs: 2, md: 4 },
            py: 4,
            display: "flex",
            flexWrap: "wrap",
            alignItems: "center",
            gap: 2,
          }}
        >
          <LenaLogo size={20} />
          <Box sx={{ flexGrow: 1 }} />
          {NAV.map((item) => (
            <Typography
              key={item.label}
              component={Link}
              href={item.href}
              variant="body2"
              color="text.secondary"
              sx={{ textDecoration: "none", "&:hover": { color: "primary.main" } }}
              {...(item.external
                ? { target: "_blank", rel: "noopener noreferrer" }
                : {})}
            >
              {item.label}
            </Typography>
          ))}
          <Typography variant="body2" color="text.secondary">
            LENA keeps the household kitchen in one place.
          </Typography>
        </Box>
      </Box>
    </Box>
  );
}
