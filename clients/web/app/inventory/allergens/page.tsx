"use client";

import CrudPage from "@/app/components/CrudPage";
import { api } from "@/lib/api";
import { Allergen } from "@/lib/types";

// The allergen registry — member records and ingredient/item flags both
// key on these rows. There is no hard delete: removal is deactivation so
// existing references stay resolvable.
export default function AllergensPage() {
  return (
    <CrudPage<Allergen>
      title="Allergens"
      queryKey={["allergens"]}
      listFn={api.getAllergens}
      fields={[
        { key: "name", label: "Name" },
        { key: "description", label: "Description" },
        { key: "isActive", label: "Active", type: "boolean" },
      ]}
      createFn={(row) =>
        api.createAllergen({
          name: String(row.name ?? ""),
          description: (row.description as string) || null,
        })
      }
      updateFn={(row) =>
        api.updateAllergen(row.allergenID as number, {
          name: row.name as string,
          description: (row.description as string) || null,
          isActive: Boolean(row.isActive),
        })
      }
      deleteFn={(row) => api.updateAllergen(row.allergenID, { isActive: false })}
    />
  );
}
