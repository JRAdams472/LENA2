import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  Callout,
  DocLink,
  DocList,
  DocSub,
  P,
  Section,
  Screenshot,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Allergy Tracking — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Allergy Tracking"} intro={"LENA tracks each household member's allergies and dietary restrictions and warns before anyone cooks something risky — on recipe pages, meal plans, grocery lists, and food events, on both web and mobile."}>
      <Section title={"Recording your allergies"}>
        <P>{<Fragment key={0}>{"Every member edits their own records — warnings always name the member so nobody has to guess who a flag is for."}</Fragment>}</P>
        <P>{<Fragment key={1}><strong key={0}>{"On the web"}</strong>{", open "}<strong key={1}>{"Profile"}</strong>{" in the sidebar and find "}<strong key={2}>{"Allergies & dietary restrictions"}</strong>{". Every allergen in the registry gets a tri-state toggle:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={2}><strong key={0}>{"None"}</strong>{" — no record (the default)"}</Fragment>,
          <Fragment key={3}><strong key={0}>{"Dietary"}</strong>{" — a preference or restriction, not a medical allergy (vegetarian-adjacent flags like pork or gelatin live here)"}</Fragment>,
          <Fragment key={4}><strong key={0}>{"Allergy"}</strong>{" — a true allergy; warnings against confirmed allergens escalate to red"}</Fragment>,
        ]} />
        <Screenshot src={"/images/profile-allergies.png"} alt={"Web — the profile allergy editor with tri-state toggles per allergen"} device="web" />
        <DocSub>{<Fragment key={5}>{"The medical & allergen disclaimer"}</Fragment>}</DocSub>
        <P>{<Fragment key={6}>{"The first time you record an allergy or dietary restriction — on web or mobile — LENA asks you to accept the "}<strong key={0}>{"medical & allergen disclaimer"}</strong>{" once. It states plainly that allergen flags are a convenience, not medical advice, that records can be incomplete or out of date, and that the product's physical label is always authoritative. Acceptance is stored once per user and covers every device."}</Fragment>}</P>
        <DocList items={[
          <Fragment key={7}><strong key={0}>{"Recording requires acceptance."}</strong>{" Setting a record to "}<em key={1}>{"Dietary"}</em>{" or "}<em key={2}>{"Allergy"}</em>{" without the disclaimer on file opens the consent prompt; accepting writes your record right away. The server enforces the same rule, so no client can skip it."}</Fragment>,
          <Fragment key={8}><strong key={0}>{"Clearing never requires it."}</strong>{" Switching back to "}<em key={1}>{"None"}</em>{" is always allowed, disclaimer or not."}</Fragment>,
          <Fragment key={9}><strong key={0}>{"Warnings keep working either way."}</strong>{" Flags you've already recorded — and warnings other members see — are never gated."}</Fragment>,
        ]} />
        <Screenshot src={"/images/disclaimer-consent.png"} alt={"Web — the one-time disclaimer consent prompt on the profile page"} device="web" />
        <P>{<Fragment key={10}>{"The full text lives on the public "}<DocLink key={0} href="https://lenaskitchen.app/disclaimer" external>{"disclaimer page"}</DocLink>{" (linked from the site footer and the mobile caption lines). Recipe pages and mobile cook mode also carry a short "}<em key={1}>{"\"always check the product's label\""}</em>{" caption near the allergen flags — see "}<DocLink key={2} href="/docs/mobile-allergies">{"Mobile Allergy Tracking"}</DocLink>{" for the phone version."}</Fragment>}</P>
        <P>{<Fragment key={11}><strong key={0}>{"On mobile"}</strong>{", the same tri-state editor lives under the "}<strong key={1}>{"People"}</strong>{" tab — see "}<DocLink key={2} href="/docs/mobile-allergies">{"Allergy Tracking on Mobile"}</DocLink>{"."}</Fragment>}</P>
      </Section>
      <Section title={"Where warnings appear"}>
        <P>{<Fragment key={12}>{"When a recipe, meal slot, grocery line, or event dish conflicts with someone's records, LENA says so inline:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={13}><strong key={0}>{"Recipe detail"}</strong>{" — an alert banner naming the member, the allergen, and the kind of conflict ("}<em key={1}>{"E2E User — milk (allergy; contains)"}</em>{"). Confirmed-allergen + "}<code key={2}>{"contains"}</code>{" conflicts are red; dietary restrictions and "}<code key={3}>{"may_contain"}</code>{" flags stay amber."}</Fragment>,
          <Fragment key={14}><strong key={0}>{"Lists and cards"}</strong>{" — meal plans, grocery lists, and event pages show a small \"N allergy warnings\" chip; hover (web) or tap (mobile) for the member-by-member breakdown."}</Fragment>,
        ]} />
        <P>{<Fragment key={15}><strong key={0}>{"On the web"}</strong>{", the banner sits at the top of the recipe page:"}</Fragment>}</P>
        <Screenshot src={"/images/recipe-allergy-warning.png"} alt={"Web — a recipe flagged for a member's milk allergy, with its allergen tags"} device="web" />
        <P>{<Fragment key={16}><strong key={0}>{"On mobile"}</strong>{", warning icons ride along on grocery lines, meal-plan slots, recipe pages, and event dishes — see "}<DocLink key={1} href="/docs/mobile-allergies">{"Allergy Tracking on Mobile"}</DocLink>{"."}</Fragment>}</P>
        <P>{<Fragment key={17}>{"Warnings are advisory — LENA flags the conflict and lets your household decide; nothing is blocked or hidden."}</Fragment>}</P>
        <Callout>{<Fragment key={18}><strong key={0}>{"\"No allergen information\" is not \"no allergens.\""}</strong>{" Flag coverage is curated and still growing, so an entity that shows "}<em key={1}>{"No allergen information"}</em>{" simply has no data yet — it is never a statement that the food is safe."}</Fragment>}</Callout>
      </Section>
      <Section title={"Curating allergen data (web, admins)"}>
        <P>{<Fragment key={19}>{"Warnings come from flags on the catalog, managed on the web:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={20}><strong key={0}>{"The registry"}</strong>{" — "}<strong key={1}>{"Inventory → Allergens"}</strong>{" lists the taxonomy (milk, eggs, peanuts, tree nuts, wheat, gluten, soy, sesame, fish, shellfish, sulfites, corn, dietary entries like pork/beef/gelatin, and more). Admins can add, rename, or deactivate entries; an inactive allergen stops producing warnings everywhere at once."}</Fragment>,
          <Fragment key={21}><strong key={0}>{"Flags"}</strong>{" — item and ingredient editors carry "}<strong key={1}>{"contains"}</strong>{" / "}<strong key={2}>{"may contain"}</strong>{" flag controls. "}<code key={3}>{"contains"}</code>{" means the food has the allergen; "}<code key={4}>{"may contain"}</code>{" is advisory (shared equipment, processing risk). Ingredient flags flow to every recipe line that references them — and a household's "}<DocLink key={5} href="/docs/ingredients">{"ingredient override"}</DocLink>{" wins over the catalog mapping, so flagging stays correct even when your household re-maps a product."}</Fragment>,
        ]} />
        <Screenshot src={"/images/allergen-registry.png"} alt={"Web — the allergen registry, the taxonomy every warning draws from"} device="web" />
      </Section>
      <Section title={"AI-suggested flags (web, admins)"}>
        <P>{<Fragment key={22}>{"Flagging a whole catalog by hand is tedious, so the assistant can propose flags for you — "}<strong key={0}>{"never applying them automatically"}</strong>{". On "}<strong key={1}>{"Inventory → Allergen Suggestions"}</strong>{":"}</Fragment>}</P>
        <DocList ordered items={[
          <Fragment key={23}>{"Pick a recipe and hit "}<strong key={0}>{"Suggest flags"}</strong>{" — the assistant reads its ingredients and proposes target + allergen + kind ("}<code key={1}>{"contains"}</code>{"/"}<code key={2}>{"may_contain"}</code>{") with a short rationale."}</Fragment>,
          <Fragment key={24}>{"Proposals land in the "}<strong key={0}>{"review queue"}</strong>{", filtered by pending / accepted / dismissed. Re-running the suggester can't stack duplicates — one open proposal per target + allergen."}</Fragment>,
          <Fragment key={25}><strong key={0}>{"Accept"}</strong>{" writes the real flag under your name; "}<strong key={1}>{"Dismiss"}</strong>{" records the rejection for the audit trail. Either way the row keeps its provenance (which recipe produced it, who reviewed it, when)."}</Fragment>,
        ]} />
        <Screenshot src={"/images/allergen-suggestions.png"} alt={"Web — the allergen suggestion review queue with pending proposals"} device="web" />
        <P>{<Fragment key={26}>{"Because every proposal needs a human accept, a wrong guess (and small local models do guess) never becomes a false warning — it just sits in the queue until you dismiss it."}</Fragment>}</P>
        <P>{<Fragment key={27}>{"← Back to "}<DocLink key={0} href="/docs">{"Home"}</DocLink></Fragment>}</P>
      </Section>
    </DocPage>
  );
}
