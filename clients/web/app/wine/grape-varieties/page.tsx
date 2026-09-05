"use client";

import CrudPage from "@/app/components/CrudPage";
import { api, asEntity } from "@/lib/api";

export default function GrapeVarietiesPage() {
  return (
    <CrudPage
      title="Grape Varieties"
      queryKey={["grapeVarieties"]}
      listFn={api.getGrapeVarieties}
      activeOnlyFn={api.getActiveGrapeVarieties}
      fields={[
        { key: "grapeVarietyName", label: "Name", sortable: true },
        { key: "description", label: "Description" },
        { key: "isActive", label: "Active", type: "boolean", sortable: true },
      ]}
      createFn={(row) => api.createGrapeVariety(asEntity(row))}
      updateFn={(row) =>
        api.updateGrapeVariety(row.grapeVarietyID as number, asEntity(row))
      }
      deleteFn={(row) => api.deleteGrapeVariety(row.grapeVarietyID)}
    />
  );
}
