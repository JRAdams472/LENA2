"use client";

import CrudPage from "@/app/components/CrudPage";
import { api, asEntity } from "@/lib/api";

export default function FoodNutrientsPage() {
  return (
    <CrudPage
      title="Food Nutrients"
      queryKey={["food-nutrients"]}
      listFn={api.getFoodNutrients}
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
