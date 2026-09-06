"use client";

import { use, useState, useEffect, Fragment } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import TextField from "@mui/material/TextField";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import Select from "@mui/material/Select";
import MenuItem from "@mui/material/MenuItem";
import Chip from "@mui/material/Chip";
import Alert from "@mui/material/Alert";
import CircularProgress from "@mui/material/CircularProgress";
import IconButton from "@mui/material/IconButton";
import DeleteIcon from "@mui/icons-material/Delete";
import Autocomplete from "@mui/material/Autocomplete";
import { api, asEntity } from "@/lib/api";
import CrudDialog, { FieldDef } from "@/app/components/CrudDialog";
import {
  AuditableEntity,
  MealSlot,
  MealSlotItem,
  Recipe,
  MealPlanNutrition,
  Brand,
} from "@/lib/types";

const recipeFields: FieldDef<Recipe>[] = [
  { key: "recipeName", label: "Recipe Name" },
  { key: "description", label: "Description" },
  { key: "servings", label: "Servings", type: "number" },
  { key: "prepTimeMinutes", label: "Prep Time (min)", type: "number" },
  { key: "cookTimeMinutes", label: "Cook Time (min)", type: "number" },
  { key: "isActive", label: "Active", type: "boolean" },
];

const DAY_NAMES = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const MEAL_TYPES = ["Breakfast", "Lunch", "Dinner"];
const MEAL_TYPE_IDS = [0, 1, 2];

interface SlotDialogState {
  open: boolean;
  day: number;
  mealType: number;
  slot: MealSlot | null;
}

