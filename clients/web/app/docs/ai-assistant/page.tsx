import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  DocLink,
  DocList,
  DocTable,
  P,
  Section,
  Screenshot,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "AI Assistant (Dot) — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"AI Assistant (Dot)"} intro={"LENA can run a local LLM (Ollama by default) as an optional assistant that answers questions about your household's data and makes reviewable suggestions. When no AI provider is configured, every AI surface stays hidden — the app works exactly the same without it."}>
      <Section title={"Enabling the assistant"}>
        <P>{<Fragment key={0}>{"Set "}<code key={0}>{"LENA_AI_PROVIDER=ollama"}</code>{" and point it at a running Ollama ("}<code key={1}>{"LENA_OLLAMA_URL"}</code>{", "}<code key={2}>{"LENA_AI_MODEL"}</code>{"), or start the bundled service with "}<code key={3}>{"docker compose --profile ai up"}</code>{". For tests and demos, "}<code key={4}>{"LENA_AI_PROVIDER=mock"}</code>{" returns a canned deterministic answer."}</Fragment>}</P>
        <P>{<Fragment key={1}>{"Whenever "}<code key={0}>{"LENA_OLLAMA_URL"}</code>{" is set, LENA also enables "}<strong key={1}>{"semantic recipe search"}</strong>{": a small embedding model ("}<code key={2}>{"LENA_AI_EMBED_MODEL"}</code>{", default "}<code key={3}>{"nomic-embed-text"}</code>{") turns each recipe's name, description, ingredients, and categories into a vector, and a background sweep keeps embeddings fresh as recipes change. The "}<code key={4}>{"ai"}</code>{" compose profile pulls the embedding model alongside the chat model."}</Fragment>}</P>
      </Section>
      <Section title={"Ask Dot"}>
        <P>{<Fragment key={2}><strong key={0}>{"Ask Dot"}</strong>{" lives in the web header (the ✨ link on the second row) and as the "}<strong key={1}>{"Assistant"}</strong>{" tab on mobile — a free-form chat. Ask about your pantry, meal plan, recipes, or cellar — Dot looks up real data through a set of read-only, household-scoped tools and shows you which lookups it used under each reply (\"looked up: get_expiring_items\")."}</Fragment>}</P>
        <Screenshot src={"/images/assistant.png"} alt={"Ask Dot"} device="web" />
        <Screenshot src={"/images/assistant-answer.png"} alt={"Assistant answer with tool attribution"} device="web" />
        <Screenshot src={"/images/assistant-answer-dark.png"} alt={"An answered question in dark mode — engine badge and provenance"} device="web" />
        <P>{<Fragment key={3}>{"The model can only read — it cannot change anything directly. Anything actionable is suggested, never applied silently."}</Fragment>}</P>
        <P>{<Fragment key={4}>{"Dot's recipe tools include "}<strong key={0}>{"semantic search"}</strong>{" ("}<code key={1}>{"search_recipes_semantic"}</code>{"): ask \"something cozy for a rainy night\" and it matches on meaning — a recipe called \"Creamy Tomato Soup\" surfaces even though you never typed \"soup\". Because embeddings can't express exclusions, Dot verifies \"no eggs\"-style requests by reading each candidate's ingredient list before recommending."}</Fragment>}</P>
      </Section>
      <Section title={"On-device inference"}>
        <P>{<Fragment key={5}>{"Both clients can run the model "}<strong key={0}>{"on your own device"}</strong>{" — keeping prompts local and taking the compute load off the server entirely:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={6}><strong key={0}>{"Web"}</strong>{" tries Chrome's built-in AI model first, then a downloadable WebLLM model (fetched once, cached by the browser), then falls back to the server."}</Fragment>,
          <Fragment key={7}><strong key={0}>{"Mobile"}</strong>{" runs a small Gemma model on the phone via "}<code key={1}>{"flutter_gemma"}</code>{"."}</Fragment>,
        ]} />
        <P>{<Fragment key={8}>{"It's "}<strong key={0}>{"opt-in"}</strong>{": the assistant page shows an \"On this device\" card with download progress, and an engine badge on each reply tells you where the answer came from. You can switch to server-only mode any time. Whether Dot runs on your device or on the server, it uses the exact same tools, prompts, and household scope — the server stays the authority and suggestions still apply through the normal controls."}</Fragment>}</P>
        <Screenshot src={"/images/assistant-dark.png"} alt={"Ask Dot in dark mode"} device="web" />
        <P>{<Fragment key={9}>{"The full privacy story — engine fallback chain, what the tools can see, what never leaves the device — lives on "}<DocLink key={0} href="/docs/privacy">{"Privacy & Local AI"}</DocLink>{"."}</Fragment>}</P>
      </Section>
      <Section title={"Built-in suggestions"}>
        <DocTable head={["Feature","Where","What it does"]} rows={[[<Fragment key={10}><strong key={0}>{"Suggest Meals"}</strong></Fragment>, <Fragment key={11}>{"Meal plan page"}</Fragment>, <Fragment key={12}>{"Proposes recipes for open slots, informed by what's in stock, what's expiring soon, and your household's taste analytics. Each suggestion is a reviewable card you add via the normal slot controls."}</Fragment>], [<Fragment key={13}><strong key={0}>{"Suggest Fixes"}</strong></Fragment>, <Fragment key={14}>{"Event timeline"}</Fragment>, <Fragment key={15}>{"When the cooking timeline flags appliance conflicts, the assistant proposes fixes — shift a serve time, reassign an appliance, adjust a duration or dependency. Each fix applies through the existing event controls after you review it."}</Fragment>], [<Fragment key={16}><strong key={0}>{"Wine pairing"}</strong></Fragment>, <Fragment key={17}>{"Recipe detail"}</Fragment>, <Fragment key={18}>{"Suggests bottles from your cellar (marked \"In your cellar\") plus general style picks for the recipe."}</Fragment>], [<Fragment key={19}><strong key={0}>{"Cocktail ideas"}</strong></Fragment>, <Fragment key={20}>{"Recipes list"}</Fragment>, <Fragment key={21}>{"Picks cocktail recipes; the \"Only what I can make\" toggle drops anything your pantry can't cover and lists missing ingredients."}</Fragment>], [<Fragment key={22}><strong key={0}>{"Allergen flags"}</strong></Fragment>, <Fragment key={23}>{"Inventory → Allergen Suggestions (admins)"}</Fragment>, <Fragment key={24}>{"Reads a recipe's ingredients and proposes "}<code key={0}>{"contains"}</code>{"/"}<code key={1}>{"may_contain"}</code>{" flags with a rationale. Proposals land in a review queue — "}<strong key={2}>{"Accept"}</strong>{" writes the flag under your name, "}<strong key={3}>{"Dismiss"}</strong>{" keeps the audit trail. Nothing applies automatically."}</Fragment>]]} />
        <P>{<Fragment key={25}>{"See "}<DocLink key={0} href="/docs/allergy-tracking">{"Allergy Tracking"}</DocLink>{" for how the flags feed cooking warnings."}</Fragment>}</P>
      </Section>
      <Section title={"Age gate for alcohol features"}>
        <P>{<Fragment key={26}>{"Wine pairing and cocktail suggestions require a "}<strong key={0}>{"birthdate on your profile"}</strong>{" showing you're of legal drinking age. The check runs server-side — it can't be bypassed from a client. If your birthdate isn't set, the buttons stay hidden and the API returns an error asking you to set it."}</Fragment>}</P>
      </Section>
      <Section title={"Privacy & safety"}>
        <DocList items={[
          <Fragment key={27}>{"Tools only ever see "}<strong key={0}>{"your household's"}</strong>{" data — the assistant cannot read or expose another household's inventory, plans, or cellar."}</Fragment>,
          <Fragment key={28}>{"All suggestions are "}<strong key={0}>{"advisory"}</strong>{": the AI proposes, the existing forms and mutations apply — with the same permissions as if you did it by hand."}</Fragment>,
          <Fragment key={29}>{"The provider is "}<strong key={0}>{"pluggable"}</strong>{": Ollama today, a commercial API tomorrow with a small adapter."}</Fragment>,
        ]} />
      </Section>
    </DocPage>
  );
}
