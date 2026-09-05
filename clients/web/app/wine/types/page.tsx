"use client";

import CrudPage from "@/app/components/CrudPage";
import { api, asEntity } from "@/lib/api";

export default function TypesPage() {
  return (
    <CrudPage
      title="Types"
      queryKey={["types"]}
      pagedListFn={api.getTypesPaged}
      fields={[
        { key: "typeName", label: "Type Name" },
        { key: "description", label: "Description" },
      ]}
      createFn={(row) => api.createType(asEntity(row))}
      updateFn={(row) =>
        api.updateType(row.typeID as number, asEntity(row))
      }
      deleteFn={(row) => api.deleteType(row.typeID)}
    />
  );
}
