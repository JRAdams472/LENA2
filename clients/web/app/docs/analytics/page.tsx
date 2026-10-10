import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  DocList,
  DocTable,
  P,
  Section,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Analytics & Recommendations — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Analytics & Recommendations"} intro={"LENA quietly learns from how your household uses it and feeds that back into search results, recipe suggestions, and restock prompts. You never have to configure any of it — normal use is the training data."}>
      <Section title={"Ranked search everywhere"}>
        <P>{<Fragment key={0}>{"Every search and picker — items, brands, ingredients, recipes, wine bottles, pantry — orders results by how likely "}<em key={0}>{"you"}</em>{" are to want them, not alphabetically. Roughly, the ranking tiers are:"}</Fragment>}</P>
        <DocList ordered items={[
          <Fragment key={1}><strong key={0}>{"Favorites"}</strong>{" you've marked"}</Fragment>,
          <Fragment key={2}><strong key={0}>{"What your household uses"}</strong>{" — items picked in menus, grocery lists, and events"}</Fragment>,
          <Fragment key={3}><strong key={0}>{"What you've viewed or selected before"}</strong></Fragment>,
          <Fragment key={4}><strong key={0}>{"Things matching terms you've searched for"}</strong></Fragment>,
          <Fragment key={5}>{"Everything else, alphabetical"}</Fragment>,
        ]} />
        <P>{<Fragment key={6}>{"On shared catalogs (items, recipes, brands) your own history outranks your household's, which outranks global usage across all households. Signals also "}<strong key={0}>{"decay over time"}</strong>{" — something you used last week counts for more than something from a year ago. With no history at all, results simply fall back to alphabetical, so nothing ever looks broken."}</Fragment>}</P>
        <P>{<Fragment key={7}>{"Every search you run and every pick you make records a small event, so the ranking keeps improving the more you use the app."}</Fragment>}</P>
      </Section>
      <Section title={"Recipe suggestions"}>
        <P>{<Fragment key={8}>{"The Dashboard's "}<strong key={0}>{"Delicious ideas for tonight"}</strong>{" card merges several recommendation sources and shows why each recipe appeared:"}</Fragment>}</P>
        <DocTable head={["Reason","Where it comes from"]} rows={[[<Fragment key={9}><strong key={0}>{"Similar to your menu"}</strong></Fragment>, <Fragment key={10}>{"Ingredient overlap with recipes already on your meal plan"}</Fragment>], [<Fragment key={11}><strong key={0}>{"Due for a revisit"}</strong></Fragment>, <Fragment key={12}>{"Good rating + long time since it last appeared on your menus"}</Fragment>], [<Fragment key={13}><strong key={0}>{"Matches your household's tastes"}</strong></Fragment>, <Fragment key={14}>{"Categories your household picks most often"}</Fragment>], [<Fragment key={15}><strong key={0}>{"Trending in your household"}</strong></Fragment>, <Fragment key={16}>{"Recent velocity — on your menus much more lately than historically"}</Fragment>]]} />
      </Section>
      <Section title={"Meal-type-aware recipe pickers"}>
        <P>{<Fragment key={17}>{"When you fill a meal-plan slot, the recipe list boosts recipes tagged with that slot's course — breakfast dishes rise in the Breakfast slot, dinner dishes in Dinner. Unknown or untagged recipes still appear, just lower down."}</Fragment>}</P>
      </Section>
      <Section title={"Restock suggestions"}>
        <P>{<Fragment key={18}>{"Set a "}<strong key={0}>{"minimum quantity"}</strong>{" on any pantry item (web "}<strong key={1}>{"Items"}</strong>{" page, "}<em key={2}>{"Min Quantity"}</em>{" field). When stock falls to or below it:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={19}>{"the item appears under "}<strong key={0}>{"Time to restock"}</strong>{" on the Dashboard, and"}</Fragment>,
          <Fragment key={20}>{"under "}<strong key={0}>{"Time to restock"}</strong>{" on your grocery list page, where "}<strong key={1}>{"Add"}</strong>{" drops it straight onto the list in the unassigned "}<strong key={2}>{"Other items"}</strong>{" group."}</Fragment>,
        ]} />
        <P>{<Fragment key={21}>{"Suggestions need both low stock "}<em key={0}>{"and"}</em>{" signs your household actually uses the item — a one-off pantry entry you never touch won't pester you. Anything already on your latest list is filtered out, and rejected or still-pending catalog items never appear."}</Fragment>}</P>
      </Section>
      <Section title={"Checking items off matters"}>
        <P>{<Fragment key={22}>{"Grocery check-offs are recorded as engagement events. They're why \"sugar\" can outrank \"flour\" in restock suggestions — your checkout history is direct evidence of what your household burns through fastest."}</Fragment>}</P>
      </Section>
    </DocPage>
  );
}