function SlotDialog({
  planId,
  state,
  onClose,
}: {
  planId: number;
  state: SlotDialogState;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const { slot, day, mealType } = state;

  const recipesQuery = useQuery({
    queryKey: ["recipes"],
    queryFn: () => api.getRecipes(),
  });

  const itemsQuery = useQuery({
    queryKey: ["items"],
    queryFn: () => api.getItems(),
  });

  const [newItemId, setNewItemId] = useState<string>("");
  const [newQty, setNewQty] = useState<string>("");
  const [newUnit, setNewUnit] = useState<string>("");
  const [brand, setBrand] = useState<string | "">("");
  const [brandInput, setBrandInput] = useState("");
  const [itemSearch, setItemSearch] = useState("");
  const [debouncedItemSearch, setDebouncedItemSearch] = useState("");

  const brandsQuery = useQuery({
    queryKey: ["item-brands", brandInput],
    queryFn: () =>
      brandInput === ""
        ? api.getFrequentBrands(10)
        : api.getBrands(brandInput),
  });

  const brandOptions = brandsQuery.data ?? [];
  const selectedBrand =
    brand === "" ? null : brandOptions.find((b) => b.brandName === brand) ?? null;

  const searchQuery = useQuery({
    queryKey: ["items-search", debouncedItemSearch, brand],
    queryFn: () =>
      api.searchItems(
        debouncedItemSearch,
        brand,
        brand !== "" && debouncedItemSearch.length === 0 ? 1000 : 50
      ),
    enabled: brand !== "" || debouncedItemSearch.length >= 2,
  });

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedItemSearch(itemSearch), 300);
    return () => clearTimeout(timer);
  }, [itemSearch]);

  useEffect(() => {
    if (debouncedItemSearch.trim()) {
      void api.recordSearch("item", debouncedItemSearch);
    }
  }, [debouncedItemSearch]);

  const [recipeId, setRecipeId] = useState<string>(
    slot?.recipeID ? String(slot.recipeID) : ""
  );

  const selectedRecipeId = recipeId === "" ? null : Number(recipeId);

  const recipeDetailQuery = useQuery({
    queryKey: ["recipe", selectedRecipeId],
    queryFn: () => (selectedRecipeId ? api.getRecipe(selectedRecipeId) : null),
    enabled: selectedRecipeId !== null,
  });

  const selectedRecipe =
    recipeDetailQuery.data ??
    (selectedRecipeId
      ? recipesQuery.data?.find((r) => r.recipeID === selectedRecipeId)
      : null);
  const [selectedOptionalIds, setSelectedOptionalIds] = useState<string[]>(
    () =>
      (slot?.mealSlotItems?.filter((i) => i.isFromRecipe) ?? []).map((i) =>
        String(i.itemID)
      )
  );
  const [servings, setServings] = useState<string>(
    String(slot?.servings ?? 1)
  );
  const [replacementNote, setReplacementNote] = useState(
    slot?.replacementNote ?? ""
  );
  const [adhoc, setAdhoc] = useState<{ itemID: number; quantity: string; unit: string }[]>(
    () =>
      (slot?.mealSlotItems?.filter((i) => !i.isFromRecipe) ?? []).map((i) => ({
        itemID: i.itemID ?? 0,
        quantity: String(i.quantity),
        unit: i.unitOfMeasure ?? "",
      }))
  );

  const [recipeDialogOpen, setRecipeDialogOpen] = useState(false);

  const slotSaveMutation = useMutation({
    mutationFn: async () => {
      const rid = selectedRecipeId;
      const slotServings = Number(servings) > 0 ? Number(servings) : 1;
      const recipeOptional = selectedRecipe?.recipeItems?.filter((ri) => ri.isOptional) ?? [];

      let currentSlot = slot;
      if (!currentSlot) {
        currentSlot = await api.addMealSlot(planId, {
          dayOfWeek: day,
          mealType,
          recipeID: rid,
          servings: slotServings,
          replacementNote: replacementNote || null,
        } as Omit<MealSlot, "mealSlotID" | "mealPlanID" | "mealPlan" | "recipe" | "mealSlotItems">);
      } else {
        currentSlot = await api.updateMealSlot(planId, currentSlot.mealSlotID, {
          ...currentSlot,
          mealPlanID: planId,
          dayOfWeek: day,
          mealType,
          recipeID: rid,
          servings: slotServings,
          replacementNote: replacementNote || null,
        });
      }

      const slotId = currentSlot.mealSlotID;

      const fromRecipe = currentSlot.mealSlotItems?.filter((i) => i.isFromRecipe) ?? [];
      for (const i of fromRecipe) {
        await api.deleteMealSlotItem(slotId, i.mealSlotItemID);
      }

      for (const oid of selectedOptionalIds) {
        const recipeItem = recipeOptional.find((ri) => String(ri.itemID) === oid);
        if (recipeItem && currentSlot) {
          await api.addMealSlotItem(slotId, {
            itemID: Number(oid),
            quantity: Number(recipeItem.quantity),
            unitOfMeasure: recipeItem.unitOfMeasure,
            isFromRecipe: true,
          } as Omit<MealSlotItem, "mealSlotItemID" | "mealSlotID" | "mealSlot" | "item">);
        }
      }

      for (const a of adhoc) {
        await api.addMealSlotItem(slotId, {
          itemID: a.itemID,
          quantity: Number(a.quantity),
          unitOfMeasure: a.unit,
          isFromRecipe: false,
        } as Omit<MealSlotItem, "mealSlotItemID" | "mealSlotID" | "mealSlot" | "item">);
      }

      return currentSlot;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["mealPlan", planId] });
      queryClient.invalidateQueries({ queryKey: ["mealPlanNutrition", planId] });
      onClose();
    },
  });

  const createRecipeMutation = useMutation({
    mutationFn: (row: Record<string, unknown>) =>
      api.createRecipe(asEntity<Omit<Recipe, keyof AuditableEntity>>(row)),
    onSuccess: (recipe) => {
      setRecipeId(String(recipe.recipeID));
      setSelectedOptionalIds([]);
      setAdhoc([]);
      queryClient.invalidateQueries({ queryKey: ["recipes"] });
      setRecipeDialogOpen(false);
    },
  });

  const deleteSlotMutation = useMutation({
    mutationFn: () =>
      slot ? api.deleteMealSlot(slot.mealPlanID, slot.mealSlotID) : Promise.resolve(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["mealPlan", planId] });
      queryClient.invalidateQueries({ queryKey: ["mealPlanNutrition", planId] });
      onClose();
    },
  });

  const optionalItems =
    selectedRecipe?.recipeItems?.filter((ri) => ri.isOptional) ?? [];

  const handleAddAdhoc = () => {
    if (newItemId !== "" && newQty !== "") {
      setAdhoc((prev) => [
        ...prev,
        { itemID: Number(newItemId), quantity: newQty, unit: newUnit },
      ]);
      setNewItemId("");
      setNewQty("");
      setNewUnit("");
      setBrand("");
      setItemSearch("");
      setDebouncedItemSearch("");
    }
  };

  const handleRemoveAdhoc = (index: number) => {
    setAdhoc((prev) => prev.filter((_, i) => i !== index));
  };

  return (
    <>
      <Paper sx={{ p: 3, minWidth: 400, maxWidth: 600 }}>
        <Typography variant="h6" gutterBottom>
          {DAY_NAMES[day]} - {MEAL_TYPES[mealType]}
        </Typography>

        {(slotSaveMutation.error || deleteSlotMutation.error) && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {((slotSaveMutation.error ?? deleteSlotMutation.error) as Error).message}
          </Alert>
        )}

        <Box sx={{ mb: 2 }}>
          <FormControl fullWidth size="small" sx={{ mb: 1 }}>
            <InputLabel id="recipe-select-label">Recipe</InputLabel>
            <Select
              labelId="recipe-select-label"
              label="Recipe"
              value={recipeId}
              onChange={(e) => {
                setRecipeId(e.target.value as string);
                setSelectedOptionalIds([]);
              }}
            >
              <MenuItem value="">
                <em>Blank</em>
              </MenuItem>
              {(recipesQuery.data ?? []).map((r) => (
                <MenuItem key={r.recipeID} value={String(r.recipeID)}>
                  {r.recipeName}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <Button size="small" onClick={() => setRecipeDialogOpen(true)}>
            + Create New Recipe
          </Button>
        </Box>

        {recipeId !== "" && optionalItems.length > 0 && (
          <FormControl fullWidth size="small" sx={{ mb: 2 }}>
            <InputLabel id="optional-select-label">Include Optional Ingredients</InputLabel>
            <Select
              labelId="optional-select-label"
              label="Include Optional Ingredients"
              multiple
              value={selectedOptionalIds}
              onChange={(e) =>
                setSelectedOptionalIds(
                  typeof e.target.value === "string"
                    ? [e.target.value]
                    : (e.target.value as string[])
                )
              }
              renderValue={(selected) => (
                <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5 }}>
                  {(selected as string[]).map((value) => {
                    const item = itemsQuery.data?.find(
                      (i) => String(i.itemID) === value
                    );
                    return (
                      <Chip
                        key={value}
                        label={item?.name ?? value}
                        size="small"
                      />
                    );
                  })}
                </Box>
              )}
            >
              {optionalItems.map((oi) => {
                const item = itemsQuery.data?.find((i) => i.itemID === oi.itemID);
                return (
                  <MenuItem key={oi.itemID} value={String(oi.itemID)}>
                    {item?.name ?? oi.itemID} ({oi.quantity} {oi.unitOfMeasure})
                  </MenuItem>
                );
              })}
            </Select>
          </FormControl>
        )}

        <TextField
          fullWidth
          size="small"
          label="Servings"
          type="number"
          value={servings}
          onChange={(e) => setServings(e.target.value)}
          helperText="Servings of the recipe planned for this slot"
          sx={{ mb: 2 }}
        />

        <TextField
          fullWidth
          size="small"
          label="Replacement Note"
          value={replacementNote}
          onChange={(e) => setReplacementNote(e.target.value)}
          sx={{ mb: 2 }}
        />

        <Typography variant="subtitle2" gutterBottom>
          Ad-hoc Additional Items
        </Typography>
        <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap", mb: 1 }}>
          <Autocomplete
            size="small"
            options={brandOptions}
            getOptionLabel={(b) => (typeof b === "string" ? b : b?.brandName ?? "")}
            isOptionEqualToValue={(a, b) =>
              (typeof a === "object" && typeof b === "object" && a?.brandID === b?.brandID)
            }
            inputValue={brandInput}
            onInputChange={(_, value) => setBrandInput(value)}
            value={selectedBrand}
            onChange={(_, value) => {
              const b = value as Brand | null;
              setBrand(b?.brandName ?? "");
              setBrandInput(b?.brandName ?? "");
              setNewItemId("");
              setNewUnit("");
              setItemSearch("");
              setDebouncedItemSearch("");
              if (b) void api.recordSelection("brand", b.brandID);
            }}
            filterOptions={(options) => options}
            loading={brandsQuery.isLoading}
            noOptionsText="No brands found"
            renderInput={(params) => (
              <TextField {...params} label="Brand" size="small" />
            )}
            sx={{ minWidth: 160 }}
          />
          <Autocomplete
            size="small"
            options={searchQuery.data ?? []}
            getOptionLabel={(item) => item?.name ?? ""}
            isOptionEqualToValue={(option, value) =>
              option?.itemID === value?.itemID
            }
            inputValue={itemSearch}
            onInputChange={(_, value) => setItemSearch(value)}
            value={
              searchQuery.data?.find(
                (item) => String(item.itemID) === newItemId
              ) ?? null
            }
            onChange={(_, value) => {
              setNewItemId(value ? String(value.itemID) : "");
              setNewUnit(value ? value.unit ?? "" : "");
              if (value) void api.recordSelection("item", value.itemID);
            }}
            filterOptions={(options) => options}
            loading={searchQuery.isLoading}
            noOptionsText={
              brand === "" && debouncedItemSearch.length < 2
                ? "Type at least 2 characters"
                : brand !== "" && debouncedItemSearch.length === 0
                ? "No items for this brand"
                : "No items found"
            }
            renderOption={(props, item) => (
              <li {...props} key={item.itemID}>
                {item.name}
                {item.brand ? ` — ${item.brand}` : ""}
              </li>
            )}
            renderInput={(params) => (
              <TextField {...params} label="Item" size="small" />
            )}
            sx={{ minWidth: 260 }}
          />
          <TextField
            size="small"
            label="Qty"
            type="number"
            value={newQty}
            onChange={(e) => setNewQty(e.target.value)}
          />
          <TextField
            size="small"
            label="Unit"
            value={newUnit}
            onChange={(e) => setNewUnit(e.target.value)}
            helperText="Match the item's inventory unit"
          />
          <Button variant="outlined" onClick={handleAddAdhoc}>
            Add
          </Button>
        </Box>

        {adhoc.map((a, i) => {
          const item = itemsQuery.data?.find((it) => it.itemID === a.itemID);
          const itemName = item
            ? (item.brand ? `${item.brand} — ${item.name}` : item.name)
            : `Item ${a.itemID}`;
          return (
            <Box
              key={i}
              sx={{ display: "flex", alignItems: "center", gap: 1, mb: 0.5 }}
            >
              <Typography variant="body2">
                {itemName} - {a.quantity} {a.unit}
              </Typography>
              <IconButton size="small" onClick={() => handleRemoveAdhoc(i)}>
                <DeleteIcon fontSize="small" />
              </IconButton>
            </Box>
          );
        })}

        <Box sx={{ display: "flex", gap: 1, mt: 2 }}>
          <Button
            variant="contained"
            onClick={() => slotSaveMutation.mutate()}
            disabled={slotSaveMutation.isPending || deleteSlotMutation.isPending}
          >
            Save
          </Button>
          {slot && (
            <Button
              variant="outlined"
              color="error"
              onClick={() => deleteSlotMutation.mutate()}
              disabled={slotSaveMutation.isPending || deleteSlotMutation.isPending}
            >
              Remove Slot
            </Button>
          )}
          <Button onClick={onClose}>Cancel</Button>
        </Box>
      </Paper>

      <CrudDialog
        open={recipeDialogOpen}
        title="Create Recipe"
        fields={recipeFields}
        values={{ isActive: true }}
        onClose={() => setRecipeDialogOpen(false)}
        onSave={(values) => createRecipeMutation.mutate(values)}
      />
    </>
  );
}

