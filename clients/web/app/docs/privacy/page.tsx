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
  title: "Privacy & Local AI — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Privacy & Local AI"} intro={"LENA's assistant (Dot) can run its language model on your own device, so the thinking step never leaves your phone or browser. This page is the complete privacy story: what the model can see, what never leaves the device, and where the authority lives."}>
      <Section title={"The engine chain"}>
        <P>{<Fragment key={0}><strong key={0}>{"Web"}</strong>{" tries local engines in this order, then falls back to the server:"}</Fragment>}</P>
        <DocList ordered items={[
          <Fragment key={1}><strong key={0}>{"Chrome built-in AI"}</strong>{" ("}<code key={1}>{"LanguageModel"}</code>{" / "}<code key={2}>{"window.ai.languageModel"}</code>{") — Chrome's bundled model; zero download for LENA."}</Fragment>,
          <Fragment key={2}><strong key={0}>{"WebLLM"}</strong>{" — a small model over WebGPU; works in any GPU-capable browser. Downloaded once and cached by the browser."}</Fragment>,
          <Fragment key={3}><strong key={0}>{"Server"}</strong>{" — the configured provider (Ollama by default, or the "}<code key={1}>{"mock"}</code>{" provider in demos/tests)."}</Fragment>,
        ]} />
        <P>{<Fragment key={4}><strong key={0}>{"Mobile"}</strong>{" runs a small "}<strong key={1}>{"Gemma"}</strong>{" model on the phone via "}<code key={2}>{"flutter_gemma"}</code>{" — opt-in, downloaded once, deletable from the assistant's overflow menu."}</Fragment>}</P>
        <P>{<Fragment key={5}>{"If the local engine can't initialize or isn't ready, Dot "}<strong key={0}>{"quietly falls back to the server"}</strong>{" — answers keep working either way."}</Fragment>}</P>
      </Section>
      <Section title={"It's opt-in, and you can see which engine answered"}>
        <DocList items={[
          <Fragment key={6}>{"Nothing downloads without your say-so: the assistant shows an opt-in card with a progress bar (and cancel) before any model lands on your device."}</Fragment>,
          <Fragment key={7}>{"Every reply carries an "}<strong key={0}>{"engine badge"}</strong>{" — "}<em key={1}>{"On this device"}</em>{" or "}<em key={2}>{"Via server"}</em>{" — so the provenance is never ambiguous."}</Fragment>,
          <Fragment key={8}>{"On mobile you can delete the model or force server-only mode at any time from the assistant's overflow menu."}</Fragment>,
        ]} />
        <Screenshot src={"/images/assistant.png"} alt={"Ask Dot showing the \"Via server\" engine badge and tool provenance"} device="web" />
        <Screenshot src={"/images/assistant-dark.png"} alt={"Ask Dot in dark mode"} device="web" />
      </Section>
      <Section title={"What the model can see — and can't"}>
        <P>{<Fragment key={9}>{"Whether Dot runs on your device or the server, it uses "}<strong key={0}>{"the same set of read-only, household-scoped tools"}</strong>{", and the server executes every lookup:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={10}><code key={0}>{"get_pantry_inventory"}</code>{", "}<code key={1}>{"get_expiring_items"}</code></Fragment>,
          <Fragment key={11}><code key={0}>{"get_meal_plan"}</code>{", "}<code key={1}>{"list_recipes"}</code>{", "}<code key={2}>{"get_recipe_details"}</code>{", "}<code key={3}>{"search_recipes_semantic"}</code></Fragment>,
          <Fragment key={12}><code key={0}>{"get_event_timeline"}</code></Fragment>,
          <Fragment key={13}><code key={0}>{"get_wine_cellar"}</code>{", "}<code key={1}>{"list_cocktail_recipes"}</code></Fragment>,
          <Fragment key={14}><code key={0}>{"get_household_tastes"}</code>{", "}<code key={1}>{"get_recipe_allergen_candidates"}</code></Fragment>,
        ]} />
        <P>{<Fragment key={15}>{"Scope guarantees:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={16}><strong key={0}>{"Household-scoped"}</strong>{" — the tools can only read "}<em key={1}>{"your"}</em>{" household's data. The assistant cannot reach another household's pantry, plans, or cellar."}</Fragment>,
          <Fragment key={17}><strong key={0}>{"Read-only"}</strong>{" — there is no write tool. Anything actionable is a "}<em key={1}>{"suggestion"}</em>{"; applying it goes through the same forms and mutations as if you did it by hand, with the same permissions."}</Fragment>,
          <Fragment key={18}><strong key={0}>{"Provenance shown"}</strong>{" — each answer lists the lookups it used (\"looked up: get_expiring_items\") so you can see exactly what data informed it."}</Fragment>,
        ]} />
      </Section>
      <Section title={"What leaves the device"}>
        <DocList items={[
          <Fragment key={19}><strong key={0}>{"On-device inference:"}</strong>{" your question and the tool results are processed locally — prompts never transit the server for the thinking step."}</Fragment>,
          <Fragment key={20}><strong key={0}>{"Server fallback:"}</strong>{" when the engine badge reads "}<em key={1}>{"Via server"}</em>{", the prompt goes to the configured provider (Ollama, run wherever you host it — there is no third-party API in the default stack)."}</Fragment>,
          <Fragment key={21}><strong key={0}>{"Model weights"}</strong>{" come from the platform's own distribution (Chrome's built-in model, the WebLLM cache, or the Gemma package for mobile) — LENA doesn't host or modify them."}</Fragment>,
        ]} />
      </Section>
      <Section title={"Server stays the authority"}>
        <P>{<Fragment key={22}>{"Local inference changes "}<em key={0}>{"where the model runs"}</em>{", not "}<em key={1}>{"what it's allowed to do"}</em>{". Every tool call is executed server-side under your session; the local engine only proposes. Suggestions still land through the normal reviewable controls — nothing is applied silently, on any engine."}</Fragment>}</P>
      </Section>
      <Section title={"Age gate for alcohol features"}>
        <P>{<Fragment key={23}>{"Wine pairing and cocktail suggestions require a "}<strong key={0}>{"birthdate on your profile showing 21+"}</strong>{". The check runs server-side in the suggestion resolvers — it can't be bypassed by a client. Without a birthdate the buttons stay hidden and the API returns an error explaining what's needed. Set it under "}<strong key={1}>{"Profile"}</strong>{" (web) — it's labeled \"Optional — unlocks wine pairing and cocktail suggestions (21+)\"."}</Fragment>}</P>
      </Section>
      <Section title={"When AI is entirely off"}>
        <P>{<Fragment key={24}>{"With no provider configured ("}<code key={0}>{"LENA_AI_PROVIDER"}</code>{" unset), every AI surface hides — no Ask Dot link, no suggestion buttons — and the app is otherwise identical. "}<code key={1}>{"LENA_AI_PROVIDER=mock"}</code>{" returns a deterministic canned answer for demos and tests, so screenshots and e2e runs never touch a real model."}</Fragment>}</P>
        <P>{<Fragment key={25}>{"See also: "}<DocLink key={0} href="/docs/ai-assistant">{"AI Assistant"}</DocLink>{" for features, "}<DocLink key={1} href="/docs/getting-started">{"Getting Started"}</DocLink>{" for provider configuration."}</Fragment>}</P>
      </Section>
    </DocPage>
  );
}
