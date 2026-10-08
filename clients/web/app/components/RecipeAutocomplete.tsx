"use client";

import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Autocomplete from "@mui/material/Autocomplete";
import TextField from "@mui/material/TextField";
import { api } from "@/lib/api";
import { Recipe } from "@/lib/types";

// RecipeAutocomplete searches the household recipe catalog server-side —
// pickers never bulk-download it. mealType/categoryIds narrow on the
// server, matching the recipes(page:1) shape the recipes page uses.
export default function RecipeAutocomplete({
  value,
  onChange,
  label = "Recipe",
  size = "small",
  sx,
  mealType,
  categoryIds,
  onSelect,
}: {
  value: Recipe | null;
  onChange: (recipe: Recipe | null) => void;
  label?: string;
  size?: "small" | "medium";
  sx?: object;
  // Course filter (e.g. "Dinner") applied server-side.
  mealType?: string;
  // Restrict results to recipes carrying any of these category ids.
  categoryIds?: number[];
  onSelect?: (recipe: Recipe) => void;
}) {
  const [input, setInput] = useState("");
  const [debounced, setDebounced] = useState("");

  useEffect(() => {
    const t = setTimeout(() => setDebounced(input), 300);
    return () => clearTimeout(t);
  }, [input]);

  // External value changes (e.g. the parent clearing after a save) reset
  // the visible text to match.
  useEffect(() => {
    // eslint-disable-next-line @eslint-react/set-state-in-effect
    setInput(value?.recipeName ?? "");
  }, [value]);

  const query = useQuery({
    queryKey: [
      "recipe-autocomplete",
      debounced,
      mealType ?? null,
      categoryIds ?? [],
    ],
    queryFn: () =>
      api.getRecipesPaged(
        1,
        25,
        debounced || undefined,
        undefined,
        categoryIds,
        undefined,
        mealType
      ),
  });

  const options = query.data?.items ?? [];
  // Keep the current value in the option list so its label renders even
  // when the active search excludes it.
  const merged =
    value && !options.some((o) => o.recipeID === value.recipeID)
      ? [value, ...options]
      : options;

  return (
    <Autocomplete<Recipe, false, false, false>
      size={size}
      options={merged}
      getOptionLabel={(r) => r.recipeName}
      isOptionEqualToValue={(o, v) => o.recipeID === v.recipeID}
      inputValue={input}
      onInputChange={(_, v) => setInput(v)}
      value={value}
      onChange={(_, v) => {
        onChange(v);
        if (v) onSelect?.(v);
      }}
      filterOptions={(x) => x}
      loading={query.isLoading}
      noOptionsText="No recipes found"
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          margin={size === "medium" ? "dense" : undefined}
          fullWidth={size === "medium"}
        />
      )}
      sx={sx}
    />
  );
}
