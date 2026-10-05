"use client";

import { createElement, useMemo } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { useMe } from "@/app/auth/useMe";
import Typography from "@mui/material/Typography";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import CircularProgress from "@mui/material/CircularProgress";
import Alert from "@mui/material/Alert";
import Paper from "@mui/material/Paper";
import Skeleton from "@mui/material/Skeleton";
import FreeBreakfastIcon from "@mui/icons-material/FreeBreakfast";
import LunchDiningIcon from "@mui/icons-material/LunchDining";
import DinnerDiningIcon from "@mui/icons-material/DinnerDining";
import LocalCafeIcon from "@mui/icons-material/LocalCafe";
import CakeIcon from "@mui/icons-material/Cake";
import SetMealIcon from "@mui/icons-material/SetMeal";
import RestaurantIcon from "@mui/icons-material/Restaurant";
import { alpha, styled, useTheme, Theme } from "@mui/material/styles";
import { Recipe } from "@/lib/types";
import { sizeBadge, stripSize } from "@/lib/format";

const MEAL_TYPES = ["Breakfast", "Lunch", "Dinner"];

const MEAL_ICONS = [FreeBreakfastIcon, LunchDiningIcon, DinnerDiningIcon];

const REASON_LABELS: Record<string, string> = {
  ingredient_overlap: "Similar to your menu",
  rating_recency: "Due for a revisit",
  collaborative_filtering: "Recommended for you",
  category_affinity: "Matches your household's tastes",
  household_trending: "Trending in your household",
};

function reasonLabel(reason: string) {
  return REASON_LABELS[reason] ?? "Recommended for you";
}

// design.md --accent-terracotta / --accent-wheat: the two warm accents that
// don't have a MUI palette slot. Sage/olive come from theme tokens.
const TERRACOTTA = "#C0876B";
const WHEAT = "#D9B26A";

// Deterministic accent per recipe so suggestion tiles differentiate
// without real photos: sage, olive, terracotta, wheat.
function accentFor(theme: Theme, recipeID: number) {
  const accents = [
    theme.palette.primary.main,
    theme.palette.success.main,
    TERRACOTTA,
    WHEAT,
  ];
  return accents[Math.abs(recipeID) % accents.length];
}

// Category/dish-type keywords drive the suggestion tile icon.
function suggestionIcon(recipe: Recipe) {
  const text = (recipe.categories ?? [])
    .map((c) => `${c.group?.groupName ?? ""} ${c.categoryName}`)
    .join(" ")
    .toLowerCase();
  if (/breakfast|brunch/.test(text)) return FreeBreakfastIcon;
  if (/café|cafe|coffee|beverage|drink/.test(text)) return LocalCafeIcon;
  if (/dessert|cake|sweet|baking|pastry|cookie/.test(text)) return CakeIcon;
  if (/seafood|fish/.test(text)) return SetMealIcon;
  if (/dinner|supper|main course/.test(text)) return DinnerDiningIcon;
  if (/lunch|salad|sandwich/.test(text)) return LunchDiningIcon;
  return RestaurantIcon;
}

function suggestionMeta(recipe: Recipe): string {
  const parts: string[] = [];
  const mins = (recipe.prepTimeMinutes ?? 0) + (recipe.cookTimeMinutes ?? 0);
  if (mins > 0) parts.push(`${mins} min`);
  if (recipe.averageRating != null)
    parts.push(`★ ${recipe.averageRating.toFixed(1)}`);
  return parts.join(" · ");
}

const PlanMealLink = styled(Link)(({ theme }) => ({
  display: "inline-flex",
  alignItems: "center",
  fontSize: "0.75rem",
  fontWeight: 500,
  fontStyle: "normal",
  color: theme.palette.text.secondary,
  backgroundColor: theme.palette.background.paper,
  border: `1px dashed ${theme.palette.divider}`,
  borderRadius: 999,
  padding: "5px 12px",
  textDecoration: "none",
  "&:hover": { backgroundColor: alpha(theme.palette.primary.main, 0.08) },
}));

