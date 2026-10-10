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
      <Typography
        variant="body1"
        color="text.secondary"
        component="div"
        sx={{ maxWidth: "65ch" }}
      >
        {children}
      </Typography>
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
          height={device === "web" ? 700 : device === "tablet" ? 640 : 640}
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

export function DocLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <Typography
      component={Link}
      href={href}
      color="primary.main"
      sx={{ fontWeight: 600, textDecoration: "none", "&:hover": { textDecoration: "underline" } }}
    >
      {children}
    </Typography>
  );
}
