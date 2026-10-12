import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  Callout,
  DocLink,
  DocList,
  DocSub,
  DocTable,
  P,
  Section,
  Screenshot,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Getting Started — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Getting Started"} intro={"LENA offers Sign in with Google plus OAuth sign-in with Discord, Microsoft, or Facebook — there's no separate password to remember."}>
      <Section title={"Signing in"}>
        <Screenshot src={"/images/login.png"} alt={"Web sign-in"} device="web" />
        <DocList items={[
          <Fragment key={0}><strong key={0}>{"Web:"}</strong>{" open the app URL and click your provider's sign-in button (OAuth buttons appear when the instance admin has configured that provider's app)."}</Fragment>,
          <Fragment key={1}><strong key={0}>{"Mobile:"}</strong>{" tap "}<strong key={1}>{"Sign in with Google"}</strong>{" on the login screen (Discord sign-in on mobile is planned)."}</Fragment>,
        ]} />
        <P>{<Fragment key={2}>{"LENA follows your device's light/dark setting by default. On web, flip modes with the sun/moon button in the header or choose "}<strong key={0}>{"Light / System / Dark"}</strong>{" under "}<strong key={1}>{"Profile → Appearance"}</strong>{"; on mobile the same three options live under "}<strong key={2}>{"More → Appearance"}</strong>{"."}</Fragment>}</P>
        <Screenshot src={"/images/login-dark.png"} alt={"The sign-in card in dark mode"} device="web" />
        <P>{<Fragment key={3}>{"The first time you sign in, LENA creates your account automatically."}</Fragment>}</P>
        <DocSub>{<Fragment key={4}>{"Access approval"}</Fragment>}</DocSub>
        <P>{<Fragment key={5}>{"On most deployments, a new account isn't active right away — it lands at an "}<strong key={0}>{"approval barrier"}</strong>{" while an administrator reviews the request. You'll see an \"Access request sent\" screen on both web and mobile; every active admin is notified and can approve or deny you from the Users page. Once approved, your next sign-in lands you straight in the app. If a request is denied, signing in shows an \"Access not granted\" screen instead. (Deployments can turn the barrier off — self-hosted instances with "}<code key={1}>{"LENA_REQUIRE_MEMBER_APPROVAL=false"}</code>{" admit everyone immediately, and the very first sign-in on a fresh install always bootstraps as admin so a new deployment can't lock itself out.)"}</Fragment>}</P>
        <Screenshot src={"/images/access-request-pending.png"} alt={"The access-request screen"} device="web" />
        <P>{<Fragment key={6}>{"Once signed in, you stay signed in — LENA keeps your session alive for up to 30 days and refreshes it in the background, so you don't have to re-authenticate every hour. Signing out ends the session on that device."}</Fragment>}</P>
        <DocSub>{<Fragment key={7}>{"Multiple sign-ins, one account"}</Fragment>}</DocSub>
        <P>{<Fragment key={8}>{"A LENA account can link more than one login — for example both a Google account and a Discord or Microsoft account. The account-linking API is in place and a settings UI for it is planned. LENA never merges accounts by matching email addresses: linking is always something you do deliberately while signed in, so a lookalike email can never take over your household."}</Fragment>}</P>
      </Section>
      <Section title={"Set up your profile"}>
        <P>{<Fragment key={9}>{"Go to "}<strong key={0}>{"Profile"}</strong>{" (web) and fill in:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={10}><strong key={0}>{"First name / Last name"}</strong>{" — shown to your household members."}</Fragment>,
          <Fragment key={11}><strong key={0}>{"Backup email"}</strong>{" — only used if your sign-in email ever fails."}</Fragment>,
          <Fragment key={12}><strong key={0}>{"Birthdate"}</strong>{" — optional; unlocks the "}<DocLink key={1} href="/docs/ai-assistant#age-gate-for-alcohol-features">{"wine pairing and cocktail suggestions"}</DocLink>{" (the server checks 21+)."}</Fragment>,
          <Fragment key={13}><strong key={0}>{"Appearance"}</strong>{" — Light / System / Dark for the web app (see "}<DocLink key={1} href="/docs/themes">{"Themes"}</DocLink>{")."}</Fragment>,
          <Fragment key={14}><strong key={0}>{"Allergies & dietary restrictions"}</strong>{" — your records, which drive the cooking warnings (see "}<DocLink key={1} href="/docs/allergy-tracking">{"Allergy Tracking"}</DocLink>{"); each household member keeps their own."}</Fragment>,
          <Fragment key={15}><strong key={0}>{"\"Let other users find me by name or email\""}</strong>{" — turn this "}<strong key={1}>{"on"}</strong>{" if you want family members to be able to search for you and send you a household invite. If it's off, nobody can invite you."}</Fragment>,
        ]} />
        <Callout>{<Fragment key={16}><strong key={0}>{"Tip:"}</strong>{" everyone who will share a household needs this toggle on to be found — at least until they've been invited."}</Fragment>}</Callout>
      </Section>
      <Section title={"Your household"}>
        <P>{<Fragment key={17}>{"Everything in LENA — recipes, meal plans, grocery lists, pantry stock, wine — belongs to a "}<strong key={0}>{"household"}</strong>{". When you create your account you're placed in a household of your own."}</Fragment>}</P>
        <P>{<Fragment key={18}>{"Two paths from here:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={19}><strong key={0}>{"You're starting fresh:"}</strong>{" keep your own household and invite your family. See "}<DocLink key={1} href="/docs/households">{"Households & Members"}</DocLink>{"."}</Fragment>,
          <Fragment key={20}><strong key={0}>{"You're joining someone else's household:"}</strong>{" ask them to invite you (make sure your discoverability toggle is on). The invite appears on your "}<strong key={1}>{"Dashboard"}</strong>{" and on the "}<strong key={2}>{"Household"}</strong>{" page — accept it there, and your meal plans, grocery lists, and events move with you into the shared household."}</Fragment>,
        ]} />
      </Section>
      <Section title={"Where things live"}>
        <P>{<Fragment key={21}>{"The web app's left sidebar is split into two zones. On a phone-sized screen the sidebar collapses into a hamburger menu, tables stack into cards, and dialogs go full-screen — the web app stays usable, though the mobile app remains the better phone experience (a one-time banner suggests it; dismiss it and it never comes back on that device)."}</Fragment>}</P>
        <P>{<Fragment key={22}>{"The top zone is your day-to-day kitchen work:"}</Fragment>}</P>
        <DocTable head={["Section","What you'll find"]} rows={[[<Fragment key={23}><strong key={0}>{"Dashboard"}</strong></Fragment>, <Fragment key={24}>{"Today's meals, recipe recommendations, incoming invites"}</Fragment>], [<Fragment key={25}><strong key={0}>{"Meal Planning"}</strong></Fragment>, <Fragment key={26}>{"Weekly Plan, Events, Grocery Lists"}</Fragment>], [<Fragment key={27}><strong key={0}>{"Recipes"}</strong></Fragment>, <Fragment key={28}>{"Your recipe collection and (admins) categories + pending reviews"}</Fragment>], [<Fragment key={29}><strong key={0}>{"Inventory"}</strong></Fragment>, <Fragment key={30}>{"The item catalog: items — plus (admins) brands, categories, ingredients, allergens, allergen suggestions, nutrition, and flavor profiles"}</Fragment>], [<Fragment key={31}><strong key={0}>{"Wine"}</strong></Fragment>, <Fragment key={32}>{"Bottles — plus (admins) the wine reference catalogs"}</Fragment>]]} />
        <P>{<Fragment key={33}>{"Pinned to the bottom of the sidebar are two captioned groups:"}</Fragment>}</P>
        <DocTable head={["Section","What you'll find"]} rows={[[<Fragment key={34}><strong key={0}>{"Administration"}</strong>{" "}<em key={1}>{"(admin)"}</em></Fragment>, <Fragment key={35}>{"Users, Pending Items"}</Fragment>], [<Fragment key={36}><strong key={0}>{"Account"}</strong></Fragment>, <Fragment key={37}>{"Household (members, invites, roles), Profile (name, backup email, discoverability)"}</Fragment>]]} />
        <P>{<Fragment key={38}>{"Items marked "}<em key={0}>{"(admin)"}</em>{" — the whole Administration group, Recipes' Categories and Pending Reviews children, and every catalog entry under Inventory and Wine except Items and Bottles — only appear for household admins. Standard members who visit an admin URL directly are shown a Forbidden notice instead."}</Fragment>}</P>
        <Screenshot src={"/images/admin-gate-member-nav.png"} alt={"A standard member's sidebar — only member destinations under each group"} device="web" />
        <Screenshot src={"/images/admin-gate-member-forbidden.png"} alt={"The Forbidden notice a member sees on an admin-only URL"} device="web" />
        <P>{<Fragment key={39}><strong key={0}>{"Ask Dot"}</strong>{", the free-form assistant that can look up your pantry, plans, recipes, and cellar, isn't in the sidebar — it's the ✨ link on the second row of the header (see "}<DocLink key={1} href="/docs/ai-assistant">{"AI Assistant"}</DocLink>{")."}</Fragment>}</P>
      </Section>
      <Section title={"Notifications"}>
        <P>{<Fragment key={40}>{"The "}<strong key={0}>{"bell icon"}</strong>{" in the web header shows an unread count. You'll be notified when someone in your household changes shared things — a meal plan update, a new grocery list, an event change, or an invite. Click a notification to jump straight to the thing that changed."}</Fragment>}</P>
        <P>{<Fragment key={41}>{"On mobile, the unread count rides on the "}<strong key={0}>{"Household"}</strong>{" tab badge."}</Fragment>}</P>
      </Section>
      <Section title={"First five things to do"}>
        <DocList ordered items={[
          <Fragment key={42}>{"Fill in your profile and turn on discoverability."}</Fragment>,
          <Fragment key={43}>{"Invite your household (or accept their invite)."}</Fragment>,
          <Fragment key={44}>{"Add a few recipes — or snap a photo of a recipe card and let the importer do the typing."}</Fragment>,
          <Fragment key={45}>{"Create this week's meal plan and generate a grocery list."}</Fragment>,
          <Fragment key={46}>{"Open the mobile app on your phone and try scanning a barcode in your pantry."}</Fragment>,
        ]} />
      </Section>
    </DocPage>
  );
}
