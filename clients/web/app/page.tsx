"use client";

import { useMemo } from "react";
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
import FreeBreakfastIcon from "@mui/icons-material/FreeBreakfast";
import LunchDiningIcon from "@mui/icons-material/LunchDining";
import DinnerDiningIcon from "@mui/icons-material/DinnerDining";
import { alpha, useTheme } from "@mui/material/styles";

const MEAL_TYPES = ["Breakfast", "Lunch", "Dinner"];

const MEAL_ICONS = [
  <FreeBreakfastIcon key="breakfast" fontSize="small" />,
  <LunchDiningIcon key="lunch" fontSize="small" />,
  <DinnerDiningIcon key="dinner" fontSize="small" />,
];

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

const SIZE_RE =
  /\b(\d+(?:\.\d+)?\s?(?:pk|ct|count|pack|oz|fl\.?\s?oz|lb|g|kg|ml|l))\b/i;

function sizeBadge(name: string, unit: string): string | null {
  const m = name.match(SIZE_RE);
  if (m) return m[1];
  return unit && unit !== "each" ? unit : null;
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
    if (!recipeId) return "Blank";
    return (
      recipesQuery.data?.find((r) => r.recipeID === recipeId)?.recipeName ??
      `Recipe ${recipeId}`
    );
  };

  if (plansQuery.isLoading) return <CircularProgress />;
  if (plansQuery.error)
    return <Alert severity="error">{(plansQuery.error as Error).message}</Alert>;
  if (planQuery.isLoading || recipesQuery.isLoading) return <CircularProgress />;
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
        })}
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
            No meal plan for this week.{" "}
            <Link href="/meal-plans">+ Plan a meal</Link>
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
                  sx={{ bgcolor: "#f8fafc", borderRadius: 2, p: 2 }}
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
                    {MEAL_ICONS[mt]}
                    <Typography variant="subtitle2">{meal}</Typography>
                  </Box>
                  {slot ? (
                    <Typography variant="body2">
                      {slot.recipe?.recipeName ?? recipeName(slot.recipeID)}
                    </Typography>
                  ) : (
                    <Typography
                      variant="body2"
                      sx={{ fontStyle: "italic", color: "text.secondary" }}
                    >
                      <Link href="/meal-plans">+ Plan a meal</Link>
                    </Typography>
                  )}
                </Box>
              );
            })}
          </Box>
        )}
      </Paper>

      <Paper sx={{ p: 2, mt: 3 }}>
        <Typography variant="h6" gutterBottom>
          Suggested for You
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
              No suggestions yet — rate some recipes and plan a few meals.
            </Typography>
          )}
        {(suggestionsQuery.data ?? []).length > 0 && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            {(suggestionsQuery.data ?? []).map((s) => (
              <Box
                key={`${s.recipe.recipeID}-${s.reason}`}
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1.5,
                  flexWrap: "wrap",
                }}
              >
                <Link href={`/recipes/${s.recipe.recipeID}`}>
                  {s.recipe.recipeName}
                </Link>
                <Chip
                  size="small"
                  label={reasonLabel(s.reason)}
                  sx={{
                    bgcolor: alpha(theme.palette.success.main, 0.12),
                    color: "success.dark",
                    fontWeight: 500,
                  }}
                />
              </Box>
            ))}
          </Box>
        )}
      </Paper>

      {(restockQuery.data ?? []).length > 0 && (
        <Paper sx={{ p: 2, mt: 3 }}>
          <Typography variant="h6" gutterBottom>
            Running low
          </Typography>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            {(restockQuery.data ?? []).map((it) => (
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
                <Typography variant="body2">
                  {it.name}
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
                {sizeBadge(it.name, it.unit) && (
                  <Chip
                    size="small"
                    variant="outlined"
                    label={sizeBadge(it.name, it.unit)}
                    sx={{ color: "text.secondary", borderColor: "divider" }}
                  />
                )}
              </Box>
            ))}
          </Box>
          <Typography variant="body2" sx={{ mt: 1 }}>
            <Link href="/grocery-lists">Open grocery lists</Link>
          </Typography>
        </Paper>
      )}
    </Box>
  );
}
