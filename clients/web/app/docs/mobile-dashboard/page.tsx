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
  title: "Mobile Dashboard — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Mobile Dashboard"} intro={"The Home tab is the first thing you see — a greeting header with today's date and your name, then two zones covering \"what am I eating\" and \"what could I make.\""}>
        <Screenshot src={"/images/mobile-dashboard.png"} alt={"Mobile dashboard"} device="phone" />
        <Screenshot src={"/images/mobile-dashboard-dark.png"} alt={"The Home tab in dark mode"} device="phone" />
      <Section title={"Today's meals"}>
        <P>{<Fragment key={0}>{"Every slot on today's meal plan shows a meal-type icon (breakfast, lunch, or dinner), the recipe name, servings and cook time, and a chevron — tap a slot to open the recipe."}</Fragment>}</P>
        <P>{<Fragment key={1}>{"Nothing planned yet? The card says "}<em key={0}>{"\"Nothing planned for today yet\""}</em>{" with a "}<strong key={1}>{"Plan it →"}</strong>{" link that jumps straight to meal planning."}</Fragment>}</P>
      </Section>
      <Section title={"Delicious ideas for tonight"}>
        <P>{<Fragment key={2}>{"Below the plan sits the household's recommendations — the same engine as the web dashboard:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={3}>{"A "}<strong key={0}>{"featured card"}</strong>{" leads with the top pick: a why-it's-suggested chip, a category-colored icon, cook time, and a "}<strong key={1}>{"View recipe →"}</strong>{" link."}</Fragment>,
          <Fragment key={4}><strong key={0}>{"Compact suggestion rows"}</strong>{" follow — icon badges, cook times, friendly reasons like "}<em key={1}>{"\"Matches your household's tastes\""}</em>{" or "}<em key={2}>{"\"Due for a revisit\""}</em>{" (never raw backend codes), and a chevron that taps through to the recipe."}</Fragment>,
        ]} />
        <P>{<Fragment key={5}>{"See "}<DocLink key={0} href="/docs/analytics">{"Analytics & Recommendations"}</DocLink>{" for how suggestions are ranked."}</Fragment>}</P>
        <P>{<Fragment key={6}>{"← Back to "}<DocLink key={0} href="/docs/mobile-app">{"Using the Mobile App"}</DocLink></Fragment>}</P>
      </Section>
    </DocPage>
  );
}
