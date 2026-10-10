import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  DocLink,
  DocTable,
  P,
  Section,
  Screenshot,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Tablet & Landscape Layouts (Mobile) — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Tablet & Landscape Layouts (Mobile)"} intro={"The mobile app is responsive: instead of stretching the phone layout, it adapts at Material 3 width breakpoints. Everything below applies to tablets, foldables, and phones rotated to landscape."}>
      <Section title={"The three size classes"}>
        <DocTable head={["Width","Class","What changes"]} rows={[[<Fragment key={0}><strong key={0}>{"< 600dp"}</strong></Fragment>, <Fragment key={1}>{"Compact"}</Fragment>, <Fragment key={2}>{"Bottom navigation bar, single-column screens — the phone layout"}</Fragment>], [<Fragment key={3}><strong key={0}>{"600–839dp"}</strong></Fragment>, <Fragment key={4}>{"Medium"}</Fragment>, <Fragment key={5}>{"Bottom bar becomes a "}<strong key={0}>{"NavigationRail"}</strong>{" down the left edge, every tab labeled"}</Fragment>], [<Fragment key={6}><strong key={0}>{"≥ 840dp"}</strong></Fragment>, <Fragment key={7}>{"Expanded"}</Fragment>, <Fragment key={8}>{"Rail stays, and list screens go "}<strong key={0}>{"two-pane"}</strong>{" (list on the left, detail beside it)"}</Fragment>]]} />
        <P>{<Fragment key={9}>{"A Pixel Tablet in landscape (1280×800dp) is fully expanded; the same device in portrait is medium — rail, single pane."}</Fragment>}</P>
      </Section>
      <Section title={"NavigationRail"}>
        <P>{<Fragment key={10}>{"Past 600dp the bottom bar becomes a left-edge rail with every destination labeled:"}</Fragment>}</P>
        <Screenshot src={"/images/tablet-01-dashboard-rail.png"} alt={"The dashboard with the navigation rail on a tablet"} device="tablet" />
        <Screenshot src={"/images/tablet-13-assistant.png"} alt={"Ask Dot on a tablet — rail on the left, chat centered at a comfortable width"} device="tablet" />
        <P>{<Fragment key={11}>{"Wide screens also cap content width (~840dp) so forms and chat don't sprawl edge to edge."}</Fragment>}</P>
      </Section>
      <Section title={"Two-pane list/detail"}>
        <P>{<Fragment key={12}>{"At expanded widths, tapping a list row opens its detail "}<strong key={0}>{"beside"}</strong>{" the list instead of covering the screen — the list pane stays ~360dp on the left. This applies to grocery lists, recipes, meal plans, events, pantry items, and wine bottles:"}</Fragment>}</P>
        <Screenshot src={"/images/tablet-03-grocery-two-pane.png"} alt={"Grocery lists in two-pane mode — the list on the left, an aisle-grouped list on the right"} device="tablet" />
        <P>{<Fragment key={13}>{"Before anything is selected, the right pane shows a gentle placeholder:"}</Fragment>}</P>
        <Screenshot src={"/images/tablet-02-grocery-lists.png"} alt={"An unselected two-pane layout — the list beside a \"Select a list\" placeholder"} device="tablet" />
        <P>{<Fragment key={14}>{"The detail pane has its own navigation: dialogs and sub-screens inside it pop within the pane, never pop the whole tab away. Deep links and push notifications still open the target detail directly, and on a compact phone nothing changes — taps push full-screen detail screens as before."}</Fragment>}</P>
        <P>{<Fragment key={15}>{"The More tab keeps its tile layout on tablets:"}</Fragment>}</P>
        <Screenshot src={"/images/tablet-04-more.png"} alt={"The More tab on a tablet — rail on the left, the same tile grid"} device="tablet" />
      </Section>
      <Section title={"Meal planning on a tablet"}>
        <P>{<Fragment key={16}>{"Opening a meal plan in the right pane shows the "}<strong key={0}>{"week grid"}</strong>{" — seven day columns by three meal rows:"}</Fragment>}</P>
        <Screenshot src={"/images/tablet-05-meal-plans.png"} alt={"Meal plans in two-pane mode — plans on the left, the week grid on the right"} device="tablet" />
        <P>{<Fragment key={17}>{"The grid sizes its day columns to the pane it's in: on a real tablet detail pane (~920dp next to the 360dp list) all seven days fit at once — columns shrink toward a 110dp floor, and below that the grid scrolls horizontally rather than squashing. On panes under ~700dp the plan falls back to scrolling day sections instead of the grid."}</Fragment>}</P>
        <Screenshot src={"/images/tablet-06-meal-plan-week.png"} alt={"The week grid — all seven days visible with servings and allergen badges"} device="tablet" />
        <P>{<Fragment key={18}>{"The week view's sheets open over the pane:"}</Fragment>}</P>
        <Screenshot src={"/images/tablet-07-suggest-meals.png"} alt={"The Suggest meals sheet — proposed slot, reason, and expiring-item reuse"} device="tablet" />
        <Screenshot src={"/images/tablet-08-nutrition.png"} alt={"The Nutrition sheet — aggregate nutrients and unlinked-ingredient warnings"} device="tablet" />
      </Section>
      <Section title={"Cook mode on a tablet"}>
        <P>{<Fragment key={19}><DocLink key={0} href="/docs/mobile-cook-mode">{"Cook mode"}</DocLink>{" uses the extra width for a persistent ingredients rail beside the step pager:"}</Fragment>}</P>
        <Screenshot src={"/images/tablet-11-cook-mode.png"} alt={"Cook mode on a tablet — ingredients rail left, step pager right"} device="tablet" />
      </Section>
      <Section title={"What a screenshot shows vs. what a real device does"}>
        <P>{<Fragment key={20}>{"The tablet shots on this page were captured on a Pixel Tablet emulator (2560×1600 @ 320dpi → 1280×800 logical). Portrait on the same hardware is medium width — rail without the second pane."}</Fragment>}</P>
        <P>{<Fragment key={21}>{"← Back to "}<DocLink key={0} href="/docs/mobile-app">{"Using the Mobile App"}</DocLink></Fragment>}</P>
      </Section>
    </DocPage>
  );
}
