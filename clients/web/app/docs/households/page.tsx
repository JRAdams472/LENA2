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
  title: "Households & Members — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Households & Members"} intro={"A household is the shared space everything in LENA lives in. All recipes, meal plans, grocery lists, pantry stock, events, and wine belong to the household — not to individual people — so everyone sees and works with the same data."}>
      <Section title={"The Household page"}>
        <P>{<Fragment key={0}>{"Open "}<strong key={0}>{"Household"}</strong>{" in the sidebar (web) or tap the "}<strong key={1}>{"People"}</strong>{" tab (mobile) to see:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={1}>{"Everyone in the active household and their "}<strong key={0}>{"role"}</strong></Fragment>,
          <Fragment key={2}>{"Pending invites you've sent and received"}</Fragment>,
          <Fragment key={3}>{"The user search box for inviting new members"}</Fragment>,
          <Fragment key={4}>{"Your allergy/dietary records (mobile)"}</Fragment>,
        ]} />
        <Screenshot src={"/images/household.png"} alt={"The Household page — your households, members, and invites"} device="web" />
        <Screenshot src={"/images/household-dark.png"} alt={"The Household page in dark mode"} device="web" />
      </Section>
      <Section title={"Belonging to more than one household"}>
        <P>{<Fragment key={5}>{"One account can be a member of several households — your own home, a shared vacation place, a partner's family. At any moment one of them is your "}<strong key={0}>{"active household"}</strong>{": it's the one your pantry, meal plans, grocery lists, events, cellar, and notification feed show."}</Fragment>}</P>
        <P>{<Fragment key={6}>{"The "}<strong key={0}>{"Your households"}</strong>{" card on the web Household page (and the switcher sheet behind the active-household header on mobile) lists every household you belong to with its member count and your role. From there you can:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={7}><strong key={0}>{"Switch"}</strong>{" — make another household active; the whole app follows it"}</Fragment>,
          <Fragment key={8}><strong key={0}>{"Leave"}</strong>{" — drop that one membership (the owner of a shared household passes ownership to the earliest remaining member; leaving your active household falls back to your earliest other membership, or a fresh household of your own)"}</Fragment>,
          <Fragment key={9}><strong key={0}>{"New household"}</strong>{" — create an additional household (optionally named) that starts empty and becomes active"}</Fragment>,
        ]} />
        <Screenshot src={"/images/household-create.png"} alt={"Creating another household"} device="web" />
        <P>{<Fragment key={10}>{"Notifications follow the active household — switching to the vacation house means the feed shows that household's activity, not your home's. Invites addressed to you always arrive, whichever household is active."}</Fragment>}</P>
      </Section>
      <Section title={"Inviting someone"}>
        <DocList ordered items={[
          <Fragment key={11}>{"Ask them to sign in to LENA once and turn on "}<strong key={0}>{"\"Let other users find me by name or email\""}</strong>{" on their Profile page."}</Fragment>,
          <Fragment key={12}>{"On the Household page, search for them by name or email."}</Fragment>,
          <Fragment key={13}>{"Send the invite — it shows up on their Household page and in their notifications."}</Fragment>,
          <Fragment key={14}>{"When they "}<strong key={0}>{"accept"}</strong>{", they join your household and it becomes their active one."}</Fragment>,
        ]} />
        <DocSub>{<Fragment key={15}>{"What happens when someone accepts"}</Fragment>}</DocSub>
        <P>{<Fragment key={16}>{"Accepting "}<strong key={0}>{"adds a membership"}</strong>{" — it never takes anything away. If the new member is the sole owner of another household, accept offers a choice:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={17}><strong key={0}>{"Join and merge"}</strong>{" — their old household's pantry, cellar, meal plans, grocery lists, and events move into yours, and the empty household is deleted"}</Fragment>,
          <Fragment key={18}><strong key={0}>{"Just join"}</strong>{" — they keep both households and can switch between them"}</Fragment>,
        ]} />
        <Screenshot src={"/images/household-accept-merge.png"} alt={"Choosing whether to merge on accept"} device="web" />
        <P>{<Fragment key={19}>{"Merge is only offered when they alone own the source household — a shared household can never be dissolved out from under its other members. The other members get a notification that someone joined either way."}</Fragment>}</P>
      </Section>
      <Section title={"Roles"}>
        <P>{<Fragment key={20}>{"Members carry roles (shown as chips next to their names), and a role is per household — you can be the owner of one household and a regular member of another. Regular members can plan, cook, shop, and edit shared data. Admin-only surfaces exist for the accounts designated as admins — managing users, reviewing pending items and recipe imports, and curating every shared reference catalog (brands, categories, ingredients, allergens, nutrition and flavor data, recipe taxonomy, and the wine tables). Those pages are web-only, hidden from members' navigation, and return a Forbidden notice on direct access."}</Fragment>}</P>
        <P>{<Fragment key={21}>{"Admins get a "}<strong key={0}>{"Users"}</strong>{" page under "}<strong key={1}>{"Administration"}</strong>{": per-user role assignment (member ↔ admin), account status, last sign-in, and ban controls — an admin's own row can't be banned."}</Fragment>}</P>
        <P>{<Fragment key={22}>{"When the access-approval barrier is on, the same page opens with an "}<strong key={0}>{"Access requests"}</strong>{" queue: every new sign-in waits there as "}<code key={1}>{"pending"}</code>{" (a matching chip shows in the Status column) until an admin approves or denies it. Denied users can be approved later from the main table — a denial is a hold, not a deletion. Each new request also lands in the notification bell for every admin."}</Fragment>}</P>
        <Screenshot src={"/images/users-access-requests.png"} alt={"The Users admin page — access requests, roles, status, and bans"} device="web" />
      </Section>
      <Section title={"Allergies are per member"}>
        <P>{<Fragment key={23}>{"Each member records their own "}<DocLink key={0} href="/docs/allergy-tracking">{"allergies and dietary restrictions"}</DocLink>{" on the Household page — warnings name the person they protect, so a flag on dinner is never anonymous. Records follow the "}<em key={1}>{"person"}</em>{": if someone belongs to two households, their allergies warn in both."}</Fragment>}</P>
      </Section>
      <Section title={"Notifications between members"}>
        <P>{<Fragment key={24}>{"When one member changes something shared, the "}<em key={0}>{"other members of that household"}</em>{" get a notification — so if you regenerate the grocery list, your partner sees it instead of being surprised by it at the store."}</Fragment>}</P>
      </Section>
      <Section title={"Leaving and merging"}>
        <P>{<Fragment key={25}>{"Households are meant to be long-lived, but people move. Use "}<strong key={0}>{"Leave"}</strong>{" on the Your households card (or the mobile switcher sheet) to drop a single membership — the rest of your households are untouched. If your household situation consolidates (two homes become one), the invite + merge flow is the tool: invite everyone to the household you want to keep, and those who solely own their old household can merge its contents in."}</Fragment>}</P>
      </Section>
    </DocPage>
  );
}
