"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import Box from "@mui/material/Box";
import List from "@mui/material/List";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemText from "@mui/material/ListItemText";
import ListSubheader from "@mui/material/ListSubheader";
import { DOCS_NAV } from "@/app/components/marketing/docsModel";

// Left rail for /docs — same visual treatment as the member drawer:
// paper surface, 10px item radius, sage-tinted active state.
export default function DocsNav() {
  const pathname = usePathname() ?? "";

  return (
    <Box
      component="nav"
      aria-label="Documentation"
      sx={{
        width: 260,
        flexShrink: 0,
        borderRight: 1,
        borderColor: "divider",
        bgcolor: "background.paper",
        display: { xs: "none", md: "block" },
        overflowY: "auto",
        position: "sticky",
        top: 64,
        alignSelf: "flex-start",
        height: "calc(100vh - 64px)",
      }}
    >
      {DOCS_NAV.map((group) => (
        <List
          key={group.label}
          dense
          subheader={
            <ListSubheader
              sx={{ bgcolor: "transparent", fontWeight: 700, lineHeight: "40px" }}
            >
              {group.label}
            </ListSubheader>
          }
        >
          {group.items.map((item) => {
            const selected = pathname === item.href;
            // Pages render muted and non-navigating until their content
            // lands — the rail never offers a dead link.
            if (!item.live) {
              return (
                <ListItemButton
                  key={item.href}
                  disabled
                  sx={{ borderRadius: 2.5, mx: 1.5, px: 2, py: 1 }}
                >
                  <ListItemText
                    primary={item.label}
                    sx={{ "& .MuiTypography-root": { color: "text.disabled" } }}
                  />
                </ListItemButton>
              );
            }
            return (
              <ListItemButton
                key={item.href}
                component={Link}
                href={item.href}
                selected={selected}
                aria-current={selected ? "page" : undefined}
                sx={{ borderRadius: 2.5, mx: 1.5, px: 2, py: 1 }}
              >
                <ListItemText primary={item.label} />
              </ListItemButton>
            );
          })}
        </List>
      ))}
    </Box>
  );
}
