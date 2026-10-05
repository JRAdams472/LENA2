"use client";

import CrudPage from "@/app/components/CrudPage";
import { api, asEntity } from "@/lib/api";

export default function FoodFlavorsPage() {
  return (
    <CrudPage
      title="Food Flavors"
      queryKey={["food-flavors"]}
      pagedListFn={api.getFoodFlavorsPaged}
      tableFields={[
        {
          key: "foodId",
          label: "Item",
          render: (r) => r.item?.name ?? `#${r.foodId}`,
        },
        {
          key: "flavorId",
          label: "Flavor",
          render: (r) => r.flavorProfile?.flavorName ?? `#${r.flavorId}`,
        },
        { key: "intensityScore", label: "Intensity Score", type: "number" },
      ]}
      fields={[
        { key: "foodId", label: "Food ID", type: "number" },
        { key: "flavorId", label: "Flavor ID", type: "number" },
        { key: "intensityScore", label: "Intensity Score", type: "number" },
      ]}
      createFn={(row) => api.createFoodFlavor(asEntity(row))}
      updateFn={(row) =>
        api.updateFoodFlavor(
          row.foodId as number,
          row.flavorId as number,
          asEntity(row)
        )
      }
      deleteFn={(row) =>
        api.deleteFoodFlavor(row.foodId, row.flavorId)
      }
    />
  );
}
