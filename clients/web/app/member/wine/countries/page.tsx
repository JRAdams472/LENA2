"use client";

import CrudPage from "@/app/components/CrudPage";
import { api, asEntity } from "@/lib/api";

export default function CountriesPage() {
  return (
    <CrudPage
      title="Countries"
      queryKey={["countries"]}
      pagedListFn={api.getCountriesPaged}
      fields={[
        { key: "countryName", label: "Country Name", sortable: true },
        { key: "isoCode", label: "Country Code", sortable: true },
      ]}
      createFn={(row) => api.createCountry(asEntity(row))}
      updateFn={(row) =>
        api.updateCountry(row.countryID as number, asEntity(row))
      }
      deleteFn={(row) => api.deleteCountry(row.countryID)}
    />
  );
}
