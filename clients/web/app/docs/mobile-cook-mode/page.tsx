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
  title: "Cook Mode (Mobile) — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Cook Mode (Mobile)"} intro={"Cook mode turns a recipe into a full-screen, glanceable step pager — built for the phone or tablet propped on the counter while your hands are busy. It lives only in the mobile app; the web shows recipes as scrollable pages instead."}>
      <Section title={"Opening it"}>
        <P>{<Fragment key={0}>{"Open any recipe in the app ("}<strong key={0}>{"More → Recipes"}</strong>{", or a suggestion card on Home) and tap the "}<strong key={1}>{"chef's-hat button"}</strong>{" in the recipe editor's top bar. Cook mode takes over the whole screen — even when the recipe was open in a two-pane layout on a tablet."}</Fragment>}</P>
      </Section>
      <Section title={"The step pager"}>
        <P>{<Fragment key={1}>{"Each page is one step:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={2}><strong key={0}>{"Step counter"}</strong>{" — a large "}<code key={1}>{"N / total"}</code>{" so you always know where you are"}</Fragment>,
          <Fragment key={3}><strong key={0}>{"Instructions"}</strong>{" in headline-sized type, readable from arm's length"}</Fragment>,
          <Fragment key={4}><strong key={0}>{"Badges"}</strong>{" for step type and hands-off steps (e.g. \"bake — hands off\")"}</Fragment>,
          <Fragment key={5}><strong key={0}>{"Countdown chips"}</strong>{" on timed steps — tap to start the timer for that step"}</Fragment>,
          <Fragment key={6}><strong key={0}>{"Chevrons or swipes"}</strong>{" to move forward and back; the progress bar tracks your place"}</Fragment>,
        ]} />
        <P>{<Fragment key={7}>{"Cook mode keeps the screen awake while it's open — no unlocking with floury hands. Leaving the screen (back button) restores normal sleep behavior."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-cook-mode.png"} alt={"Cook mode on a phone — step 1 of 5 with the progress bar and Ingredients button"} device="phone" />
      </Section>
      <Section title={"Ingredients alongside the steps"}>
        <P>{<Fragment key={8}>{"The recipe's ingredient list stays reachable without leaving the pager:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={9}><strong key={0}>{"On tablets and landscape"}</strong>{" the ingredients ride in a "}<strong key={1}>{"left rail"}</strong>{" beside the step:"}</Fragment>,
        ]} />
        <Screenshot src={"/images/tablet-11-cook-mode.png"} alt={"Cook mode on a tablet — the ingredients rail on the left, the step pager on the right"} device="tablet" />
        <DocList items={[
          <Fragment key={10}><strong key={0}>{"On phones"}</strong>{" a floating "}<strong key={1}>{"Ingredients"}</strong>{" button opens them as a bottom sheet, so the step stays visible behind it:"}</Fragment>,
        ]} />
        <Screenshot src={"/images/mobile-cook-mode-ingredients.png"} alt={"The ingredients bottom sheet over a cook-mode step"} device="phone" />
        <P>{<Fragment key={11}>{"Advancing steps updates the counter and progress bar:"}</Fragment>}</P>
        <Screenshot src={"/images/tablet-12-cook-mode-step2.png"} alt={"Cook mode step 2 on a tablet — the progress bar tracks your place"} device="tablet" />
      </Section>
      <Section title={"Notes"}>
        <DocList items={[
          <Fragment key={12}>{"Cook mode shows the recipe exactly as stored — it follows the "}<DocLink key={0} href="/docs/recipes#household-tweaks">{"household tweaks"}</DocLink>{" view when that's what the recipe page is showing."}</Fragment>,
          <Fragment key={13}>{"Timers are advisory countdowns shown on the step; they don't alarm across the app."}</Fragment>,
        ]} />
        <P>{<Fragment key={14}>{"← Back to "}<DocLink key={0} href="/docs/mobile-app">{"Using the Mobile App"}</DocLink>{" · See also "}<DocLink key={1} href="/docs/mobile-recipes">{"Recipes & Meal Plans"}</DocLink></Fragment>}</P>
      </Section>
    </DocPage>
  );
}
