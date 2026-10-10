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
  title: "Ingredients — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Ingredients"} intro={"LENA draws a line between the food and the product:"}>
        <DocList items={[
          <Fragment key={0}>{"An "}<strong key={0}>{"ingredient"}</strong>{" is the generic thing a recipe calls for — "}<em key={1}>{"garlic"}</em>{", "}<em key={2}>{"olive oil"}</em>{", "}<em key={3}>{"corn"}</em>{"."}</Fragment>,
          <Fragment key={1}>{"An "}<strong key={0}>{"item"}</strong>{" is a branded product you can actually buy — "}<em key={1}>{"Ahold Garlic Salt"}</em>{", "}<em key={2}>{"Green Giant Corn Niblets"}</em>{"."}</Fragment>,
        ]} />
        <P>{<Fragment key={2}>{"Recipes, meal plans, and grocery lists can reference either, so a recipe can say \"corn\" without committing you to a brand — and your pantry stays item-precise."}</Fragment>}</P>
      <Section title={"Recipes: ingredient first, brand optional"}>
        <P>{<Fragment key={3}>{"When you add a line to a recipe, the "}<strong key={0}>{"Ingredient"}</strong>{" picker comes first. Type a name to search the catalog of generic ingredients, or pick "}<strong key={1}>{"Create \"…\""}</strong>{" to mint a new one on the spot. The "}<strong key={2}>{"Preferred brand"}</strong>{" picker underneath is optional — use it when the recipe genuinely wants a specific product (\"Heinz ketchup\"), and the row still displays ingredient-first."}</Fragment>}</P>
        <Screenshot src={"/images/recipe-detail-ingredients.png"} alt={"The recipe editor — ingredient picker, optional preferred brand, and a mixed item table"} device="web" />
        <P>{<Fragment key={4}>{"A line can be:"}</Fragment>}</P>
        <DocTable head={["Form","Example","What it means"]} rows={[[<Fragment key={5}>{"Ingredient only"}</Fragment>, <Fragment key={6}><code key={0}>{"garlic"}</code></Fragment>, <Fragment key={7}>{"Any garlic product counts"}</Fragment>], [<Fragment key={8}>{"Ingredient + preferred brand"}</Fragment>, <Fragment key={9}><code key={0}>{"salt — Ahold Garlic Salt"}</code></Fragment>, <Fragment key={10}>{"Prefer this product, but it's still \"salt\""}</Fragment>], [<Fragment key={11}>{"Item only (classic)"}</Fragment>, <Fragment key={12}><code key={0}>{"Ahold Angel Hair Pasta"}</code></Fragment>, <Fragment key={13}>{"Exactly this product"}</Fragment>]]} />
        <P>{<Fragment key={14}>{"Existing branded recipe lines keep working unchanged — ingredients are additive, not a migration you have to do."}</Fragment>}</P>
      </Section>
      <Section title={"Usual brands"}>
        <P>{<Fragment key={15}>{"When a grocery line is ingredient-only, the first time you check it off LENA asks "}<strong key={0}>{"\"Which \\<ingredient\\> did you buy?\""}</strong>{" Pick the product you grabbed and two things happen: the line is checked and the pantry is credited with "}<em key={1}>{"that item"}</em>{", and LENA remembers it as your household's "}<strong key={2}>{"usual brand"}</strong>{" for that ingredient."}</Fragment>}</P>
        <Screenshot src={"/images/grocery-brand-picker.png"} alt={"The brand picker on first check-off"} device="web" />
        <P>{<Fragment key={16}>{"From then on, that ingredient's grocery lines show a \"usually buys …\" caption and check straight off against the remembered product — no picker. You can also tap "}<strong key={0}>{"Check off without a brand"}</strong>{" to credit the ingredient generically instead."}</Fragment>}</P>
        <Screenshot src={"/images/grocery-list.png"} alt={"A checked ingredient line showing its remembered usual brand"} device="web" />
      </Section>
      <Section title={"Household overrides"}>
        <P>{<Fragment key={17}>{"A catalog item can carry a "}<em key={0}>{"canonical"}</em>{" ingredient (admins set it under "}<strong key={1}>{"Inventory → Items → edit"}</strong>{" — the "}<strong key={2}>{"Catalog ingredient"}</strong>{" field). If your household buys a product that's categorized differently — or isn't categorized at all — the "}<strong key={3}>{"Household ingredient"}</strong>{" field on the same dialog lets "}<em key={4}>{"your"}</em>{" household map it to a different ingredient without touching the shared catalog. Your override wins everywhere your household looks; other households keep the catalog mapping."}</Fragment>}</P>
      </Section>
      <Section title={"Managing ingredients (admins)"}>
        <P>{<Fragment key={18}><strong key={0}>{"Inventory → Ingredients"}</strong>{" lists every ingredient with search, create, edit, deactivate — and "}<strong key={1}>{"Merge into…"}</strong>{", which repoints every recipe line, grocery line, usual-brand memory, and household override from a duplicate into the canonical ingredient. Use it when \"carrot\" and \"carrots\" both got created."}</Fragment>}</P>
        <Screenshot src={"/images/ingredients-admin.png"} alt={"The ingredients admin page"} device="web" />
        <P>{<Fragment key={19}>{"Ingredients also carry "}<strong key={0}>{"allergen flags"}</strong>{" ("}<code key={1}>{"contains"}</code>{" / "}<code key={2}>{"may contain"}</code>{", set in the ingredient edit dialog) — flagging an ingredient protects every recipe that references it. See "}<DocLink key={3} href="/docs/allergy-tracking">{"Allergy Tracking"}</DocLink>{"."}</Fragment>}</P>
      </Section>
      <Section title={"Where ingredients show up"}>
        <DocList items={[
          <Fragment key={20}><strong key={0}>{"Grocery lists"}</strong>{" — ingredient-primary labels with \"usually buys …\" brand captions; the "}<DocLink key={1} href="/docs/grocery-lists">{"brand picker"}</DocLink>{" on first check-off"}</Fragment>,
          <Fragment key={21}><strong key={0}>{"Allergy warnings"}</strong>{" — ingredient flags roll up into recipe, meal-plan, and grocery warnings ("}<DocLink key={1} href="/docs/allergy-tracking">{"Allergy Tracking"}</DocLink>{")"}</Fragment>,
          <Fragment key={22}><strong key={0}>{"Meal plans & events"}</strong>{" — recipe lines display the ingredient name with the preferred brand after a dash"}</Fragment>,
          <Fragment key={23}><strong key={0}>{"Household tweaks"}</strong>{" — a "}<DocLink key={1} href="/docs/recipes#household-tweaks">{"tweak"}</DocLink>{" can swap a line to a different ingredient or add an ingredient-keyed line without touching the shared recipe"}</Fragment>,
          <Fragment key={24}><strong key={0}>{"Barcode scanning"}</strong>{" — a scanned item shows its resolved ingredient, and unlinked items offer to map one"}</Fragment>,
          <Fragment key={25}><strong key={0}>{"Mobile"}</strong>{" — the same pickers and flows as the web ("}<DocLink key={1} href="/docs/mobile-grocery">{"Grocery Lists"}</DocLink>{", "}<DocLink key={2} href="/docs/mobile-recipes">{"Recipes"}</DocLink>{")"}</Fragment>,
        ]} />
      </Section>
    </DocPage>
  );
}
