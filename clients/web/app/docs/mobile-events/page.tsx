import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  DocLink,
  P,
  Screenshot,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Food Events on Mobile — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Food Events on Mobile"} intro={"The Events tab lists your household's food events — dinners, parties, holiday meals."}>
        <Screenshot src={"/images/mobile-events.png"} alt={"Events list"} device="phone" />
        <P>{<Fragment key={0}>{"Tap an event to see its dishes (recipe or free-form, meal type, servings with the scale badge, serve time), add or remove dishes — the dish field opens a searchable recipe sheet, or pick "}<strong key={0}>{"Free-form dish"}</strong>{" — and open the "}<strong key={1}>{"cooking timeline"}</strong>{": a step-by-step checklist of start-by times with hands-off and appliance tags, so you can prop the phone in the kitchen and cook from it."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-event-detail.png"} alt={"Event detail — dishes with serve times and scaling"} device="phone" />
        <Screenshot src={"/images/mobile-event-timeline.png"} alt={"The cooking timeline as a cook-from checklist"} device="phone" />
        <Screenshot src={"/images/mobile-edit-event.png"} alt={"Editing an event — name, date, granularity"} device="phone" />
        <Screenshot src={"/images/mobile-events-dark.png"} alt={"Events in dark mode"} device="phone" />
        <P>{<Fragment key={1}>{"Deeper editing — renaming event steps, tweaking the copied ingredient list, syncing a slot from its recipe — stays on the web; mobile covers planning, viewing, and cooking. See "}<DocLink key={0} href="/docs/food-events">{"Food Events"}</DocLink>{" for how timelines and snapshots work."}</Fragment>}</P>
        <P>{<Fragment key={2}>{"← Back to "}<DocLink key={0} href="/docs/mobile-app">{"Using the Mobile App"}</DocLink></Fragment>}</P>
    </DocPage>
  );
}
