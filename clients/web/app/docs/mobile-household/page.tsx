import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  DocLink,
  P,
  Section,
  Screenshot,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Household & Notifications on Mobile — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Household & Notifications on Mobile"} intro={"The People tab is two things in one: your notification feed and your member list."}>
        <Screenshot src={"/images/mobile-household.png"} alt={"Household tab"} device="phone" />
      <Section title={"Your households"}>
        <P>{<Fragment key={0}>{"The header card names your "}<strong key={0}>{"active household"}</strong>{" — the one whose pantry, plans, lists, events, and feed you're looking at. Tap "}<strong key={1}>{"Switch"}</strong>{" to open the switcher sheet: every household you belong to, with member counts, your role, and an "}<strong key={2}>{"Active"}</strong>{" marker on the current one. Each inactive row has a "}<strong key={3}>{"Switch"}</strong>{" button and a leave icon; "}<strong key={4}>{"New household"}</strong>{" at the bottom creates an additional household and makes it active."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-household-switcher.png"} alt={"Switching households"} device="phone" />
        <Screenshot src={"/images/mobile-household-dark.png"} alt={"The household switcher in dark mode"} device="phone" />
        <Screenshot src={"/images/mobile-household-create.png"} alt={"Creating another household"} device="phone" />
        <P>{<Fragment key={1}>{"Accepting an invite always joins the sender's household and makes it active. If you're the sole owner of another household you'll get the same choice as on the web — "}<strong key={0}>{"Join and merge"}</strong>{" (folds your pantry, cellar, plans, lists, and events in, then deletes the empty household) or "}<strong key={1}>{"Just join"}</strong>{" (keep both)."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-household-merge.png"} alt={"Merge or just join"} device="phone" />
        <Screenshot src={"/images/mobile-household-invitations.png"} alt={"Pending invitations on mobile"} device="phone" />
        <P>{<Fragment key={2}>{"See "}<DocLink key={0} href="/docs/households">{"Households & Members"}</DocLink>{" for roles and the invite flow."}</Fragment>}</P>
      </Section>
      <Section title={"Notifications"}>
        <P>{<Fragment key={3}>{"The feed at the top carries household notifications — items expiring soon, members joining, meal-plan nudges — scoped to your "}<strong key={0}>{"active"}</strong>{" household. Unread count shows as a badge on the "}<strong key={1}>{"People"}</strong>{" tab itself, so nothing gets missed. Expiry reminders include an "}<strong key={2}>{"Add to list"}</strong>{" action that drops a replacement onto your grocery list."}</Fragment>}</P>
        <P>{<Fragment key={4}>{"Tap the "}<strong key={0}>{"gear icon"}</strong>{" in the top bar to open "}<strong key={1}>{"Notification settings"}</strong>{". Each category has independent "}<strong key={2}>{"Push"}</strong>{" and "}<strong key={3}>{"Feed"}</strong>{" switches plus a labeled "}<strong key={4}>{"Snooze"}</strong>{" button that pauses it for a preset window — the global "}<strong key={5}>{"All notifications"}</strong>{" row carries the master Push opt-in and Snooze only, so a mute can never be permanent by accident. Push is opt-in per category; when a push arrives while the app is open it appears as a heads-up banner, and taps open the same place a feed tap would."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-notification-settings.png"} alt={"Per-category notification settings on mobile"} device="phone" />
        <P>{<Fragment key={5}>{"See "}<DocLink key={0} href="/docs/dashboard">{"Dashboard & Notifications"}</DocLink>{" for the notification categories and the push details."}</Fragment>}</P>
      </Section>
      <Section title={"Members"}>
        <P>{<Fragment key={6}>{"Below the feed: the household name (edit it and save with the inline icon) and the member list with roles. Invites are managed from here — see "}<DocLink key={0} href="/docs/households">{"Households & Members"}</DocLink>{" for roles and invite flow."}</Fragment>}</P>
      </Section>
      <Section title={"Your allergy records"}>
        <P>{<Fragment key={7}>{"The "}<strong key={0}>{"Allergies & dietary restrictions"}</strong>{" card on this tab holds your tri-state toggles — None / Dietary / Allergy per allergen. The records are yours, not the household's: they follow you into every household you belong to, and warnings across recipes, meal plans, events, and grocery lists fire for every member's records. See "}<DocLink key={1} href="/docs/allergy-tracking">{"Allergy Tracking"}</DocLink>{"."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-allergies.png"} alt={"The mobile allergy editor"} device="phone" />
        <P>{<Fragment key={8}>{"← Back to "}<DocLink key={0} href="/docs/mobile-app">{"Using the Mobile App"}</DocLink></Fragment>}</P>
      </Section>
    </DocPage>
  );
}
