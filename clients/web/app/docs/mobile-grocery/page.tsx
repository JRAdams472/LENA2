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
  title: "Grocery Lists on Mobile — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Grocery Lists on Mobile"} intro={"The Grocery tab lists your household's grocery lists — tap one to open it and shop."}>
        <Screenshot src={"/images/mobile-grocery.png"} alt={"Grocery lists"} device="phone" />
        <Screenshot src={"/images/mobile-grocery-list.png"} alt={"An opened list grouped by store aisle"} device="phone" />
      <Section title={"Shopping with a list"}>
        <DocList items={[
          <Fragment key={0}>{"Items are grouped into "}<strong key={0}>{"store route groups"}</strong>{" — the same aisle order the web shows, computed server-side from your household's store layout and learned check-off order. The phone renders the server's groups verbatim, so web and mobile always agree."}</Fragment>,
          <Fragment key={1}><strong key={0}>{"Check items off"}</strong>{" as you shop — the check order feeds back into the route learning for next time."}</Fragment>,
          <Fragment key={2}><strong key={0}>{"Drag to reorder"}</strong>{" within an aisle group; "}<strong key={1}>{"move to aisle"}</strong>{" from an item's menu when it landed in the wrong group — including moving an item back to the unassigned "}<strong key={2}>{"Other items"}</strong>{" group."}</Fragment>,
          <Fragment key={3}>{"Pull down to refresh; use the store picker on the detail screen if your household has more than one store layout."}</Fragment>,
          <Fragment key={4}><strong key={0}>{"Shop with Instacart"}</strong>{" — when the server has a provider configured, a cart action in the app bar pushes the unchecked lines to Instacart and shows a link dialog: open it externally to finish on Instacart, or copy it to share. Same provider gating as the web — no configured key, no button."}</Fragment>,
        ]} />
        <Screenshot src={"/images/mobile-grocery-store-picker.png"} alt={"Aisle-grouped list with the store picker open"} device="phone" />
        <Screenshot src={"/images/mobile-grocery-move-aisle.png"} alt={"The per-item Move to aisle menu"} device="phone" />
        <Screenshot src={"/images/mobile-generate-grocery.png"} alt={"Generating a list from the meal plan"} device="phone" />
        <Screenshot src={"/images/mobile-grocery-list-dark.png"} alt={"The aisle-grouped list in dark mode"} device="phone" />
        <Screenshot src={"/images/mobile-grocery-dark.png"} alt={"The lists index in dark mode"} device="phone" />
      </Section>
      <Section title={"Ingredients and usual brands"}>
        <P>{<Fragment key={5}>{"Ingredient-keyed lines show the generic "}<DocLink key={0} href="/docs/ingredients">{"ingredient"}</DocLink>{" name, with a \"usually buys …\" caption once your household has a remembered brand. The first check-off of an ingredient-only line opens the "}<strong key={1}>{"brand picker"}</strong>{" — pick what you bought and it's credited to the pantry "}<em key={2}>{"and"}</em>{" remembered as the usual for next time."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-brand-picker.png"} alt={"The brand picker on first check-off"} device="phone" />
        <Screenshot src={"/images/mobile-usual-brand.png"} alt={"Checked ingredient lines showing their usual brands"} device="phone" />
        <P>{<Fragment key={6}>{"Lines that conflict with a member's "}<DocLink key={0} href="/docs/allergy-tracking">{"allergy records"}</DocLink>{" show a labeled allergy badge — a warning icon plus a count. Tapping it opens a dialog naming the member and the allergen."}</Fragment>}</P>
        <P>{<Fragment key={7}>{"See "}<DocLink key={0} href="/docs/grocery-lists">{"Grocery Lists"}</DocLink>{" for how lists are generated and how routing works."}</Fragment>}</P>
        <P>{<Fragment key={8}>{"← Back to "}<DocLink key={0} href="/docs/mobile-app">{"Using the Mobile App"}</DocLink></Fragment>}</P>
      </Section>
    </DocPage>
  );
}
