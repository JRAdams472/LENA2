import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  DocLink,
  DocList,
  P,
  Section,
  Screenshot,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Mobile: Recipes and Meal Plans — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Mobile: Recipes and Meal Plans"} intro={"Recipes, meal plans, the wine cellar, and the item catalog live under the More tab — the planning-and-browsing surfaces that don't fit the quick-action tabs. You can also reach them from Home (suggestion cards, \"Plan it →\") and from links throughout the app."}>
      <Section title={"Finding recipes"}>
        <Screenshot src={"/images/mobile-recipes.png"} alt={"The recipes list with search and filters"} device="phone" />
        <Screenshot src={"/images/tablet-09-recipes.png"} alt={"The recipes list on a tablet — search and filters across the full pane"} device="tablet" />
        <P>{<Fragment key={0}>{"The Recipes screen has the same search + filter powers as the web:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={1}><strong key={0}>{"Search"}</strong>{" by name in the field at the top — results narrow as you type (pausing records the search, which helps rank future results)"}</Fragment>,
          <Fragment key={2}><strong key={0}>{"Star toggle"}</strong>{" in the top bar limits the list to favorites"}</Fragment>,
          <Fragment key={3}><strong key={0}>{"Filter icon"}</strong>{" (with a count badge) opens a category sheet — tap chips to filter; exclusive groups like Course and Cuisine pick one, Dish Type and Dietary can pick several"}</Fragment>,
        ]} />
        <P>{<Fragment key={4}>{"Tapping a recipe opens a read-only view — its name, the ingredient list, and the numbered steps — with an "}<strong key={0}>{"Edit"}</strong>{" action in the app bar for the full editor. Going back lands on the list exactly as you left it: search text, filters, and scroll position are all preserved. The same view opens when you tap a recipe suggestion on the dashboard."}</Fragment>}</P>
        <P>{<Fragment key={5}>{"On the recipe view or editor, scroll to "}<strong key={0}>{"Categories"}</strong>{" to assign them with the same tap-to-apply chips. You can edit a recipe in place — name, description, ingredients, steps, and times."}</Fragment>}</P>
        <P>{<Fragment key={6}>{"The recipe editor's add-row is "}<DocLink key={0} href="/docs/ingredients">{"ingredient-first"}</DocLink>{": the "}<strong key={1}>{"Ingredient"}</strong>{" picker searches generic ingredients (with inline create), and "}<strong key={2}>{"Preferred brand"}</strong>{" underneath is optional. Existing lines display the ingredient name with the brand after a dash. If the recipe conflicts with a member's allergy records, a warning banner and allergen chips sit at the top of the editor."}</Fragment>}</P>
        <P>{<Fragment key={7}>{"On a tablet the recipe list and the editor sit side by side:"}</Fragment>}</P>
        <Screenshot src={"/images/tablet-10-recipe-two-pane.png"} alt={"Recipes two-pane — the list on the left, the recipe editor with allergen banner and tweak badges on the right"} device="tablet" />
      </Section>
      <Section title={"Cook mode"}>
        <P>{<Fragment key={8}>{"The chef's-hat button in the recipe editor's top bar opens "}<strong key={0}>{"cook mode"}</strong>{" — a full-screen step pager built for propped-up-on-the-counter use, with a keep-awake display, timed-step countdowns, and ingredients in a side rail (tablet) or bottom sheet (phone). See "}<DocLink key={1} href="/docs/mobile-cook-mode">{"Cook Mode"}</DocLink>{" for the full walkthrough."}</Fragment>}</P>
        <Screenshot src={"/images/tablet-11-cook-mode.png"} alt={"Cook mode on a tablet — the ingredients rail on the left, the step pager on the right"} device="tablet" />
        <P>{<Fragment key={9}>{"The recipe page shows your household's "}<DocLink key={0} href="/docs/recipes#household-tweaks">{"tweaks"}</DocLink>{" by default — swapped, adjusted, removed, or added lines and steps carry badges, and a "}<strong key={1}>{"Household / Original"}</strong>{" switch flips to the untouched recipe. A "}<strong key={2}>{"Household tweaks"}</strong>{" card lists the pending changes; per-line and per-step tweak sheets cover swap, adjust, remove, and add, then "}<strong key={3}>{"Save"}</strong>{" writes the whole set at once. When the original recipe changes underneath, a banner offers "}<strong key={4}>{"Mark reviewed"}</strong>{" to acknowledge the drift."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-recipe-tweaks-stale.png"} alt={"The stale-tweaks banner over the household view — delta badges on Adjusted, Swapped, Added, and Replaced lines"} device="phone" />
        <Screenshot src={"/images/mobile-recipe-tweaks-reviewed.png"} alt={"The same recipe after tapping Mark reviewed — the banner clears and the tweaked household view stays"} device="phone" />
        <Screenshot src={"/images/mobile-recipe-tweaks-original.png"} alt={"The Original view — canonical lines without badges, with the household tweaks listed below"} device="phone" />
        <Screenshot src={"/images/mobile-recipe-tweak-sheet.png"} alt={"The per-line tweak sheet — swap, adjust, or remove an ingredient row"} device="phone" />
        <Screenshot src={"/images/mobile-recipe-tweaks-card.png"} alt={"The Household tweaks card — every pending change with add ingredient and add step actions"} device="phone" />
        <Screenshot src={"/images/mobile-recipe-editor.png"} alt={"The mobile recipe editor — floating labels, allergen banner, and ingredient rows"} device="phone" />
        <Screenshot src={"/images/mobile-ingredient-picker.png"} alt={"The mobile recipe editor's ingredient picker"} device="phone" />
        <Screenshot src={"/images/mobile-recipe-filter-sheet.png"} alt={"The filter sheet — search and category filters"} device="phone" />
        <Screenshot src={"/images/mobile-new-recipe.png"} alt={"Creating a recipe on mobile"} device="phone" />
      </Section>
      <Section title={"Meal plans"}>
        <P>{<Fragment key={10}>{"The Meal Plans screen shows the household's plans — each row is a card with the plan's name and its localized week. Opening a plan lands on the "}<strong key={0}>{"week view"}</strong>{": a 7-day × Breakfast/Lunch/Dinner grid on wide panes, or scrolling day sections on a phone. The grid sizes its columns to the pane it lands in — on a tablet's two-pane detail view all seven days fit at once (columns adapt down to a 110dp floor; below that the grid scrolls horizontally). Each occupied cell shows the recipe (and any free-form items) with its servings; tap an empty "}<code key={1}>{"+"}</code>{" to add a slot — a weekday is implied by the cell, so you only pick a meal type and an optional recipe. Tapping a filled cell offers edit (remove + re-add) or remove. Meal types outside the three standard ones group under "}<strong key={2}>{"Other"}</strong>{"."}</Fragment>}</P>
        <Screenshot src={"/images/tablet-06-meal-plan-week.png"} alt={"The meal-plan week grid — seven days by three meal types, recipe cells with servings and allergen badges"} device="tablet" />
        <Screenshot src={"/images/tablet-05-meal-plans.png"} alt={"Meal plans in two-pane mode — plans on the left, the week grid on the right"} device="tablet" />
        <P>{<Fragment key={11}>{"The week view's top bar carries two extras:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={12}><strong key={0}>{"Suggest meals"}</strong>{" (sparkles) asks the server's AI for ideas — each card shows the recipe, the proposed day and meal type, the reason, and which expiring pantry items it would use up. "}<strong key={1}>{"Add"}</strong>{" drops it straight into the grid."}</Fragment>,
          <Fragment key={13}><strong key={0}>{"Nutrition"}</strong>{" (chart icon) renders the plan's aggregate nutrients and warnings — including lines skipped because an ingredient isn't linked to an item yet."}</Fragment>,
        ]} />
        <Screenshot src={"/images/tablet-07-suggest-meals.png"} alt={"The Suggest meals sheet — proposed slot, reason, and expiring-item reuse"} device="tablet" />
        <Screenshot src={"/images/tablet-08-nutrition.png"} alt={"The Nutrition sheet — aggregate nutrients and unlinked-ingredient warnings"} device="tablet" />
        <Screenshot src={"/images/mobile-meal-plans.png"} alt={"Mobile meal plans"} device="phone" />
        <Screenshot src={"/images/mobile-recipe-picker.png"} alt={"The meal-plan recipe picker — server-side search with a None row"} device="phone" />
        <Screenshot src={"/images/mobile-edit-meal-plan.png"} alt={"Editing a meal plan's days and slots"} device="phone" />
        <P>{<Fragment key={14}>{"The dashboard's "}<strong key={0}>{"Today's meals"}</strong>{" card and its "}<strong key={1}>{"Plan it →"}</strong>{" link land here too."}</Fragment>}</P>
      </Section>
      <Section title={"Wine"}>
        <P>{<Fragment key={15}>{"The cellar lives under "}<strong key={0}>{"More → Wine"}</strong>{" — browse bottles and adjust quantities from the pantry floor. See "}<DocLink key={1} href="/docs/mobile-wine">{"Wine Cellar on Mobile"}</DocLink>{" for the bottles list, the searchable bottle picker, and the Adjust form."}</Fragment>}</P>
        <P>{<Fragment key={16}>{"← Back to "}<DocLink key={0} href="/docs/mobile-app">{"Using the Mobile App"}</DocLink></Fragment>}</P>
      </Section>
    </DocPage>
  );
}
