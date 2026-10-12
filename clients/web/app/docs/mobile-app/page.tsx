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
  title: "Using the Mobile App — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Using the Mobile App"} intro={"The web app is the recommended way to use LENA for everyday planning and organization — the sidebar sections under \"Web (Recommended)\" cover it. The mobile app is built for the moments you're not at a desk: cooking (cook mode), shopping, scanning barcodes, and standing in front of the pantry — and on tablets it grows into a full two-pane experience. Sign in with the same Google account and you're in the same household — everything syncs."}>
        <P>{<Fragment key={0}>{"The app shares the web client's look: a sage header and navigation bar over a warm cream background, rounded white cards, and the same friendly Nunito typeface. It also follows your device's light/dark setting — or pick your own under "}<strong key={0}>{"More → Appearance"}</strong>{" (Light / System / Dark), the same choices the web offers under "}<strong key={1}>{"Profile → Appearance"}</strong>{" and the header toggle."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-dashboard.png"} alt={"Mobile dashboard"} device="phone" />
        <Screenshot src={"/images/mobile-more-dark.png"} alt={"More → Appearance, and the dashboard in dark mode"} device="phone" />
        <Screenshot src={"/images/mobile-dashboard-dark.png"} alt={"The dashboard in dark mode"} device="phone" />
      <Section title={"The eight tabs"}>
        <P>{<Fragment key={1}>{"Eight tabs is a lot for a phone's bottom bar, so only the tab you're on shows its label — the rest sit there as icons until you need them."}</Fragment>}</P>
        <DocTable head={["Tab","What it's for"]} rows={[[<Fragment key={2}><strong key={0}>{"Home"}</strong></Fragment>, <Fragment key={3}>{"Your day at a glance — a greeting header, "}<strong key={0}>{"Today's meals"}</strong>{", and "}<strong key={1}>{"Delicious ideas for tonight"}</strong>{" → "}<DocLink key={2} href="/docs/mobile-dashboard">{"details"}</DocLink></Fragment>], [<Fragment key={4}><strong key={0}>{"Grocery"}</strong></Fragment>, <Fragment key={5}>{"Your grocery lists — open one and check items off as you shop → "}<DocLink key={0} href="/docs/mobile-grocery">{"details"}</DocLink></Fragment>], [<Fragment key={6}><strong key={0}>{"Events"}</strong></Fragment>, <Fragment key={7}>{"Food events and their cooking timelines → "}<DocLink key={0} href="/docs/mobile-events">{"details"}</DocLink></Fragment>], [<Fragment key={8}><strong key={0}>{"Scan"}</strong></Fragment>, <Fragment key={9}>{"The barcode scanner — add to / remove from pantry stock by UPC → "}<DocLink key={0} href="/docs/mobile-scanning">{"details"}</DocLink></Fragment>], [<Fragment key={10}><strong key={0}>{"Pantry"}</strong></Fragment>, <Fragment key={11}>{"What's in stock: quantities, minimums, notes → "}<DocLink key={0} href="/docs/mobile-pantry">{"details"}</DocLink></Fragment>], [<Fragment key={12}><strong key={0}>{"People"}</strong></Fragment>, <Fragment key={13}>{"Members, invites, and the notification feed → "}<DocLink key={0} href="/docs/mobile-household">{"details"}</DocLink></Fragment>], [<Fragment key={14}><strong key={0}>{"Ask Dot"}</strong></Fragment>, <Fragment key={15}>{"The same \"Ask Dot\" chat as the web — free-form questions about your household's data → "}<DocLink key={0} href="/docs/mobile-ask-dot">{"details"}</DocLink></Fragment>], [<Fragment key={16}><strong key={0}>{"More"}</strong></Fragment>, <Fragment key={17}>{"The planning-and-browsing surfaces: recipes, meal plans, wine cellar, and the item catalog → "}<DocLink key={0} href="/docs/mobile-recipes">{"details"}</DocLink></Fragment>]]} />
        <Screenshot src={"/images/mobile-more.png"} alt={"The More tab"} device="phone" />
        <P>{<Fragment key={18}>{"Two small kindnesses worth knowing about: the camera permission is only asked for when you first open "}<strong key={0}>{"Scan"}</strong>{", never at launch, and forms you didn't type in (like an edit screen that loads a bottle's details) always float their labels above the values instead of drawing through them."}</Fragment>}</P>
        <P>{<Fragment key={19}>{"Allergy warnings travel with you too — recipes, meal plans, grocery lines, and event dishes show the same member-named flags as the web, and you can edit your own records from the People tab. See "}<DocLink key={0} href="/docs/allergy-tracking">{"Allergy Tracking"}</DocLink>{"."}</Fragment>}</P>
      </Section>
      <Section title={"Tablets and landscape"}>
        <P>{<Fragment key={20}>{"Turn the app sideways — or run it on a tablet — and it stops stretching the phone layout. Past ~600dp wide the bottom bar becomes a "}<strong key={0}>{"NavigationRail"}</strong>{"; past ~840dp list screens go "}<strong key={1}>{"two-pane"}</strong>{" with the detail beside the list. See "}<DocLink key={2} href="/docs/mobile-tablet">{"Tablet & Landscape Layouts"}</DocLink>{" for the full tour, including the adaptive meal-plan week grid and tablet cook mode."}</Fragment>}</P>
        <Screenshot src={"/images/tablet-03-grocery-two-pane.png"} alt={"Grocery lists in two-pane mode — the list on the left, an aisle-grouped list on the right"} device="tablet" />
      </Section>
      <Section title={"The item catalog on mobile"}>
        <P>{<Fragment key={21}>{"Under "}<strong key={0}>{"More → Items"}</strong>{" the whole catalog is browsable and editable — item names, brands, categories, UPCs, and the household ingredient override."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-items.png"} alt={"The item catalog list"} device="phone" />
        <Screenshot src={"/images/mobile-edit-item.png"} alt={"Editing an item on mobile"} device="phone" />
      </Section>
      <Section title={"What mobile doesn't do (yet)"}>
        <P>{<Fragment key={22}>{"A few surfaces stay web-first:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={23}><strong key={0}>{"All administration"}</strong>{" — mobile has no admin screens at all. User management, pending-item and recipe-import reviews, and every reference catalog (brands, categories, nutrient types, allergens, the wine tables, and the rest) are admin-only on the web; mobile edits individual items and bottles."}</Fragment>,
          <Fragment key={24}><strong key={0}>{"Event snapshot editing"}</strong>{" — per-step and per-ingredient edits to event copies are on the web event page."}</Fragment>,
        ]} />
        <Callout>{<Fragment key={25}>{"The plan is to keep mobile focused on fast, in-the-moment tasks while the web stays the home for planning and organization — with the gaps filled in over time."}</Fragment>}</Callout>
      </Section>
    </DocPage>
  );
}
