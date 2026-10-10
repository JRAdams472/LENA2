import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  DocLink,
  P,
  Section,
  Screenshot,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Wine Cellar on Mobile — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Wine Cellar on Mobile"} intro={"Your cellar travels with you — the mobile app covers the day-to-day bottle work while the web stays home for the reference catalogs. Find it under More → Wine."}>
        <Screenshot src={"/images/mobile-wine.png"} alt={"Mobile — the wine cellar screen"} device="phone" />
      <Section title={"Bottles"}>
        <P>{<Fragment key={0}>{"The "}<strong key={0}>{"Bottles"}</strong>{" list shows your cellar with quantities on hand — the same data as the web's Wine → Bottles page."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-bottles.png"} alt={"Mobile — the bottles list with quantities"} device="phone" />
        <P>{<Fragment key={1}>{"Tap a bottle to "}<strong key={0}>{"edit"}</strong>{" it — identity, character, and logistics fields all editable on the phone:"}</Fragment>}</P>
        <Screenshot src={"/images/mobile-edit-bottle.png"} alt={"Mobile — editing a bottle's details"} device="phone" />
      </Section>
      <Section title={"Adjusting quantities"}>
        <P>{<Fragment key={2}>{"Pulled a bottle for dinner? The "}<strong key={0}>{"Adjust"}</strong>{" button decrements the count right there in the cellar — no walking back to a computer."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-adjust-bottle.png"} alt={"Mobile — adjusting a bottle's on-hand count"} device="phone" />
        <P>{<Fragment key={3}>{"The Adjust form's bottle field opens a "}<strong key={0}>{"searchable bottom sheet"}</strong>{" instead of a long dropdown, so a large catalog stays usable:"}</Fragment>}</P>
        <Screenshot src={"/images/mobile-bottle-picker.png"} alt={"Mobile — the bottle picker: server-side search in a bottom sheet"} device="phone" />
      </Section>
      <Section title={"Web-only"}>
        <P>{<Fragment key={4}>{"The "}<strong key={0}>{"reference catalogs"}</strong>{" (types, countries, regions, vintages, grape varieties, flavor profiles) are admin-managed on the web — see "}<DocLink key={1} href="/docs/wine-cellar">{"Wine Cellar"}</DocLink>{". Mobile edits individual bottles against that shared vocabulary."}</Fragment>}</P>
        <P>{<Fragment key={5}>{"← Back to "}<DocLink key={0} href="/docs/mobile-app">{"Using the Mobile App"}</DocLink>{" · See also "}<DocLink key={1} href="/docs/wine-cellar">{"Wine Cellar (web)"}</DocLink></Fragment>}</P>
      </Section>
    </DocPage>
  );
}
