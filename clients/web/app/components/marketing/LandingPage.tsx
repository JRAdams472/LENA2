"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import Image from "next/image";
import { useRouter } from "next/navigation";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import CalendarMonthIcon from "@mui/icons-material/CalendarMonth";
import ShoppingCartOutlinedIcon from "@mui/icons-material/ShoppingCartOutlined";
import KitchenIcon from "@mui/icons-material/Kitchen";
import WineBarIcon from "@mui/icons-material/WineBar";
import AutoAwesomeIcon from "@mui/icons-material/AutoAwesome";
import EventIcon from "@mui/icons-material/Event";
import { useAuth } from "@/app/auth/AuthProvider";
import MarketingShell from "@/app/components/marketing/MarketingShell";
import LoginMenu from "@/app/components/marketing/LoginMenu";

const FEATURES = [
  {
    icon: <CalendarMonthIcon />,
    title: "Meal planning",
    body: "A week grid for breakfasts, lunches, and dinners, with household favorites a tap away.",
    image: "/images/meal-plan-week.png",
    href: "/docs/meal-planning",
  },
  {
    icon: <ShoppingCartOutlinedIcon />,
    title: "Grocery lists",
    body: "Lists build themselves from your plans and group items by your store's aisles.",
    image: "/images/grocery-list.png",
    href: "/docs/grocery-lists",
  },
  {
    icon: <KitchenIcon />,
    title: "Pantry & barcode scanning",
    body: "Know what you have. Scan items in from the pantry shelf, track expirations before food goes to waste.",
    image: "/images/mobile-scan.png",
    href: "/docs/mobile-scanning",
  },
  {
    icon: <WineBarIcon />,
    title: "Wine cellar",
    body: "Bottles, vintages, regions, and what pairs with tonight's dinner.",
    image: "/images/wine-bottles.png",
    href: "/docs/wine-cellar",
  },
  {
    icon: <EventIcon />,
    title: "Food events",
    body: "Plan the holiday dinner or the cookout with timelines that keep every dish on schedule.",
    image: "/images/event-timeline.png",
    href: "/docs/food-events",
  },
  {
    icon: <AutoAwesomeIcon />,
    title: "Ask Dot",
    body: "An assistant that knows your pantry and recipes. Ask what to cook tonight; get real answers from your own kitchen.",
    image: "/images/assistant.png",
    href: "/docs/ai-assistant",
  },
];

const WEEK_STEPS = [
  {
    title: "Plan the week",
    body: "Drop recipes onto the week grid, or let Dot suggest meals around what is already in the pantry.",
  },
  {
    title: "Shop once",
    body: "The grocery list builds itself from the plan, grouped by aisle, minus what you already stock.",
  },
  {
    title: "Cook from the phone",
    body: "Cook mode keeps the recipe at arm's reach on the counter: big steps, no smudged screens.",
  },
  {
    title: "Share the household",
    body: "Everyone sees the same pantry, plans, and lists. Allergy warnings follow each member.",
  },
];

function Shot({
  src,
  alt,
  tall = false,
}: {
  src: string;
  alt: string;
  tall?: boolean;
}) {
  return (
    <Paper
      variant="outlined"
      sx={{ overflow: "hidden", lineHeight: 0, width: "100%" }}
    >
      <Image
        src={src}
        alt={alt}
        width={1120}
        height={tall ? 900 : 700}
        style={{ width: "100%", height: "auto" }}
      />
    </Paper>
  );
}

