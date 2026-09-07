"use client";

import { useMemo, Fragment } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import Typography from "@mui/material/Typography";
import Box from "@mui/material/Box";
import CircularProgress from "@mui/material/CircularProgress";
import Alert from "@mui/material/Alert";
import Paper from "@mui/material/Paper";

const MEAL_TYPES = ["Breakfast", "Lunch", "Dinner"];

const REASON_LABELS: Record<string, string> = {
  ingredient_overlap: "Similar to your menu",
  rating_recency: "Due for a revisit",
  collaborative_filtering: "Recommended for you",
};

function reasonLabel(reason: string) {
  return REASON_LABELS[reason] ?? "Recommended for you";
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
      <Paper sx={{ p: 2 }}>
        {activePlanId === null ? (
          <Typography color="text.secondary">
            No meal plan for this week.
          </Typography>
        ) : todaySlots.length === 0 ? (
          <Typography color="text.secondary">No meals planned for today.</Typography>
        ) : (
          <Box sx={{ display: "grid", gridTemplateColumns: "1fr 2fr", gap: 2 }}>
            {MEAL_TYPES.map((meal, mt) => {
              const slot = todaySlots.find((s) => s.mealType === mt);
              return (
                <Fragment key={meal}>
                  <Box sx={{ fontWeight: 700 }}>{meal}</Box>
                  <Box>
                    {slot
                      ? slot.recipe?.recipeName ?? recipeName(slot.recipeID)
                      : "Nothing planned"}
                  </Box>
                </Fragment>
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
          <Box component="ul" sx={{ m: 0, pl: 2 }}>
            {(suggestionsQuery.data ?? []).map((s) => (
              <li key={`${s.recipe.recipeID}-${s.reason}`}>
                <Link href={`/recipes/${s.recipe.recipeID}`}>
                  {s.recipe.recipeName}
                </Link>{" "}
                <Typography component="span" variant="body2" color="text.secondary">
                  — {reasonLabel(s.reason)}
                </Typography>
              </li>
            ))}
          </Box>
        )}
      </Paper>
    </Box>
  );
}
