"use client";

import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Autocomplete from "@mui/material/Autocomplete";
import TextField from "@mui/material/TextField";
import { api } from "@/lib/api";
import { Ingredient } from "@/lib/types";

const CREATE_ID = -1;

// IngredientAutocomplete searches the generic ingredient catalog and can
// mint a missing one inline via getOrCreateIngredient — the backend
// dedupes on the normalized name so free-creation can't double up.
export default function IngredientAutocomplete({
  value,
  onChange,
  label = "Ingredient",
  helperText,
  size = "small",
  sx,
  onSelect,
  onInputTextChange,
}: {
  value: Ingredient | null;
  onChange: (ingredient: Ingredient | null) => void;
  label?: string;
  helperText?: string;
  size?: "small" | "medium";
  sx?: object;
  // Fires once the resolved ingredient lands — picked or freshly created.
  onSelect?: (ingredient: Ingredient) => void;
  // Exposes the raw input text for callers tracking search analytics.
  onInputTextChange?: (text: string) => void;
}) {
  const [input, setInput] = useState("");
  const [debounced, setDebounced] = useState("");

  useEffect(() => {
    const t = setTimeout(() => setDebounced(input), 300);
    return () => clearTimeout(t);
  }, [input]);

  useEffect(() => {
    onInputTextChange?.(debounced);
  }, [debounced, onInputTextChange]);

  // External value changes (e.g. the parent clearing after a save) reset
  // the visible text to match.
  useEffect(() => {
    // eslint-disable-next-line @eslint-react/set-state-in-effect
    setInput(value?.name ?? "");
  }, [value]);

  const query = useQuery({
    queryKey: ["ingredient-autocomplete", debounced],
    queryFn: () => api.searchIngredients(debounced, 20),
  });

  return (
    <Autocomplete<Ingredient, false, false, false>
      size={size}
      options={query.data ?? []}
      getOptionLabel={(i) => (i.ingredientID === CREATE_ID ? "" : i.name)}
      isOptionEqualToValue={(o, v) => o.ingredientID === v.ingredientID}
      inputValue={input}
      onInputChange={(_, v) => setInput(v)}
      value={value}
      onChange={(_, v) => {
        if (v?.ingredientID === CREATE_ID) {
          const name = input.trim();
          if (name) {
            void api.getOrCreateIngredient({ name }).then((ing) => {
              onChange(ing);
              setInput(ing.name);
              onSelect?.(ing);
            });
          }
          return;
        }
        onChange(v);
        if (v) onSelect?.(v);
      }}
      filterOptions={(options, params) => {
        const iv = params.inputValue.trim();
        const exact = options.some(
          (o) => o.name.toLowerCase() === iv.toLowerCase()
        );
        if (iv !== "" && !exact) {
          return [
            ...options,
            {
              ingredientID: CREATE_ID,
              name: iv,
              category: null,
              defaultUnit: null,
              isActive: true,
            },
          ];
        }
        return options;
      }}
      renderOption={(props, i) => {
        const { key, ...liProps } = props;
        return (
          <li key={i.ingredientID} {...liProps}>
            {i.ingredientID === CREATE_ID ? `Create "${i.name}"` : i.name}
          </li>
        );
      }}
      loading={query.isLoading}
      noOptionsText="No ingredients found"
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          helperText={helperText}
          margin={size === "medium" ? "dense" : undefined}
          fullWidth={size === "medium"}
        />
      )}
      sx={sx}
    />
  );
}