function NutritionPanel({ nutrition }: { nutrition: MealPlanNutrition }) {
  return (
    <Box>
      <Typography variant="h5" gutterBottom>
        Nutrition
      </Typography>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fill, minmax(260px, 1fr))",
          gap: 2,
        }}
      >
        {nutrition.dailyTotals.map((day) => (
          <Paper key={day.dayOfWeek} sx={{ p: 2 }}>
            <Typography variant="h6">{DAY_NAMES[day.dayOfWeek]}</Typography>
            {day.nutrients.map((n) => (
              <Typography key={n.nutrientId} variant="body2">
                {n.nutrientName}: {Number(n.amount).toFixed(2)} {n.unitOfMeasure}
              </Typography>
            ))}
          </Paper>
        ))}
      </Box>
    </Box>
  );
}

export default function MealPlanDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const planId = Number(id);
  const router = useRouter();
  const [slotDialog, setSlotDialog] = useState<SlotDialogState | null>(null);

  const planQuery = useQuery({
    queryKey: ["mealPlan", planId],
    queryFn: () => api.getMealPlan(planId),
  });

  const recipesQuery = useQuery({
    queryKey: ["recipes"],
    queryFn: () => api.getRecipes(),
  });

  const nutritionQuery = useQuery({
    queryKey: ["mealPlanNutrition", planId],
    queryFn: () => api.getMealPlanNutrition(planId),
  });

  const generateGroceryListMutation = useMutation({
    mutationFn: () => api.generateGroceryList(planId),
    onSuccess: (list) => {
      router.push(`/grocery-lists/${list.groceryListID}`);
    },
  });

  const itemsQuery = useQuery({
    queryKey: ["items"],
    queryFn: () => api.getItems(),
  });

  const findSlot = (day: number, mealType: number) =>
    planQuery.data?.mealSlots?.find(
      (s) => s.dayOfWeek === day && s.mealType === mealType
    ) ?? null;

  const itemLabel = (itemId: number) => {
    const item = itemsQuery.data?.find((i) => i.itemID === itemId);
    if (!item) return `Item ${itemId}`;
    return item.brand ? `${item.brand} — ${item.name}` : item.name;
  };

  const recipeName = (rid: number | null) =>
    recipesQuery.data?.find((r) => r.recipeID === rid)?.recipeName ??
    (rid ? `Recipe ${rid}` : "Blank");

  const mealNutrition = (day: number, mealType: number) =>
    nutritionQuery.data?.meals.find(
      (m) => m.dayOfWeek === day && m.mealType === mealType
    );

  if (planQuery.isLoading) return <CircularProgress />;
  if (planQuery.error)
    return <Alert severity="error">{(planQuery.error as Error).message}</Alert>;
  if (!planQuery.data) return <Alert severity="warning">Meal plan not found</Alert>;

  const plan = planQuery.data;

  return (
    <Box>
      <Paper sx={{ p: 3, mb: 3 }}>
        <Box
          sx={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "flex-start",
            flexWrap: "wrap",
          }}
        >
          <Box>
            <Typography variant="h4" gutterBottom>
              {plan.planName}
            </Typography>
            <Typography variant="body1" color="text.secondary" gutterBottom>
              Week starting {plan.weekStartDate?.split("T")[0]} (
              {DAY_NAMES[plan.weekStartDayOfWeek]})
            </Typography>
          </Box>
          <Button
            variant="contained"
            onClick={() => generateGroceryListMutation.mutate()}
            disabled={generateGroceryListMutation.isPending}
          >
            Generate Grocery List
          </Button>
        </Box>
      </Paper>

      <Paper sx={{ p: 3, mb: 3, overflowX: "auto" }}>
        <Typography variant="h5" gutterBottom>
          Weekly Grid
        </Typography>
        <Box sx={{ display: "grid", gridTemplateColumns: "repeat(4, minmax(140px, 1fr))", gap: 1 }}>
          <Box sx={{ fontWeight: 700 }}></Box>
          {MEAL_TYPE_IDS.map((mt) => (
            <Box key={mt} sx={{ textAlign: "center", fontWeight: 700 }}>
              {MEAL_TYPES[mt]}
            </Box>
          ))}

          {Array.from({ length: 7 }).map((_, day) => (
            <Fragment key={day}>
              <Box sx={{ fontWeight: 700 }}>{DAY_NAMES[day]}</Box>
              {MEAL_TYPE_IDS.map((mt) => {
                const slot = findSlot(day, mt);
                const nut = mealNutrition(day, mt);
                return (
                  <Paper
                    key={`${day}-${mt}`}
                    sx={{
                      p: 1,
                      minHeight: 100,
                      cursor: "pointer",
                      border: slot ? "1px solid" : "1px dashed",
                      borderColor: slot ? "primary.main" : "divider",
                    }}
                    onClick={() =>
                      setSlotDialog({
                        open: true,
                        day,
                        mealType: mt,
                        slot,
                      })
                    }
                  >
                    <Typography variant="body2" sx={{ fontWeight: 600 }}>
                      {recipeName(slot?.recipeID ?? null)}
                    </Typography>
                    {slot?.replacementNote && (
                      <Typography variant="caption" color="text.secondary">
                        {slot.replacementNote}
                      </Typography>
                    )}
                    {slot?.mealSlotItems && slot.mealSlotItems.length > 0 && (
                      <Box sx={{ mt: 0.5 }}>
                        {slot.mealSlotItems.map((it) => (
                          <Chip
                            key={it.mealSlotItemID}
                            label={`${itemLabel(it.itemID)}${it.quantity ? ` - ${it.quantity} ${it.unitOfMeasure ?? ""}`.trim() : ""}`}
                            size="small"
                            sx={{ mr: 0.5, mb: 0.5 }}
                          />
                        ))}
                      </Box>
                    )}
                    {nut && nut.nutrients.length > 0 && (
                      <Box sx={{ mt: 0.5 }}>
                        <Typography variant="caption" color="text.secondary">
                          {nut.nutrients
                            .map((n) => `${n.nutrientName}: ${Number(n.amount).toFixed(1)}`)
                            .join(", ")}
                        </Typography>
                      </Box>
                    )}
                  </Paper>
                );
              })}
            </Fragment>
          ))}
        </Box>
      </Paper>

      {nutritionQuery.data && (
        <NutritionPanel nutrition={nutritionQuery.data} />
      )}

      {slotDialog?.open && (
        <Box
          sx={{
            position: "fixed",
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            bgcolor: "rgba(0,0,0,0.4)",
            zIndex: 1300,
          }}
          onClick={(e) => {
            if (e.target === e.currentTarget) setSlotDialog(null);
          }}
        >
          <SlotDialog
            planId={planId}
            state={slotDialog}
            onClose={() => setSlotDialog(null)}
          />
        </Box>
      )}
    </Box>
  );
}
