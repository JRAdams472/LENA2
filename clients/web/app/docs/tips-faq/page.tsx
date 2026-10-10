import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  DocLink,
  DocList,
  P,
  Section,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Tips & FAQ — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Tips & FAQ"} intro={"Someone in my family can't find me to send an invite. Turn on \"Let other users find me by name or email\" on your Profile page. Both people need it on to be searchable."}>
      <Section title={"Getting the most out of LENA"}>
        <DocList items={[
          <Fragment key={0}><strong key={0}>{"Rate what you cook."}</strong>{" Ratings power your Dashboard recommendations — the more you rate, the better \"Due for a revisit\" and \"Recommended for you\" get."}</Fragment>,
          <Fragment key={1}><strong key={0}>{"Keep pantry counts honest."}</strong>{" Generated grocery lists subtract what you already have — accurate stock means shorter, more accurate lists."}</Fragment>,
          <Fragment key={2}><strong key={0}>{"Put real durations on recipe steps"}</strong>{" for anything you'd cook at an event — the timeline gets dramatically more accurate."}</Fragment>,
          <Fragment key={3}><strong key={0}>{"Scan barcodes when you unpack groceries"}</strong>{" — it's ten seconds per bag and keeps pantry counts honest."}</Fragment>,
          <Fragment key={4}><strong key={0}>{"Serve dishes a little early"}</strong>{" in event timelines; holding food warm beats waiting on it."}</Fragment>,
          <Fragment key={5}><strong key={0}>{"Categorize recipes as you add them."}</strong>{" A few taps on the recipe page and every future search — yours and your household's — gets easier. Filters combine, so \"Dinner + Italian + Easy\" is one short list."}</Fragment>,
          <Fragment key={6}><strong key={0}>{"The recipe list learns you."}</strong>{" Favorites first, then dishes your household has cooked, then ones you've viewed — so opening a recipe today nudges it up tomorrow's list."}</Fragment>,
        ]} />
      </Section>
      <Section title={"Common questions"}>
        <P>{<Fragment key={7}><strong key={0}>{"I joined a household — where did my stuff go?"}</strong>{" It came with you. Meal plans, grocery lists, and events move into the shared household when you accept an invite."}</Fragment>}</P>
        <P>{<Fragment key={8}><strong key={0}>{"I edited a recipe that's in an upcoming event — did I just change the event?"}</strong>{" No. Events take a private copy of recipe steps and ingredients when the dish is added. To pull in your new edits, open the event slot and hit "}<strong key={1}>{"Sync from recipe"}</strong>{". Editing inside the event likewise never touches the shared recipe."}</Fragment>}</P>
        <P>{<Fragment key={9}><strong key={0}>{"My event timeline shows red rows."}</strong>{" Two steps want the same appliance at the same time. Adjust serve times or step timing yourself — or, with an "}<DocLink key={1} href="/docs/ai-assistant">{"AI provider configured"}</DocLink>{", tap "}<strong key={2}>{"Suggest Fixes"}</strong>{" and review the proposed changes one at a time."}</Fragment>}</P>
        <P>{<Fragment key={10}><strong key={0}>{"Where did the Wine pairing / Cocktail ideas buttons go?"}</strong>{" Those need a birthdate on your "}<strong key={1}>{"Profile"}</strong>{" showing 21+ — the check is server-side. Until it's set, the buttons hide (and the API declines politely). See "}<DocLink key={2} href="/docs/privacy#age-gate-for-alcohol-features">{"Privacy & Local AI"}</DocLink>{"."}</Fragment>}</P>
        <P>{<Fragment key={11}><strong key={0}>{"Where's the Ask Dot link / Suggest buttons?"}</strong>{" If no AI provider is configured on the server ("}<code key={1}>{"LENA_AI_PROVIDER"}</code>{"), every AI surface stays hidden — nothing is broken, there's just no model to call. See "}<DocLink key={2} href="/docs/ai-assistant">{"AI Assistant"}</DocLink>{"."}</Fragment>}</P>
        <P>{<Fragment key={12}><strong key={0}>{"Why does the timeline say \"Est.\"?"}</strong>{" That step has no duration, so LENA estimated one time slot. Give the step a real duration (in the event's step editor, or on the recipe for future events) for a sharper schedule."}</Fragment>}</P>
        <P>{<Fragment key={13}><strong key={0}>{"A generated grocery list includes things I already have."}</strong>{" The generator subtracts your pantry stock from what the plan needs — if a line still appears, the recorded stock didn't fully cover it. Update your pantry count (or check the unit — incompatible units can't be subtracted), then regenerate the list."}</Fragment>}</P>
        <P>{<Fragment key={14}><strong key={0}>{"I tapped Generate Grocery List twice — did I make two lists?"}</strong>{" No. Generating for the same meal plan refreshes the existing list in place: it recomputes the \"From Menu\" lines against the current plan and pantry, and keeps anything you added manually. Same goes for double-taps and retries anywhere in the app — mutations are idempotent, so a repeat applies once."}</Fragment>}</P>
        <P>{<Fragment key={15}><strong key={0}>{"I scanned a barcode and nothing was found."}</strong>{" That item isn't in the catalog yet — fill in its details and submit it. Once an admin approves it, the barcode will resolve for everyone."}</Fragment>}</P>
        <P>{<Fragment key={16}><strong key={0}>{"Why don't I see Users / Pending Items / Pending Reviews?"}</strong>{" Those pages are admin-only — on the web they live under the "}<strong key={1}>{"Administration"}</strong>{" caption at the bottom of the sidebar (and nested under "}<strong key={2}>{"Recipes"}</strong>{" for review pages). If you think you should have them, check with whoever manages your LENA instance."}</Fragment>}</P>
      </Section>
      <Section title={"What's planned"}>
        <P>{<Fragment key={17}>{"Roadmap items that aren't in the app yet (don't go looking for them!):"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={18}><strong key={0}>{"Social-video recipe imports"}</strong>{", nutrition tracking, and more — see the project's feature notes for the full list"}</Fragment>,
        ]} />
      </Section>
    </DocPage>
  );
}
