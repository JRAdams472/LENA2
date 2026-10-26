"use client";

import { ReactNode } from "react";
import Image from "next/image";
import Link from "next/link";
import Box from "@mui/material/Box";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";

// Shared building blocks for the public docs pages. They keep every page
// in the member app's visual language — paper surfaces, hairline borders,
// sage accents, 10px radii — without a markdown pipeline.

export function DocPage({
  title,
  intro,
  children,
}: {
  title: string;
  intro: string;
  children: ReactNode;
}) {
  return (
    <Box sx={{ maxWidth: 760 }}>
      <Typography component="h1" variant="h3" sx={{ fontWeight: 700 }} gutterBottom>
        {title}
      </Typography>
      <Typography
        variant="body1"
        color="text.secondary"
        sx={{ maxWidth: "65ch", mb: 4 }}
      >
        {intro}
      </Typography>
      {children}
    </Box>
  );
}

export function Section({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <Box component="section" sx={{ mt: 5 }}>
      <Typography component="h2" variant="h5" sx={{ fontWeight: 600 }} gutterBottom>
        {title}
      </Typography>
      <Box sx={{ maxWidth: "65ch" }}>{children}</Box>
    </Box>
  );
}

export function Screenshot({
  src,
  alt,
  caption,
  device = "web",
}: {
  src: string;
  alt: string;
  caption?: string;
  device?: "web" | "phone" | "tablet";
}) {
  return (
    <Box sx={{ my: 3 }}>
      <Paper
        variant="outlined"
        sx={{
          overflow: "hidden",
          display: "inline-block",
          lineHeight: 0,
          mx: device === "web" ? 0 : "auto",
        }}
      >
        <Image
          src={src}
          alt={alt}
          width={device === "web" ? 1120 : device === "tablet" ? 1024 : 320}
          height={device === "web" ? 700 : 640}
          style={{ maxWidth: "100%", height: "auto" }}
        />
      </Paper>
      {caption && (
        <Typography
          variant="caption"
          color="text.secondary"
          sx={{ display: "block", mt: 1 }}
        >
          {caption}
        </Typography>
      )}
    </Box>
  );
}

export function StepList({ steps }: { steps: { title: string; body: string }[] }) {
  return (
    <Box component="ol" sx={{ listStyle: "none", p: 0, m: 0, mt: 2 }}>
      {steps.map((step, i) => (
        <Box
          component="li"
          key={step.title}
          sx={{ display: "flex", gap: 2, mb: 2.5 }}
        >
          <Box
            aria-hidden
            sx={{
              flexShrink: 0,
              width: 28,
              height: 28,
              borderRadius: "50%",
              bgcolor: "primary.main",
              color: "primary.contrastText",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              fontWeight: 700,
              fontSize: 14,
            }}
          >
            {i + 1}
          </Box>
          <Box>
            <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
              {step.title}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {step.body}
            </Typography>
          </Box>
        </Box>
      ))}
    </Box>
  );
}

export function Callout({ children }: { children: ReactNode }) {
  return (
    <Paper
      variant="outlined"
      sx={{ p: 2.5, my: 3, bgcolor: "background.default" }}
    >
      <Typography variant="body2" component="div">
        {children}
      </Typography>
    </Paper>
  );
}

export function P({ children }: { children: ReactNode }) {
  return (
    <Typography variant="body1" color="text.secondary" component="div" sx={{ mb: 1.5 }}>
      {children}
    </Typography>
  );
}

export function DocSub({ children }: { children: ReactNode }) {
  return (
    <Typography component="h3" variant="h6" sx={{ fontWeight: 600, mt: 3, mb: 1 }}>
      {children}
    </Typography>
  );
}

export function DocList({
  ordered = false,
  items,
}: {
  ordered?: boolean;
  items: ReactNode[];
}) {
  return (
    <Box
      component={ordered ? "ol" : "ul"}
      sx={{
        m: 0,
        mt: 1.5,
        pl: 3,
        "& li": { mb: 0.75 },
      }}
    >
      {items.map((item, i) => (
        // eslint-disable-next-line @eslint-react/no-array-index-key -- static docs content
        <Typography key={i} component="li" variant="body1" color="text.secondary">
          {item}
        </Typography>
      ))}
    </Box>
  );
}

export function DocTable({ head, rows }: { head: string[]; rows: ReactNode[][] }) {
  return (
    <Paper
      variant="outlined"
      component="table"
      sx={{
        display: "block",
        overflowX: "auto",
        borderCollapse: "collapse",
        my: 2,
        "& th, & td": {
          px: 2,
          py: 1.25,
          textAlign: "left",
          borderBottom: "1px solid",
          borderColor: "divider",
        },
        "& th": { fontWeight: 700 },
        "& tr:last-child td": { borderBottom: 0 },
      }}
    >
      <thead>
        <tr>
          {head.map((h) => (
            <Typography key={h} component="th" variant="body2">
              {h}
            </Typography>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, i) => (
          // eslint-disable-next-line @eslint-react/no-array-index-key -- static docs content
          <tr key={i}>
            {row.map((cell, j) => (
              // eslint-disable-next-line @eslint-react/no-array-index-key -- static docs content
              <Typography key={j} component="td" variant="body2" color="text.secondary">
                {cell}
              </Typography>
            ))}
          </tr>
        ))}
      </tbody>
    </Paper>
  );
}

export function DocLink({
  href,
  external = false,
  children,
}: {
  href: string;
  external?: boolean;
  children: ReactNode;
}) {
  const sx = {
    fontWeight: 600,
    textDecoration: "none",
    "&:hover": { textDecoration: "underline" },
  } as const;
  if (external) {
    return (
      <Typography
        component="a"
        href={href}
        target="_blank"
        rel="noreferrer"
        color="primary.main"
        sx={sx}
      >
        {children}
      </Typography>
    );
  }
  return (
    <Typography component={Link} href={href} color="primary.main" sx={sx}>
      {children}
    </Typography>
  );
}