export default function LandingPage() {
  const { isAuthenticated, isRestoring } = useAuth();
  const router = useRouter();
  // Members go straight to the app. The rAF gate holds render for one
  // frame so a stored token (sessionStorage / legacy migration) hydrates
  // before the marketing page can flash for a signed-in visitor.
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    const id = requestAnimationFrame(() => setHydrated(true));
    return () => cancelAnimationFrame(id);
  }, []);

  useEffect(() => {
    if (hydrated && !isRestoring && isAuthenticated) {
      router.replace("/member");
    }
  }, [hydrated, isAuthenticated, isRestoring, router]);

  if (!hydrated || isRestoring || isAuthenticated) {
    return <Box sx={{ minHeight: "100vh", bgcolor: "background.default" }} />;
  }

  return (
    <MarketingShell>
      {/* Hero — split: message left, real product shot right */}
      <Box
        sx={{
          maxWidth: 1120,
          mx: "auto",
          px: { xs: 2, md: 4 },
          pt: { xs: 5, md: 8 },
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "5fr 6fr" },
          gap: { xs: 4, md: 6 },
          alignItems: "center",
        }}
      >
        <Box>
          <Typography
            component="h1"
            variant="h3"
            sx={{ fontWeight: 700, lineHeight: 1.15 }}
          >
            The household kitchen, kept.
          </Typography>
          <Typography
            variant="h6"
            color="text.secondary"
            sx={{ mt: 2, fontWeight: 400, maxWidth: "44ch" }}
          >
            Recipes, pantry, meal plans, grocery lists, and the wine cellar in
            one place your whole household shares.
          </Typography>
          <Box sx={{ mt: 4, display: "flex", gap: 2, alignItems: "center" }}>
            <LoginMenu />
            <Button
              component={Link}
              href="/docs"
              color="inherit"
              sx={{ textTransform: "none", color: "text.secondary" }}
            >
              See how it works
            </Button>
          </Box>
        </Box>
        <Shot src="/images/dashboard.png" alt="The LENA dashboard: today's meals, pantry suggestions, and household activity" tall />
      </Box>

      {/* Feature sections — alternating text/shot pairs, capped at two */}
      <Box sx={{ maxWidth: 1120, mx: "auto", px: { xs: 2, md: 4 }, mt: { xs: 8, md: 12 } }}>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
            gap: { xs: 4, md: 6 },
            alignItems: "center",
          }}
        >
          <Box>
            <Typography variant="h4" sx={{ fontWeight: 700 }} gutterBottom>
              Plan the week, then let the week run itself
            </Typography>
            <Typography variant="body1" color="text.secondary" sx={{ maxWidth: "52ch" }}>
              Drag recipes onto the meal grid, get servings and allergen
              warnings inline, and turn the plan into a grocery list that
              already knows what is in the pantry.
            </Typography>
            <Button
              component={Link}
              href="/docs/meal-planning"
              sx={{ mt: 2, textTransform: "none" }}
            >
              Meal planning
            </Button>
          </Box>
          <Shot src="/images/meal-plan-week.png" alt="Seven-day meal plan grid with breakfast, lunch, and dinner rows" />
        </Box>

        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
            gap: { xs: 4, md: 6 },
            alignItems: "center",
            mt: { xs: 8, md: 10 },
          }}
        >
          <Shot src="/images/grocery-list.png" alt="A grocery list grouped by store aisles with checked items" />
          <Box sx={{ order: { xs: -1, md: 0 } }}>
            <Typography variant="h4" sx={{ fontWeight: 700 }} gutterBottom>
              A grocery list that shops the way you walk
            </Typography>
            <Typography variant="body1" color="text.secondary" sx={{ maxWidth: "52ch" }}>
              Items group by your store&apos;s aisles, check off as you go, and
              restock the pantry on the way home. Split trips between
              household members and the list stays in sync.
            </Typography>
            <Button
              component={Link}
              href="/docs/grocery-lists"
              sx={{ mt: 2, textTransform: "none" }}
            >
              Grocery lists
            </Button>
          </Box>
        </Box>
      </Box>

      {/* More of it — compact feature cards */}
      <Box sx={{ maxWidth: 1120, mx: "auto", px: { xs: 2, md: 4 }, mt: { xs: 8, md: 12 } }}>
        <Typography variant="h4" sx={{ fontWeight: 700 }} gutterBottom>
          The rest of the kitchen
        </Typography>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
            gap: 3,
            mt: 3,
          }}
        >
          {FEATURES.slice(2).map((f) => (
            <Paper
              key={f.title}
              component={Link}
              href={f.href}
              variant="outlined"
              sx={{
                p: 3,
                display: "flex",
                flexDirection: "column",
                gap: 1.5,
                textDecoration: "none",
                color: "inherit",
                "&:hover": { borderColor: "primary.main" },
              }}
            >
              <Box sx={{ color: "primary.dark" }}>{f.icon}</Box>
              <Typography variant="h6" sx={{ fontWeight: 600 }}>
                {f.title}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {f.body}
              </Typography>
            </Paper>
          ))}
        </Box>
      </Box>

      {/* A typical week */}
      <Box sx={{ maxWidth: 1120, mx: "auto", px: { xs: 2, md: 4 }, mt: { xs: 8, md: 12 } }}>
        <Typography variant="h4" sx={{ fontWeight: 700 }} gutterBottom>
          A typical week
        </Typography>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr", lg: "repeat(4, 1fr)" },
            gap: 3,
            mt: 3,
          }}
        >
          {WEEK_STEPS.map((step, i) => (
            <Box key={step.title} sx={{ display: "flex", gap: 2 }}>
              <Box
                aria-hidden
                sx={{
                  flexShrink: 0,
                  width: 32,
                  height: 32,
                  borderRadius: "50%",
                  bgcolor: "primary.main",
                  color: "primary.contrastText",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontWeight: 700,
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
      </Box>

      {/* Privacy strip */}
      <Box sx={{ maxWidth: 1120, mx: "auto", px: { xs: 2, md: 4 }, mt: { xs: 8, md: 12 } }}>
        <Paper
          variant="outlined"
          sx={{ p: { xs: 3, md: 4 }, bgcolor: "background.paper" }}
        >
          <Typography variant="h5" sx={{ fontWeight: 700 }} gutterBottom>
            Your kitchen&apos;s data stays yours
          </Typography>
          <Typography variant="body1" color="text.secondary" sx={{ maxWidth: "65ch" }}>
            Dot can run against a local model on your own hardware, so pantry
            contents and dietary notes never have to leave the house. The
            cloud model is opt-in, and you choose per feature.
          </Typography>
          <Button
            component={Link}
            href="/docs/privacy"
            sx={{ mt: 2, textTransform: "none" }}
          >
            Privacy & local AI
          </Button>
        </Paper>
      </Box>

      {/* CTA band */}
      <Box sx={{ maxWidth: 1120, mx: "auto", px: { xs: 2, md: 4 }, mt: { xs: 8, md: 12 } }}>
        <Paper
          sx={{
            p: { xs: 4, md: 6 },
            bgcolor: "primary.main",
            color: "primary.contrastText",
            textAlign: "center",
          }}
        >
          <Typography variant="h4" sx={{ fontWeight: 700 }}>
            Dinner tonight starts with what you already have
          </Typography>
          <Typography
            variant="body1"
            sx={{ mt: 1.5, opacity: 0.9, maxWidth: "52ch", mx: "auto" }}
          >
            Sign in to open your household&apos;s pantry, plans, and lists.
          </Typography>
          <Box sx={{ mt: 3, display: "inline-flex" }}>
            <LoginMenu />
          </Box>
        </Paper>
      </Box>
    </MarketingShell>
  );
}
