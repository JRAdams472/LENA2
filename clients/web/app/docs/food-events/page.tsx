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
  title: "Food Events — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Food Events"} intro={"Food events are for cooking occasions bigger than a weeknight dinner — Thanksgiving, a dinner party, a bake sale. An event gathers the dishes you're making, says when each one needs to hit the table, and can generate a cooking timeline that works backwards from your serve times."}>
      <Section title={"Creating an event"}>
        <P>{<Fragment key={0}>{"Go to "}<strong key={0}>{"Meal Planning → Events"}</strong>{" (web) and create an event with:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={1}><strong key={0}>{"Name"}</strong>{" — \"Thanksgiving dinner\", \"Saturday cookout\"…"}</Fragment>,
          <Fragment key={2}><strong key={0}>{"Date"}</strong></Fragment>,
          <Fragment key={3}><strong key={0}>{"Time granularity"}</strong>{" — "}<strong key={1}>{"15 or 30 minutes"}</strong>{". This controls how finely the timeline schedules steps: 15-minute events get precise start-by times, 30-minute events stay coarser and easier to read. You can change it later."}</Fragment>,
        ]} />
        <Screenshot src={"/images/events.png"} alt={"The Food Events list"} device="web" />
      </Section>
      <Section title={"Adding dishes"}>
        <P>{<Fragment key={4}>{"On the event's page, each dish is a "}<strong key={0}>{"slot"}</strong>{". A slot can be:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={5}><strong key={0}>{"A recipe"}</strong>{" from your collection — LENA pulls its steps and ingredients into the event"}</Fragment>,
          <Fragment key={6}><strong key={0}>{"A free-form dish"}</strong>{" — just a name (\"Grandma's stuffing\", \"Store-bought rolls\") for things that aren't recipes"}</Fragment>,
        ]} />
        <P>{<Fragment key={7}>{"Each slot gets:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={8}><strong key={0}>{"Meal type"}</strong>{" (breakfast, lunch, dinner, etc.)"}</Fragment>,
          <Fragment key={9}><strong key={0}>{"Servings"}</strong>{" — how many people this dish must feed; ingredient quantities "}<strong key={1}>{"scale automatically"}</strong>{" (the slot shows a badge like "}<code key={2}>{"×2 of 4"}</code>{" when you're cooking double the recipe's base servings)"}</Fragment>,
          <Fragment key={10}><strong key={0}>{"Serve time"}</strong>{" — when the dish should be done. The time picker snaps to the event's granularity, so a 30-minute event offers 6:00, 6:30, 7:00…"}</Fragment>,
          <Fragment key={11}><strong key={0}>{"Notes"}</strong>{" — anything worth remembering (\"use the good casserole dish\")"}</Fragment>,
        ]} />
        <P>{<Fragment key={12}>{"Dishes that conflict with a guest's "}<DocLink key={0} href="/docs/allergy-tracking">{"allergy records"}</DocLink>{" surface a warning on the slot — worth checking before you lock the menu for a crowd."}</Fragment>}</P>
        <Screenshot src={"/images/event-detail.png"} alt={"An event's dish list — serve times, scale badges, and a free-form slot"} device="web" />
      </Section>
      <Section title={"Making a dish your own — without touching the recipe"}>
        <P>{<Fragment key={13}>{"This is the part worth understanding: "}<strong key={0}>{"when you add a recipe to an event, the event takes a private copy of its steps and ingredients."}</strong>{" Editing the dish inside the event never changes the shared recipe — and editing the shared recipe later never changes your event plan."}</Fragment>}</P>
        <P>{<Fragment key={14}>{"Expand a slot to see its "}<strong key={0}>{"Steps"}</strong>{" and "}<strong key={1}>{"Ingredients"}</strong>{", then:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={15}><strong key={0}>{"Adjust timings"}</strong>{" for your kitchen — give the roast a longer rest, split a step into two, mark a step hands-off"}</Fragment>,
          <Fragment key={16}><strong key={0}>{"Tweak ingredients"}</strong>{" — double the garlic for this event only; scaled quantities show alongside the base amount"}</Fragment>,
          <Fragment key={17}><strong key={0}>{"Add steps or ingredients"}</strong>{" the recipe doesn't have — brining the bird, chilling the dough"}</Fragment>,
          <Fragment key={18}><strong key={0}>{"Sync from recipe"}</strong>{" — if you "}<em key={1}>{"want"}</em>{" to pick up changes made to the shared recipe, one button re-copies it (you'll be asked to confirm, since it replaces your event edits)"}</Fragment>,
        ]} />
        <P>{<Fragment key={19}>{"Deletions are safe too — removing a dish's slot doesn't touch the recipe."}</Fragment>}</P>
      </Section>
      <Section title={"Generating the timeline"}>
        <P>{<Fragment key={20}>{"When dishes have steps with durations, hit "}<strong key={0}>{"Generate Timeline"}</strong>{". For every scheduled dish you get a step-by-step table: "}<strong key={1}>{"what to do and when to start it"}</strong>{", working backwards from the serve time so everything lands together."}</Fragment>}</P>
        <Screenshot src={"/images/event-timeline.png"} alt={"The generated timeline — start-by times, appliance tags, and hands-off steps"} device="web" />
        <P>{<Fragment key={21}>{"On mobile, open the event and tap "}<strong key={0}>{"View cooking timeline"}</strong>{" — the same schedule rendered as a cook-from checklist you can keep on the counter."}</Fragment>}</P>
        <P>{<Fragment key={22}>{"Watch for:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={23}><strong key={0}>{"Est."}</strong>{" — the step had no duration, so LENA estimated one time slot. Fill in real durations on the steps for a tighter schedule"}</Fragment>,
          <Fragment key={24}><strong key={0}>{"Hands-off"}</strong>{" — passive steps (marinating, rising, resting) you don't need to stand over"}</Fragment>,
          <Fragment key={25}><strong key={0}>{"Red conflict rows"}</strong>{" — two steps that need the same appliance at the same time (both using the oven at 5:30). The timeline "}<em key={1}>{"reports"}</em>{" conflicts so you can reschedule; it doesn't silently move your steps"}</Fragment>,
          <Fragment key={26}><strong key={0}>{"Warning banners"}</strong>{" — steps LENA couldn't schedule (e.g. a dependency loop)"}</Fragment>,
        ]} />
      </Section>
      <Section title={"Tips"}>
        <DocList items={[
          <Fragment key={27}>{"Put "}<strong key={0}>{"serve times a little early"}</strong>{" — it's easier to hold a dish warm than to wait on it."}</Fragment>,
          <Fragment key={28}>{"Use "}<strong key={0}>{"free-form slots"}</strong>{" for anything bought ready-made so it still shows in the plan."}</Fragment>,
          <Fragment key={29}>{"The timeline is generated fresh every time you open it — no stale schedules; it always reflects your latest edits."}</Fragment>,
          <Fragment key={30}>{"Hit "}<strong key={0}>{"Suggest Fixes"}</strong>{" (with an "}<DocLink key={1} href="/docs/ai-assistant">{"AI provider configured"}</DocLink>{") when the timeline flags appliance conflicts — Dot proposes reschedules you review and apply one at a time; nothing moves on its own."}</Fragment>,
        ]} />
        <Screenshot src={"/images/event-detail-dark.png"} alt={"The event page in dark mode — dishes and generated timeline"} device="web" />
      </Section>
    </DocPage>
  );
}
