"use client";

import CrudPage from "@/app/components/CrudPage";
import { api, asEntity } from "@/lib/api";

export default function WineFlavorProfilesPage() {
  return (
    <CrudPage
      title="Wine Flavor Profiles"
      queryKey={["wineFlavorProfiles"]}
      listFn={api.getWineFlavorProfiles}
      activeOnlyFn={api.getActiveWineFlavorProfiles}
      fields={[
        { key: "flavorProfileName", label: "Name", sortable: true },
        { key: "description", label: "Description" },
        { key: "isActive", label: "Active", type: "boolean", sortable: true },
      ]}
      createFn={(row) => api.createWineFlavorProfile(asEntity(row))}
      updateFn={(row) =>
        api.updateWineFlavorProfile(row.flavorProfileID as number, asEntity(row))
      }
      deleteFn={(row) => api.deleteWineFlavorProfile(row.flavorProfileID)}
    />
  );
}
