"use client";

import CrudPage from "@/app/components/CrudPage";
import { api, asEntity } from "@/lib/api";

export default function FlavorProfilesPage() {
  return (
    <CrudPage
      title="Flavor Profiles"
      queryKey={["flavor-profiles"]}
      listFn={api.getFlavorProfiles}
      activeOnlyFn={api.getActiveFlavorProfiles}
      fields={[
        { key: "flavorName", label: "Flavor Name" },
        { key: "isActive", label: "Active", type: "boolean" },
      ]}
      createFn={(row) => api.createFlavorProfile(asEntity(row))}
      updateFn={(row) =>
        api.updateFlavorProfile(row.flavorId as number, asEntity(row))
      }
      deleteFn={(row) => api.deleteFlavorProfile(row.flavorId)}
    />
  );
}
