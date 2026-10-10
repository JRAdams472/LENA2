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
  title: "Allergy Tracking on Mobile — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Allergy Tracking on Mobile"} intro={"The same member allergy records and warnings live on the phone — see Allergy Tracking for how records, flags, and warnings work overall."}>
      <Section title={"Editing your records"}>
        <P>{<Fragment key={0}>{"Open the "}<strong key={0}>{"People"}</strong>{" tab — the same tri-state toggles the web Profile page shows ("}<strong key={1}>{"None / Dietary / Allergy"}</strong>{") per allergen. Records are per-member; whoever's cooking can fix their own right from the kitchen."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-allergies.png"} alt={"Mobile — the allergy editor on the People tab, milk set to Allergy"} device="phone" />
      </Section>
      <Section title={"Where warnings show up"}>
        <P>{<Fragment key={1}>{"Warning icons ride along on the surfaces you touch while cooking and shopping — grocery lines, meal-plan slots, recipe pages, and event dishes. Tap a warning chip for the member-by-member breakdown (who's affected, which allergen, contains vs. may-contain)."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-allergy-warning.png"} alt={"Mobile — a grocery line flagged for a member's milk allergy"} device="phone" />
        <P>{<Fragment key={2}>{"Confirmed-allergen "}<code key={0}>{"contains"}</code>{" conflicts show red; dietary restrictions and "}<code key={1}>{"may_contain"}</code>{" flags stay amber — same severity rules as the web."}</Fragment>}</P>
      </Section>
      <Section title={"Web-only"}>
        <P>{<Fragment key={3}>{"The "}<strong key={0}>{"allergen registry"}</strong>{" (Inventory → Allergens) and the "}<strong key={1}>{"AI flag review queue"}</strong>{" (Inventory → Allergen Suggestions) are admin surfaces on the web — see "}<DocLink key={2} href="/docs/allergy-tracking#curating-allergen-data-web-admins">{"Allergy Tracking"}</DocLink>{"."}</Fragment>}</P>
        <P>{<Fragment key={4}>{"← Back to "}<DocLink key={0} href="/docs/mobile-app">{"Using the Mobile App"}</DocLink></Fragment>}</P>
      </Section>
    </DocPage>
  );
}
