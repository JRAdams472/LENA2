import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  DocLink,
  DocList,
  DocTable,
  P,
  Section,
  Screenshot,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Inventory & Pantry — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Inventory & Pantry"} intro={"LENA splits food into two layers, and keeping them straight makes everything else click:"}>
        <DocList items={[
          <Fragment key={0}><strong key={0}>{"The catalog"}</strong>{" — a shared reference of items, brands, categories, units, and nutrition info. Think of it as \"things that exist in the world.\""}</Fragment>,
          <Fragment key={1}><strong key={0}>{"Your pantry"}</strong>{" — what "}<em key={1}>{"your household"}</em>{" currently has in stock, and how much."}</Fragment>,
        ]} />
      <Section title={"The item catalog"}>
        <P>{<Fragment key={2}>{"Under "}<strong key={0}>{"Inventory"}</strong>{" (web) you'll find the catalog's building blocks:"}</Fragment>}</P>
        <DocTable head={["Page","What it holds"]} rows={[[<Fragment key={3}><strong key={0}>{"Items"}</strong></Fragment>, <Fragment key={4}>{"The products themselves — name, brand, category, expiry info, and their generic ingredient"}</Fragment>], [<Fragment key={5}><strong key={0}>{"Ingredients"}</strong></Fragment>, <Fragment key={6}>{"Generic foods recipes can name without a brand — "}<em key={0}>{"corn"}</em>{", not "}<em key={1}>{"Green Giant corn"}</em>{" (see "}<DocLink key={2} href="/docs/ingredients">{"Ingredients"}</DocLink>{")"}</Fragment>], [<Fragment key={7}><strong key={0}>{"Brands"}</strong></Fragment>, <Fragment key={8}>{"The brands items come in"}</Fragment>], [<Fragment key={9}><strong key={0}>{"Categories"}</strong></Fragment>, <Fragment key={10}>{"How items are grouped (produce, dairy, spices…) — the "}<em key={0}>{"Protein"}</em>{" flag drives defrost reminders"}</Fragment>], [<Fragment key={11}><strong key={0}>{"Food Nutrients / Nutrient Types"}</strong></Fragment>, <Fragment key={12}>{"Nutrition info attached to items"}</Fragment>], [<Fragment key={13}><strong key={0}>{"Food Flavors / Flavor Profiles"}</strong></Fragment>, <Fragment key={14}>{"Taste descriptors used for pairing and recommendations"}</Fragment>], [<Fragment key={15}><strong key={0}>{"Allergens"}</strong></Fragment>, <Fragment key={16}>{"The allergen taxonomy plus per-item/per-ingredient "}<code key={0}>{"contains"}</code>{" / "}<code key={1}>{"may contain"}</code>{" flags — drives cooking warnings (see "}<DocLink key={2} href="/docs/allergy-tracking">{"Allergy Tracking"}</DocLink>{")"}</Fragment>], [<Fragment key={17}><strong key={0}>{"Allergen Suggestions"}</strong></Fragment>, <Fragment key={18}>{"Admin review queue for assistant-proposed allergen flags — nothing applies until a human accepts (see "}<DocLink key={0} href="/docs/allergy-tracking">{"Allergy Tracking"}</DocLink>{")"}</Fragment>]]} />
        <P>{<Fragment key={19}><strong key={0}>{"Items"}</strong>{" is the catalog's front door — every product with its brand, stock quantity, and favorite flag. Search and the In Stock / Favorites toggles make a large catalog manageable:"}</Fragment>}</P>
        <Screenshot src={"/images/inventory-items.png"} alt={"Web — the Items catalog: search, brand filter, stock and favorite toggles"} device="web" />
        <P>{<Fragment key={20}>{"The same page in dark mode — the catalog is fully theme-aware:"}</Fragment>}</P>
        <Screenshot src={"/images/inventory-items-dark.png"} alt={"Web — the item catalog in dark mode"} device="web" />
        <P>{<Fragment key={21}><strong key={0}>{"Categories"}</strong>{" group items (produce, dairy, spices…). The "}<em key={1}>{"Protein"}</em>{" flag on a category is what drives the freezer-defrost reminders:"}</Fragment>}</P>
        <Screenshot src={"/images/inventory-categories.png"} alt={"Web — Categories admin, where the Protein flag lives"} device="web" />
        <P>{<Fragment key={22}>{"The supporting catalogs are admin-managed too. "}<strong key={0}>{"Brands"}</strong>{" are the names items come in:"}</Fragment>}</P>
        <Screenshot src={"/images/inventory-brands.png"} alt={"Web — Brands admin, the shared brand vocabulary"} device="web" />
        <P>{<Fragment key={23}><strong key={0}>{"Nutrient Types"}</strong>{" define the units nutrition is expressed in (calories, grams of protein…); "}<strong key={1}>{"Food Nutrients"}</strong>{" attach actual values to items and ingredients — together they power the meal-plan Nutrition sheet:"}</Fragment>}</P>
        <Screenshot src={"/images/inventory-nutrient-types.png"} alt={"Web — Nutrient Types admin"} device="web" />
        <Screenshot src={"/images/inventory-food-nutrients.png"} alt={"Web — Food Nutrients attached to an item"} device="web" />
        <P>{<Fragment key={24}><strong key={0}>{"Flavor Profiles"}</strong>{" are the taste vocabulary and "}<strong key={1}>{"Food Flavors"}</strong>{" apply it to items and ingredients — both feed pairing and recommendation features:"}</Fragment>}</P>
        <Screenshot src={"/images/inventory-flavor-profiles.png"} alt={"Web — Flavor Profiles admin"} device="web" />
        <Screenshot src={"/images/inventory-food-flavors.png"} alt={"Web — Food Flavors on an item or ingredient"} device="web" />
        <P>{<Fragment key={25}>{"Recipes reference catalog items as ingredients, which is why the catalog matters — the richer it is, the better your grocery lists and recommendations get."}</Fragment>}</P>
        <P>{<Fragment key={26}>{"The "}<strong key={0}>{"Items"}</strong>{" grid shows each product's linked ingredient, and the item edit dialog carries two ingredient fields — "}<strong key={1}>{"Catalog ingredient"}</strong>{" (admin, shared) and "}<strong key={2}>{"Household ingredient"}</strong>{" (your household's private override). See "}<DocLink key={3} href="/docs/ingredients">{"Ingredients"}</DocLink>{" for how generic ingredients, usual brands, and overrides fit together."}</Fragment>}</P>
      </Section>
      <Section title={"Your pantry stock"}>
        <P>{<Fragment key={27}>{"The pantry ("}<code key={0}>{"userItems"}</code>{" in the data model) tracks what you actually have — browsable on mobile under the "}<DocLink key={1} href="/docs/mobile-pantry">{"Pantry tab"}</DocLink>{":"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={28}><strong key={0}>{"Current quantity"}</strong>{" — the number your grocery lists subtract from recipe needs"}</Fragment>,
          <Fragment key={29}><strong key={0}>{"Minimum quantity"}</strong>{" — your \"don't run out\" threshold on staples (flour, oil, coffee). Set it on the web Items page; when stock falls to or below it, the item surfaces under "}<strong key={1}>{"Time to restock"}</strong>{" on the Dashboard and on your grocery list (see "}<DocLink key={2} href="/docs/analytics">{"Analytics & Recommendations"}</DocLink>{")"}</Fragment>,
          <Fragment key={30}><strong key={0}>{"Expiry date"}</strong>{" — set it on the web Items page and LENA sends an "}<em key={1}>{"expiring soon"}</em>{" reminder a few days ahead, with an "}<strong key={2}>{"Add to list"}</strong>{" action to restock (see "}<DocLink key={3} href="/docs/dashboard">{"Notifications"}</DocLink>{")"}</Fragment>,
          <Fragment key={31}><strong key={0}>{"Notes"}</strong></Fragment>,
        ]} />
        <P>{<Fragment key={32}>{"Keeping counts honest matters more than the exact numbers: a grocery list generated tomorrow subtracts whatever the pantry says you have, so stale counts mean buying duplicates — or missing something entirely."}</Fragment>}</P>
      </Section>
      <Section title={"Scanning: the fast way to keep counts honest"}>
        <P>{<Fragment key={33}>{"The mobile app's "}<strong key={0}>{"Scan"}</strong>{" tab is built for this — see "}<DocLink key={1} href="/docs/mobile-scanning">{"Barcode Scanning"}</DocLink>{" for the full flow, and "}<DocLink key={2} href="/docs/mobile-pantry">{"Pantry on Mobile"}</DocLink>{" for the stock list:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={34}><strong key={0}>{"Scan a barcode of something you bought"}</strong>{" → LENA finds the catalog item by UPC → add quantity to pantry."}</Fragment>,
          <Fragment key={35}><strong key={0}>{"Scan it again when you use it up"}</strong>{" → subtract from pantry."}</Fragment>,
        ]} />
        <P>{<Fragment key={36}>{"If the barcode is unknown, LENA offers to "}<strong key={0}>{"submit the item"}</strong>{" — fill in its name and details, and it enters a review queue ("}<strong key={1}>{"Pending Items"}</strong>{", admin-only on the web) before joining the catalog."}</Fragment>}</P>
      </Section>
      <Section title={"Approving new items (web, admins)"}>
        <P>{<Fragment key={37}>{"Submitted items land on "}<strong key={0}>{"Pending Items"}</strong>{" (admin-only, under "}<strong key={1}>{"Administration"}</strong>{" in the sidebar) where an admin approves or rejects them — this keeps the shared catalog clean while letting anyone contribute via the scanner."}</Fragment>}</P>
        <Screenshot src={"/images/items-pending.png"} alt={"Web — the Pending Items review queue"} device="web" />
      </Section>
      <Section title={"Why it matters"}>
        <DocList items={[
          <Fragment key={38}>{"Grocery lists subtract what you already have — accurate counts mean fewer duplicate purchases"}</Fragment>,
          <Fragment key={39}>{"Item records power recipe ingredients, barcode lookups, and nutrition tracking"}</Fragment>,
        ]} />
      </Section>
    </DocPage>
  );
}
