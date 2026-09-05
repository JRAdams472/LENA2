"use client";

import CrudPage from "@/app/components/CrudPage";
import { api, asEntity } from "@/lib/api";

export default function CategoriesPage() {
  return (
    <CrudPage
      title="Categories"
      queryKey={["categories"]}
      listFn={api.getCategories}
      activeOnlyFn={api.getActiveCategories}
      fields={[
        { key: "categoryName", label: "Name", sortable: true },
        { key: "description", label: "Description" },
        { key: "isActive", label: "Active", type: "boolean", sortable: true },
      ]}
      createFn={(row) => api.createCategory(asEntity(row))}
      updateFn={(row) =>
        api.updateCategory(row.categoryID as number, asEntity(row))
      }
      deleteFn={(row) => api.deleteCategory(row.categoryID)}
    />
  );
}
