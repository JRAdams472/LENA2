"use client";

import CrudPage from "@/app/components/CrudPage";
import { api, asEntity } from "@/lib/api";

export default function FoodNutrientsPage() {
  return (
    <CrudPage
      title="Food Nutrients"
      queryKey={["food-nutrients"]}
      pagedListFn={api.getFoodNutrientsPaged}
      tableFields={[
        {
          key: "foodId",
          label: "Item",
          render: (r) => r.item?.name ?? `#${r.foodId}`,
        },
        {
          key: "nutrientId",
          label: "Nutrient",
          render: (r) => r.nutrientType?.nutrientName ?? `#${r.nutrientId}`,
        },
        {
          key: "amountPerServing",
          label: "Amount per Serving",
          render: (r) =>
            `${r.amountPerServing} ${r.nutrientType?.unitOfMeasure ?? ""}`.trim(),
        },
      ]}
      fields={[
        { key: "foodId", label: "Food ID", type: "number" },
        { key: "nutrientId", label: "Nutrient ID", type: "number" },
        { key: "amountPerServing", label: "Amount per Serving", type: "number" },
      ]}
      createFn={(row) => api.createFoodNutrient(asEntity(row))}
      updateFn={(row) =>
        api.updateFoodNutrient(
          row.foodId as number,
          row.nutrientId as number,
          asEntity(row)
        )
      }
      deleteFn={(row) =>
        api.deleteFoodNutrient(row.foodId, row.nutrientId)
      }
    />
  );
}
