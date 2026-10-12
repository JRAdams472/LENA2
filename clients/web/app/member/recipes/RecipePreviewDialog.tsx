"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import Divider from "@mui/material/Divider";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import Typography from "@mui/material/Typography";
import useMediaQuery from "@mui/material/useMediaQuery";
import { useTheme } from "@mui/material/styles";
import { api } from "@/lib/api";
import { recipeLineText } from "@/lib/format";
import { RecipeItem } from "@/lib/types";

// Read-only peek at a recipe's ingredients + steps — opened from the
// recipes table without entering the editor.
export default function RecipePreviewDialog({
  recipeId,
  onClose,
}: {
  recipeId: number | null;
  onClose: () => void;
}) {
  const theme = useTheme();
  const fullScreen = useMediaQuery(theme.breakpoints.down("sm"));

  const recipeQuery = useQuery({
    queryKey: ["recipe", recipeId],
    queryFn: () => api.getRecipe(recipeId!),
    enabled: recipeId != null,
  });
  const recipe = recipeQuery.data;

  // Group lines by section, preserving their display order — most
  // recipes have no section, in which case this is one flat list.
  const sections = useMemo(() => {
    const items = recipe?.recipeItems ?? [];
    const groups: { name: string | null; items: RecipeItem[] }[] = [];
    for (const item of items) {
      const name = item.section ?? null;
      const last = groups[groups.length - 1];
      if (last && last.name === name) {
        last.items.push(item);
      } else {
        groups.push({ name, items: [item] });
      }
    }
    return groups;
  }, [recipe]);

  const steps = useMemo(
    () =>
      [...(recipe?.recipeSteps ?? [])].sort(
        (a, b) => a.stepNumber - b.stepNumber
      ),
    [recipe]
  );

  return (
    <Dialog
      open={recipeId != null}
      onClose={onClose}
      fullWidth
      fullScreen={fullScreen}
      maxWidth="md"
    >
      <DialogTitle>{recipe?.recipeName ?? "Recipe"}</DialogTitle>
      <DialogContent dividers>
        {recipeQuery.isLoading && <CircularProgress />}
        {recipeQuery.error && (
          <Alert severity="error">
            {(recipeQuery.error as Error).message}
          </Alert>
        )}
        {recipe && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Box>
              <Typography variant="h6" gutterBottom>
                Ingredients
              </Typography>
              {sections.every((s) => s.items.length === 0) && (
                <Typography color="text.secondary">No ingredients</Typography>
              )}
              {sections.map((section, i) => (
                <Box
                  key={String(
                    section.items[0].recipeItemID ?? section.name
                  )}
                >
                  {section.name && (
                    <Typography
                      variant="subtitle2"
                      color="text.secondary"
                      sx={{ mt: i > 0 ? 1.5 : 0 }}
                    >
                      {section.name}
                    </Typography>
                  )}
                  <List dense disablePadding>
                    {section.items.map((item) => (
                      <ListItem key={item.recipeItemID ?? recipeLineText(item)} disableGutters>
                        <ListItemText
                          primary={
                            recipeLineText(item) +
                            (item.isOptional ? " (optional)" : "")
                          }
                          secondary={item.notes}
                        />
                      </ListItem>
                    ))}
                  </List>
                </Box>
              ))}
            </Box>
            <Divider />
            <Box>
              <Typography variant="h6" gutterBottom>
                Steps
              </Typography>
              {steps.length === 0 && (
                <Typography color="text.secondary">No steps</Typography>
              )}
              <List dense disablePadding>
                {steps.map((step) => (
                  <ListItem key={step.recipeStepID} disableGutters>
                    <ListItemText
                      primary={`${step.stepNumber}. ${step.instruction}`}
                      secondary={
                        step.durationMinutes != null
                          ? `${step.durationMinutes} min`
                          : undefined
                      }
                    />
                  </ListItem>
                ))}
              </List>
            </Box>
          </Box>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}
