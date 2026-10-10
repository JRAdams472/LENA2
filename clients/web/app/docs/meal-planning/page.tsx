import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  Callout,
  DocLink,
  DocList,
  P,
  Section,
  Screenshot,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Meal Planning — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Meal Planning"} intro={"Meal planning in LENA is organized around weekly plans: one page per week, with a grid of days and meals."}>
        <Screenshot src={"/images/meal-plans.png"} alt={"Your meal plans"} device="web" />
      <Section title={"Creating a weekly plan"}>
        <DocList ordered items={[
          <Fragment key={0}>{"Go to "}<strong key={0}>{"Meal Planning → Weekly Plan"}</strong>{"."}</Fragment>,
          <Fragment key={1}>{"Create a new plan and pick its "}<strong key={0}>{"week start date"}</strong>{"."}</Fragment>,
          <Fragment key={2}>{"Open the plan — you'll see a card for each day of the week."}</Fragment>,
        ]} />
        <Screenshot src={"/images/meal-plan-week.png"} alt={"The weekly grid — dinners planned for Sunday and Monday"} device="web" />
      </Section>
      <Section title={"Filling in meals"}>
        <P>{<Fragment key={3}>{"Each day has slots for "}<strong key={0}>{"Breakfast, Lunch, and Dinner"}</strong>{". For each slot you can:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={4}>{"Pick a "}<strong key={0}>{"recipe"}</strong>{" from your household collection, or leave it blank"}</Fragment>,
          <Fragment key={5}>{"Set "}<strong key={0}>{"servings"}</strong>{" — how many people the meal needs to feed. Servings matter: they're used to scale ingredient quantities on your grocery list"}</Fragment>,
        ]} />
        <P>{<Fragment key={6}>{"You don't have to fill every slot — plan just dinners, or the whole week, whatever suits you."}</Fragment>}</P>
        <Callout>{<Fragment key={7}>{"If a planned recipe conflicts with someone's "}<DocLink key={0} href="/docs/allergy-tracking">{"allergy records"}</DocLink>{", the slot shows a warning chip — hover or tap for which member and which allergen."}</Fragment>}</Callout>
      </Section>
      <Section title={"The plan and the Dashboard"}>
        <P>{<Fragment key={8}>{"When today's date falls inside a plan's week, that plan becomes \"active\" and its meals show up on your "}<DocLink key={0} href="/docs/dashboard">{"Dashboard"}</DocLink>{" under today's Breakfast / Lunch / Dinner."}</Fragment>}</P>
      </Section>
      <Section title={"Generating a grocery list"}>
        <P>{<Fragment key={9}>{"From the plan page, hit "}<strong key={0}>{"Generate Grocery List"}</strong>{". LENA walks the week's recipes and writes a grocery list with every ingredient across all planned recipes — "}<strong key={1}>{"scaled by each slot's servings, minus what your pantry already has"}</strong>{" (needs your stock fully covers are dropped)."}</Fragment>}</P>
        <P>{<Fragment key={10}>{"See "}<DocLink key={0} href="/docs/grocery-lists">{"Grocery Lists"}</DocLink>{" for how to shop with it."}</Fragment>}</P>
      </Section>
      <Section title={"Editing and sharing"}>
        <P>{<Fragment key={11}>{"Weekly plans are household-shared: anyone in your household can open the plan and adjust it. Changes notify the other members, so nobody plans dinner on stale information."}</Fragment>}</P>
      </Section>
    </DocPage>
  );
}
