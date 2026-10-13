import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  Callout,
  DocLink,
  DocList,
  DocTable,
  P,
  Section,
  Screenshot,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Recipes — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Recipes"} intro={"Recipes are the heart of LENA — they feed meal plans, grocery lists, events, and recommendations."}>
      <Section title={"Browsing and searching"}>
        <P>{<Fragment key={0}>{"Open "}<strong key={0}>{"Recipes → Recipes"}</strong>{" to see the household's collection. You can:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={1}><strong key={0}>{"Search"}</strong>{" by name (the list narrows as you type)"}</Fragment>,
          <Fragment key={2}><strong key={0}>{"Semantic search"}</strong>{" (✨ "}<strong key={1}>{"Semantic"}</strong>{" switch, shown when the server has embeddings configured) — describe what you're in the mood for instead of matching names: \"cozy comfort dinner\", \"something fresh with citrus\", \"quick weeknight pasta\""}</Fragment>,
          <Fragment key={3}><strong key={0}>{"Filter by category"}</strong>{" — one dropdown per group (Course, Dish Type, Dietary, Difficulty, Main Ingredient, Cuisine). Picks combine across groups, so \"Italian\" + \"Easy\" finds easy Italian recipes; picking two options in one group matches either"}</Fragment>,
          <Fragment key={4}><strong key={0}>{"Filter to favorites"}</strong>{" with the Favorite toggle"}</Fragment>,
          <Fragment key={5}><strong key={0}>{"Peek at a recipe"}</strong>{" — click its name or description to open a read-only preview: the ingredient list (grouped by section, optional lines marked) and the numbered steps with their durations, without leaving the table. Use the row's edit pencil or "}<strong key={1}>{"Manage"}</strong>{" link for the full recipe page with ratings, categories, and tweaks"}</Fragment>,
        ]} />
        <Screenshot src={"/images/recipe-preview.png"} alt={"A recipe preview — ingredients and steps without opening the editor"} device="web" />
        <Screenshot src={"/images/recipes.png"} alt={"The recipe collection — search, filters, and the recipe table"} device="web" />
        <Screenshot src={"/images/recipe-category-filter.png"} alt={"The recipe collection, with category filters"} device="web" />
        <Screenshot src={"/images/recipes-semantic.png"} alt={"Semantic search in action"} device="web" />
        <Callout>{<Fragment key={6}><strong key={0}>{"Results are ranked for you."}</strong>{" Keyword mode puts favorites first, then recipes your household has actually cooked (from meal plans and events), then ones you've opened before, then ones matching things you've searched for. Semantic mode ranks by how close each recipe's "}<em key={1}>{"meaning"}</em>{" is to your description — name, description, ingredients, and categories all count — with a small nudge for favorites and household staples. The more you use LENA, the closer the top of the list gets to \"what you were probably going to pick anyway\".  Semantic mode runs on a small local embedding model (Ollama's "}<code key={2}>{"nomic-embed-text"}</code>{" by default). If your server doesn't have it configured, the switch simply doesn't appear and everything else works the same. One honest limitation: embeddings don't understand "}<em key={3}>{"negation"}</em>{" — \"pasta with no cheese\" may still surface cheesy recipes, so double-check the ingredients."}</Fragment>}</Callout>
      </Section>
      <Section title={"Categories"}>
        <P>{<Fragment key={7}>{"Categories are how you organize the collection. They come in "}<strong key={0}>{"groups"}</strong>{", each with its own rules:"}</Fragment>}</P>
        <DocTable head={["Group","Pick","Examples"]} rows={[[<Fragment key={8}>{"Course"}</Fragment>, <Fragment key={9}>{"one"}</Fragment>, <Fragment key={10}>{"Breakfast, Lunch, Dinner, Dessert, Snack, Side"}</Fragment>], [<Fragment key={11}>{"Dish Type"}</Fragment>, <Fragment key={12}>{"many"}</Fragment>, <Fragment key={13}>{"Soup, Bread, Casserole, Single Pan, Cocktail"}</Fragment>], [<Fragment key={14}>{"Dietary"}</Fragment>, <Fragment key={15}>{"many"}</Fragment>, <Fragment key={16}>{"Low Calorie (admins can add more — gluten-free, etc.)"}</Fragment>], [<Fragment key={17}>{"Difficulty"}</Fragment>, <Fragment key={18}>{"one"}</Fragment>, <Fragment key={19}>{"Easy, Medium, Skilled"}</Fragment>], [<Fragment key={20}>{"Main Ingredient"}</Fragment>, <Fragment key={21}>{"one"}</Fragment>, <Fragment key={22}>{"Chicken, Beef, Fish, Pork, Vegetarian"}</Fragment>], [<Fragment key={23}>{"Cuisine"}</Fragment>, <Fragment key={24}>{"one"}</Fragment>, <Fragment key={25}>{"Italian, Mexican, French, American, Asian…"}</Fragment>]]} />
        <P>{<Fragment key={26}>{"Groups marked "}<em key={0}>{"pick one"}</em>{" are exclusive — a recipe can't be both Mexican and Italian, for example. Groups marked "}<em key={1}>{"many"}</em>{" allow several categories at once."}</Fragment>}</P>
        <P>{<Fragment key={27}><strong key={0}>{"Assigning categories:"}</strong>{" open a recipe and scroll to "}<strong key={1}>{"Categories"}</strong>{". Radio buttons appear under exclusive groups, checkboxes under the rest — just tap to apply; there's no separate save step. Any household member can categorize recipes."}</Fragment>}</P>
        <Screenshot src={"/images/recipe-detail.png"} alt={"Assigning categories on a recipe"} device="web" />
        <P>{<Fragment key={28}><strong key={0}>{"Curating the taxonomy (admins):"}</strong>{" "}<strong key={1}>{"Recipes → Categories"}</strong>{" lists every group with its exclusivity and display order, and lets you add, rename, reorder, or delete groups and categories."}</Fragment>}</P>
        <Screenshot src={"/images/recipe-categories-admin.png"} alt={"The admin Recipe Categories page — groups with exclusivity and counts"} device="web" />
      </Section>
      <Section title={"Allergy warnings"}>
        <P>{<Fragment key={29}>{"If a recipe's ingredients conflict with a household member's "}<DocLink key={0} href="/docs/allergy-tracking">{"allergy or dietary records"}</DocLink>{", the recipe page shows an alert naming the member and the allergen — a red banner for a confirmed allergy against a "}<code key={1}>{"contains"}</code>{" flag, amber for dietary restrictions and "}<code key={2}>{"may contain"}</code>{". The line's allergen tags are listed just underneath."}</Fragment>}</P>
        <Screenshot src={"/images/recipe-allergy-warning.png"} alt={"A recipe flagged for a member's milk allergy"} device="web" />
        <P>{<Fragment key={30}>{"Warnings are advisory — they name the conflict so your household can decide, they don't block the recipe."}</Fragment>}</P>
      </Section>
      <Section title={"Rating recipes"}>
        <P>{<Fragment key={31}>{"On a recipe's page, tap the stars under "}<strong key={0}>{"Your rating"}</strong>{". LENA shows the household's average rating and how many people rated — ratings feed the \"Due for a revisit\" and \"Recommended for you\" suggestions on your Dashboard, so it's worth the two seconds."}</Fragment>}</P>
      </Section>
      <Section title={"Creating and editing a recipe"}>
        <P>{<Fragment key={32}>{"Open a recipe to edit it, or create a new one. A recipe has:"}</Fragment>}</P>
        <Screenshot src={"/images/recipe-detail.png"} alt={"A recipe page — servings, rating, and the ingredient table"} device="web" />
        <Screenshot src={"/images/recipe-detail-ingredients.png"} alt={"The ingredients section — ingredient picker, preferred brand, and mixed lines"} device="web" />
        <DocList items={[
          <Fragment key={33}><strong key={0}>{"Name"}</strong>{", description, and "}<strong key={1}>{"servings"}</strong></Fragment>,
          <Fragment key={34}><strong key={0}>{"Ingredients"}</strong>{" — each with a generic "}<DocLink key={1} href="/docs/ingredients">{"ingredient"}</DocLink>{" (search or create inline), an optional "}<strong key={2}>{"preferred brand"}</strong>{", quantity, and unit of measure. Classic item-only lines still work; ingredient-keyed lines let recipes say \"corn\" without pinning a brand"}</Fragment>,
          <Fragment key={35}><strong key={0}>{"Steps"}</strong>{" — numbered instructions, plus optional "}<strong key={1}>{"timing metadata"}</strong>{" per step:"}</Fragment>,
          <Fragment key={36}><strong key={0}>{"Duration"}</strong>{" (minutes)"}</Fragment>,
          <Fragment key={37}><strong key={0}>{"Step type"}</strong>{" (e.g. prep, cook)"}</Fragment>,
          <Fragment key={38}><strong key={0}>{"Passive"}</strong>{" — check this for \"walk away\" steps like simmering, rising, or marinating"}</Fragment>,
          <Fragment key={39}><strong key={0}>{"Depends on"}</strong>{" — which earlier step must finish first (defaults to the previous step; override it to describe steps that can run in parallel)"}</Fragment>,
          <Fragment key={40}><strong key={0}>{"Appliance"}</strong>{" — oven, stove, mixer, etc. LENA uses this to spot scheduling conflicts in event timelines"}</Fragment>,
        ]} />
        <Callout>{<Fragment key={41}>{"You only need timing metadata if you want the "}<DocLink key={0} href="/docs/food-events">{"event timeline"}</DocLink>{" to schedule that recipe. Plain recipes work fine without it — LENA estimates a single-slot duration for unscheduled steps."}</Fragment>}</Callout>
      </Section>
      <Section title={"Household tweaks"}>
        <P>{<Fragment key={42}>{"Recipes are shared, but tastes aren't. Any household member can "}<strong key={0}>{"tweak"}</strong>{" a recipe for the household — the original recipe stays untouched, and every view that household sees (the recipe page, meal plans, grocery lists, servings scaling, event snapshots, and Ask Dot's suggestions) uses the tweaked version."}</Fragment>}</P>
        <Screenshot src={"/images/recipe-tweaks.png"} alt={"The Household tweaks panel — swap, adjust, remove, and add for lines and steps"} device="web" />
        <P>{<Fragment key={43}>{"On a recipe page, each ingredient line and step has a "}<strong key={0}>{"Tweak"}</strong>{" button:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={44}><strong key={0}>{"Swap"}</strong>{" — replace a line's ingredient or preferred brand (e.g. garlic → shallot)"}</Fragment>,
          <Fragment key={45}><strong key={0}>{"Adjust"}</strong>{" — change the quantity or unit"}</Fragment>,
          <Fragment key={46}><strong key={0}>{"Remove"}</strong>{" — drop the line or step entirely"}</Fragment>,
          <Fragment key={47}><strong key={0}>{"Add"}</strong>{" — new lines and steps are drafted in the tweaks panel itself"}</Fragment>,
        ]} />
        <P>{<Fragment key={48}>{"Tweaked rows carry small badges ("}<strong key={0}>{"Swapped"}</strong>{", "}<strong key={1}>{"Adjusted"}</strong>{", "}<strong key={2}>{"Added"}</strong>{", "}<strong key={3}>{"Replaced"}</strong>{") so it's always clear what isn't original, and the panel lists every pending change until you save or discard it. "}<strong key={4}>{"Household version"}</strong>{" / "}<strong key={5}>{"Original recipe"}</strong>{" at the top of the page flips between the tweaked and untouched recipe."}</Fragment>}</P>
        <Screenshot src={"/images/recipe-delta-household.png"} alt={"The household (tweaked) view with delta badges"} device="web" />
        <Screenshot src={"/images/recipe-delta-original.png"} alt={"The original view — canonical lines with no badges"} device="web" />
        <Screenshot src={"/images/recipe-delta-tweak-editor.png"} alt={"The tweak editor on a line"} device="web" />
        <P>{<Fragment key={49}>{"If the original recipe is edited after your household made tweaks, a banner flags the drift — "}<strong key={0}>{"Mark reviewed"}</strong>{" dismisses it once you've checked the changes still make sense. Tweaks whose original lines were deleted don't vanish silently; they show as \"no longer applies\" chips you can remove at your leisure."}</Fragment>}</P>
        <Screenshot src={"/images/recipe-delta-stale.png"} alt={"The stale-tweaks drift banner"} device="web" />
        <P>{<Fragment key={50}>{"Every tweak action is recorded in "}<strong key={0}>{"Tweak history"}</strong>{" at the bottom of the panel — who saved the tweak set (and how many lines and steps it contained), who cleared the tweaks, and who marked them reviewed, newest first:"}</Fragment>}</P>
        <Screenshot src={"/images/recipe-tweak-history.png"} alt={"The Tweak history section — actor, action, and timestamp for every tweak change"} device="web" />
        <Screenshot src={"/images/recipe-detail-dark.png"} alt={"The recipe detail page in dark mode"} device="web" />
        <Callout>{<Fragment key={51}>{"Canonical recipe editing (the underlying ingredients and steps everyone shares) stays admin-only — tweaks are the member-level editing path."}</Fragment>}</Callout>
      </Section>
      <Section title={"Importing a recipe from a photo (OCR)"}>
        <P>{<Fragment key={52}>{"Have a handwritten card, a cookbook page, or a photo of a recipe? On the web Recipes page, use "}<strong key={0}>{"Upload Recipe Scan"}</strong>{":"}</Fragment>}</P>
        <DocList ordered items={[
          <Fragment key={53}>{"Choose the image file."}</Fragment>,
          <Fragment key={54}>{"LENA queues it for OCR processing."}</Fragment>,
          <Fragment key={55}>{"The import appears under "}<strong key={0}>{"Recipes → Pending Reviews"}</strong>{" while it works its way through processing (OCR → draft → review)."}</Fragment>,
          <Fragment key={56}>{"Once processed, review the extracted recipe, fix anything the OCR got wrong, and save it to your collection."}</Fragment>,
        ]} />
        <P>{<Fragment key={57}>{"Status chips tell you where each import stands: "}<strong key={0}>{"Processing"}</strong>{" (working), "}<strong key={1}>{"Ready"}</strong>{" (waiting for you), "}<strong key={2}>{"Reviewing"}</strong>{", "}<strong key={3}>{"Failed"}</strong>{", or "}<strong key={4}>{"Profanity"}</strong>{" (flagged by the content filter — edit and resubmit)."}</Fragment>}</P>
        <Callout>{<Fragment key={58}>{"Tip: clear, straight-on photos with good light import best. One recipe per image."}</Fragment>}</Callout>
      </Section>
      <Section title={"Reviewing submitted recipes (admins)"}>
        <P>{<Fragment key={59}>{"Recipes submitted by members land on "}<strong key={0}>{"Pending Recipes"}</strong>{" (admin-only, under "}<strong key={1}>{"Administration"}</strong>{") for review before joining the shared collection — same review-queue pattern as "}<DocLink key={2} href="/docs/pantry#approving-new-items-admins">{"pending items"}</DocLink>{"."}</Fragment>}</P>
        <Screenshot src={"/images/recipes-pending.png"} alt={"The Pending Recipes review queue"} device="web" />
      </Section>
      <Section title={"Recipes belong to the household"}>
        <P>{<Fragment key={60}>{"Everyone in the household sees the same recipe collection — a favorite for one person is visible to all, and "}<DocLink key={0} href="/docs/#household-tweaks">{"household tweaks"}</DocLink>{" apply to every member's view, plans, and lists. If a recipe is attached to a "}<DocLink key={1} href="/docs/food-events">{"food event"}</DocLink>{", the event keeps its own copy, so editing the shared recipe never surprises a planned dinner."}</Fragment>}</P>
      </Section>
    </DocPage>
  );
}
