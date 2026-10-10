import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  DocLink,
  DocList,
  DocSub,
  P,
  Section,
  Screenshot,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Ask Dot on Mobile — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Ask Dot on Mobile"} intro={"The Ask Dot tab is the same household-scoped chat as the web — questions are answered from your pantry, plans, and events, and each reply shows the lookups it used."}>
        <Screenshot src={"/images/mobile-assistant.png"} alt={"Ask Dot on mobile"} device="phone" />
        <P>{<Fragment key={0}>{"Type a question — "}<em key={0}>{"\"What should I make for dinner?\""}</em>{", "}<em key={1}>{"\"What's expiring soon?\""}</em>{" — and Dot answers from your actual household data, with a footer listing which lookups informed the answer. A fresh chat shows a few "}<strong key={2}>{"prompt chips"}</strong>{" across the top that mirror the web suggestions, so you can tap one instead of typing."}</Fragment>}</P>
      <Section title={"On-device model"}>
        <P>{<Fragment key={1}>{"Dot can run "}<strong key={0}>{"right on your phone"}</strong>{" — a small Gemma model handles inference locally, so nothing leaves the device for the thinking step. The first time you open the tab an opt-in card offers the download (with a progress bar and cancel); the overflow menu can delete the model or force server-only mode. Each reply's engine label shows where it came from — "}<strong key={1}>{"On this device"}</strong>{" or "}<strong key={2}>{"Via server"}</strong>{" — and if the local engine isn't ready, Dot quietly uses the server instead."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-assistant-optin.png"} alt={"Opt-in card offering the on-device model download"} device="phone" />
        <Screenshot src={"/images/mobile-assistant-dark.png"} alt={"Ask Dot in dark mode"} device="phone" />
        <DocSub>{<Fragment key={2}>{"Managing the model"}</Fragment>}</DocSub>
        <DocList items={[
          <Fragment key={3}><strong key={0}>{"Download"}</strong>{" — from the opt-in card; progress shows in place and can be cancelled mid-flight."}</Fragment>,
          <Fragment key={4}><strong key={0}>{"Delete"}</strong>{" — the overflow (⋮) menu offers "}<em key={1}>{"Delete model"}</em>{", reclaiming the storage and returning Dot to server mode."}</Fragment>,
          <Fragment key={5}><strong key={0}>{"Fallback"}</strong>{" — if the model isn't installed or the engine can't start, replies still work; the badge just reads "}<em key={1}>{"Via server"}</em>{"."}</Fragment>,
        ]} />
        <P>{<Fragment key={6}>{"Either way, Dot uses the same read-only, household-scoped lookups as the web assistant — see "}<DocLink key={0} href="/docs/privacy">{"Privacy & Local AI"}</DocLink>{" for exactly what the model can reach and what never leaves the device."}</Fragment>}</P>
        <P>{<Fragment key={7}>{"See "}<DocLink key={0} href="/docs/ai-assistant">{"AI Assistant"}</DocLink>{" for what Dot can do and how to enable it."}</Fragment>}</P>
        <P>{<Fragment key={8}>{"← Back to "}<DocLink key={0} href="/docs/mobile-app">{"Using the Mobile App"}</DocLink></Fragment>}</P>
      </Section>
    </DocPage>
  );
}
