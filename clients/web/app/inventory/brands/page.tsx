"use client";

import CrudPage from "@/app/components/CrudPage";
import { api, asEntity } from "@/lib/api";

export default function BrandsPage() {
  return (
    <CrudPage
      title="Brands"
      queryKey={["brands"]}
      pagedListFn={api.getBrandsPaged}
      searchFn={api.getBrands}
      fields={[{ key: "brandName", label: "Brand Name" }]}
      createFn={(row) => api.createBrand(asEntity(row))}
      updateFn={(row) =>
        api.updateBrand(row.brandID as number, asEntity(row))
      }
      deleteFn={(row) => api.deleteBrand(row.brandID)}
    />
  );
}