function DashboardSkeleton() {
  return (
    <Box>
      <Skeleton variant="text" width={220} height={40} />
      <Skeleton variant="text" width={320} height={28} sx={{ mb: 2 }} />
      <Paper sx={{ p: 2 }}>
        <Skeleton variant="text" width={140} height={28} />
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "repeat(3, 1fr)" },
            gap: 2,
            mt: 1,
          }}
        >
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} variant="rounded" height={80} />
          ))}
        </Box>
      </Paper>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
          gap: 3,
          mt: 3,
        }}
      >
        <Skeleton variant="rounded" height={220} />
        <Skeleton variant="rounded" height={160} />
      </Box>
    </Box>
  );
}

function isDateInRange(date: Date, weekStartDate: string) {
  const start = new Date(weekStartDate);
  start.setHours(0, 0, 0, 0);
  const end = new Date(start);
  end.setDate(start.getDate() + 7);
  end.setHours(0, 0, 0, 0);
  const d = new Date(date);
  d.setHours(0, 0, 0, 0);
  return d >= start && d < end;
}

export default function Dashboard() {
  const theme = useTheme();
  const today = useMemo(() => {
    const d = new Date();
    d.setHours(0, 0, 0, 0);
    return d;
  }, []);
  const todayDay = today.getDay();

  const plansQuery = useQuery({
    queryKey: ["mealPlans", "dashboard"],
    queryFn: () => api.getMealPlansPaged(1, 1000),
  });

  const activePlanId = useMemo(() => {
    if (!plansQuery.data) return null;
    return (
      plansQuery.data.items.find((p) => isDateInRange(today, p.weekStartDate))
        ?.mealPlanID ?? null
    );
  }, [plansQuery.data, today]);

  const planQuery = useQuery({
    queryKey: ["mealPlan", activePlanId],
    queryFn: () => api.getMealPlan(activePlanId!),
    enabled: !!activePlanId,
  });

  const recipesQuery = useQuery({
    queryKey: ["recipes"],
    queryFn: () => api.getRecipes(),
    enabled: !!activePlanId,
  });

  const suggestionsQuery = useQuery({
    queryKey: ["recommendedRecipes"],
    queryFn: () => api.getRecommendedRecipes(10),
  });

  const restockQuery = useQuery({
    queryKey: ["suggestedRestock"],
    queryFn: () => api.getSuggestedRestockItems(5),
  });

  const queryClient = useQueryClient();
  const { me } = useMe();
  const invitesQuery = useQuery({
    queryKey: ["householdInvites"],
    queryFn: () => api.getHouseholdInvites(),
  });
  const pendingIncoming = useMemo(
    () =>
      (invitesQuery.data ?? []).filter(
        (i) => i.status === "PENDING" && i.toUser.userID === me?.userID
      ),
    [invitesQuery.data, me?.userID]
  );
  const inviteAction = useMutation({
    mutationFn: async ({ id, accept }: { id: number; accept: boolean }): Promise<unknown> =>
      accept ? api.acceptHouseholdInvite(id) : api.declineHouseholdInvite(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["householdInvites"] });
      queryClient.invalidateQueries({ queryKey: ["myHousehold"] });
    },
  });

  const todaySlots = useMemo(() => {
    if (!planQuery.data?.mealSlots) return [];
    return planQuery.data.mealSlots.filter((s) => s.dayOfWeek === todayDay);
  }, [planQuery.data, todayDay]);

  const recipeName = (recipeId: number | null) => {
    if (!recipeId) return "Nothing planned";
    return (
      recipesQuery.data?.find((r) => r.recipeID === recipeId)?.recipeName ??
      `Recipe ${recipeId}`
    );
  };

  if (plansQuery.isLoading) return <DashboardSkeleton />;
  if (plansQuery.error)
    return <Alert severity="error">{(plansQuery.error as Error).message}</Alert>;
  if (planQuery.isLoading || recipesQuery.isLoading)
    return <DashboardSkeleton />;
  if (planQuery.error)
    return <Alert severity="error">{(planQuery.error as Error).message}</Alert>;
  if (recipesQuery.error)
    return <Alert severity="error">{(recipesQuery.error as Error).message}</Alert>;

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Dashboard
      </Typography>
      <Typography variant="h6" color="text.secondary" gutterBottom>
        {today.toLocaleDateString(undefined, {
          weekday: "long",
          month: "long",
          day: "numeric",
        })}{" "}
        — here's what's cooking
      </Typography>
      {pendingIncoming.length > 0 && (
        <Paper sx={{ p: 2, mb: 2 }}>
          <Typography variant="h6" gutterBottom>
            Household invitations
          </Typography>
          {pendingIncoming.map((inv) => (
            <Box
              key={inv.inviteID}
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: 2,
              }}
            >
              <Typography>
                {inv.fromUser.displayName ??
                  [inv.fromUser.firstName, inv.fromUser.lastName]
                    .filter(Boolean)
                    .join(" ") ??
                  `User ${inv.fromUser.userID}`}{" "}
                invited you to their household
              </Typography>
              <Box sx={{ display: "flex", gap: 1 }}>
                <Button
                  size="small"
                  variant="contained"
                  onClick={() =>
                    inviteAction.mutate({ id: inv.inviteID, accept: true })
                  }
                  disabled={inviteAction.isPending}
                >
                  Accept
                </Button>
                <Button
                  size="small"
                  onClick={() =>
                    inviteAction.mutate({ id: inv.inviteID, accept: false })
                  }
                  disabled={inviteAction.isPending}
                >
                  Decline
                </Button>
              </Box>
            </Box>
          ))}
        </Paper>
      )}
      <Paper sx={{ p: 2 }}>
        <Typography variant="h6" gutterBottom>
          Today's meals
        </Typography>
        {activePlanId === null ? (
          <Typography color="text.secondary">
            No meal plan for this week yet.{" "}
            <PlanMealLink href="/meal-plans">Plan one →</PlanMealLink>
          </Typography>
        ) : (
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", sm: "repeat(3, 1fr)" },
              gap: 2,
            }}
          >
            {MEAL_TYPES.map((meal, mt) => {
              const slot = todaySlots.find((s) => s.mealType === mt);
              return (
                <Box
                  key={meal}
                  sx={{
                    bgcolor: alpha(theme.palette.primary.main, 0.06),
                    borderRadius: 2,
                    p: 2,
                  }}
                >
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 0.75,
                      color: "text.secondary",
                      mb: 0.5,
                    }}
                  >
                    {createElement(MEAL_ICONS[mt], { fontSize: "small" })}
                    <Typography variant="subtitle2">{meal}</Typography>
                  </Box>
                  {slot ? (
                    <Typography variant="body2">
                      {slot.recipe?.recipeName ?? recipeName(slot.recipeID)}
                    </Typography>
                  ) : (
                    <Box
                      sx={{
                        display: "flex",
                        flexDirection: "column",
                        alignItems: "flex-start",
                        gap: 0.5,
                        py: 0.5,
                      }}
                    >
                      {createElement(MEAL_ICONS[mt], {
                        sx: {
                          fontSize: 40,
                          color: alpha(theme.palette.primary.main, 0.35),
                        },
                      })}
                      <Typography variant="body2" color="text.secondary">
                        Nothing planned for {meal.toLowerCase()} yet
                      </Typography>
                      <PlanMealLink href="/meal-plans">Plan it →</PlanMealLink>
                    </Box>
                  )}
                </Box>
              );
            })}
          </Box>
        )}
      </Paper>

      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
          gap: 3,
          mt: 3,
          alignItems: "start",
        }}
      >
        <Paper
          sx={{
            p: 2,
            gridColumn: {
              md:
                (restockQuery.data ?? []).length === 0 ? "1 / -1" : "auto",
            },
          }}
        >
          <Typography variant="h6" gutterBottom>
            Delicious ideas for tonight
          </Typography>
        {suggestionsQuery.isLoading && <CircularProgress />}
        {suggestionsQuery.error && (
          <Alert severity="error">
            {(suggestionsQuery.error as Error).message}
          </Alert>
        )}
        {!suggestionsQuery.isLoading &&
          !suggestionsQuery.error &&
          (suggestionsQuery.data ?? []).length === 0 && (
            <Typography color="text.secondary">
              Nothing to suggest yet — rate a few recipes and we'll get ideas
              flowing.
            </Typography>
          )}
          {(suggestionsQuery.data ?? []).length > 0 && (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              {(suggestionsQuery.data ?? []).slice(0, 5).map((s) => {
                const meta = suggestionMeta(s.recipe);
                return (
                  <Paper
                    key={`${s.recipe.recipeID}-${s.reason}`}
                    variant="outlined"
                    sx={{
                      p: 1.5,
                      display: "flex",
                      alignItems: "center",
                      gap: 1.5,
                    }}
                  >
                    <Box
                      sx={{
                        width: 48,
                        height: 48,
                        borderRadius: 2,
                        bgcolor: accentFor(theme, s.recipe.recipeID),
                        color: theme.palette.background.paper,
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        flexShrink: 0,
                      }}
                    >
                      {createElement(suggestionIcon(s.recipe))}
                    </Box>
                    <Box sx={{ flexGrow: 1, minWidth: 0 }}>
                      <Typography variant="subtitle2" component="div">
                        <Link href={`/recipes/${s.recipe.recipeID}`}>
                          {s.recipe.recipeName}
                        </Link>
                      </Typography>
                      {meta && (
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          sx={{ display: "block" }}
                        >
                          {meta}
                        </Typography>
                      )}
                    </Box>
                    <Chip
                      size="small"
                      label={reasonLabel(s.reason)}
                      sx={{
                        bgcolor: alpha(theme.palette.success.main, 0.12),
                        color: "success.dark",
                        fontWeight: 500,
                        "& .MuiChip-label": { px: 1.25 },
                      }}
                    />
                  </Paper>
                );
              })}
              {(suggestionsQuery.data ?? []).length > 5 && (
                <Typography variant="body2">
                  <Link href="/recipes">
                    and {(suggestionsQuery.data ?? []).length - 5} more…
                  </Link>
                </Typography>
              )}
            </Box>
          )}
        </Paper>

        {(restockQuery.data ?? []).length > 0 && (
          <Paper sx={{ p: 2 }}>
            <Typography variant="h6" gutterBottom>
              Time to restock
            </Typography>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              {(restockQuery.data ?? []).map((it) => {
                const size = sizeBadge(it.name, it.unit);
                return (
                  <Box
                    key={it.itemID}
                    sx={{ display: "flex", alignItems: "center", gap: 1.25 }}
                  >
                    <Box
                      sx={{
                        width: 8,
                        height: 8,
                        borderRadius: "50%",
                        bgcolor: "warning.main",
                        flexShrink: 0,
                      }}
                    />
                    <Typography variant="body2" sx={{ minWidth: 0 }}>
                      {stripSize(it.name, size, it.brand)}
                      {it.brand && (
                        <Typography
                          component="span"
                          variant="body2"
                          color="text.secondary"
                        >
                          {" "}
                          — {it.brand}
                        </Typography>
                      )}
                    </Typography>
                    {size && (
                      <Chip
                        size="small"
                        variant="outlined"
                        label={size}
                        sx={{
                          color: "text.secondary",
                          borderColor: "divider",
                          "& .MuiChip-label": { px: 1.25 },
                        }}
                      />
                    )}
                  </Box>
                );
              })}
            </Box>
            <Typography variant="body2" sx={{ mt: 1 }}>
              <Link href="/grocery-lists">Open grocery lists</Link>
            </Typography>
          </Paper>
        )}
      </Box>
    </Box>
  );
}
