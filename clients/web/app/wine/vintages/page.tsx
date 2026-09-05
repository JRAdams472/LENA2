"use client";

import CrudPage from "@/app/components/CrudPage";
import { api, asEntity } from "@/lib/api";

export default function VintagesPage() {
  return (
    <CrudPage
      title="Vintages"
      queryKey={["vintages"]}
      pagedListFn={api.getVintagesPaged}
      activeOnlyFn={api.getActiveVintages}
      fields={[
        { key: "year", label: "Year", type: "number" },
        { key: "description", label: "Description" },
        { key: "isActive", label: "Active", type: "boolean" },
      ]}
      createFn={(row) => api.createVintage(asEntity(row))}
      updateFn={(row) =>
        api.updateVintage(row.vintageID as number, asEntity(row))
      }
      deleteFn={(row) => api.deleteVintage(row.vintageID)}
    />
  );
}
