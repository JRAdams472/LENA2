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
  title: "Your Dashboard — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Your Dashboard"} intro={"The Dashboard is the first thing you see when you open the web app — it's your \"what's happening today\" page."}>
      <Section title={"Today's meals"}>
        <P>{<Fragment key={0}>{"If you have an active weekly meal plan (one whose week contains today), the Dashboard shows today's "}<strong key={0}>{"Breakfast, Lunch, and Dinner"}</strong>{" slots with the recipe you've assigned to each. It's the quickest way to answer \"what's for dinner?\""}</Fragment>}</P>
        <Screenshot src={"/images/dashboard.png"} alt={"The Dashboard — today's meal slots and suggestions"} device="web" />
        <Screenshot src={"/images/dashboard-dark.png"} alt={"The Dashboard in dark mode"} device="web" />
        <P>{<Fragment key={1}>{"If nothing is planned for today, each empty slot says so and offers a "}<strong key={0}>{"Plan it →"}</strong>{" shortcut — or open "}<strong key={1}>{"Meal Planning → Weekly Plan"}</strong>{" and fill in the week."}</Fragment>}</P>
      </Section>
      <Section title={"Household invitations"}>
        <P>{<Fragment key={2}>{"If someone has invited you to join their household, the invite appears at the top of the Dashboard with "}<strong key={0}>{"Accept / Decline"}</strong>{" buttons. You don't have to go hunting for it."}</Fragment>}</P>
      </Section>
      <Section title={"Recipe recommendations"}>
        <P>{<Fragment key={3}>{"The "}<strong key={0}>{"Delicious ideas for tonight"}</strong>{" card suggests recipes you might want to cook, each with a reason chip:"}</Fragment>}</P>
        <DocTable head={["Reason shown","What it means"]} rows={[[<Fragment key={4}><strong key={0}>{"Similar to your menu"}</strong></Fragment>, <Fragment key={5}>{"Shares ingredients with things you're already planning to cook — a good way to use up what you buy"}</Fragment>], [<Fragment key={6}><strong key={0}>{"Due for a revisit"}</strong></Fragment>, <Fragment key={7}>{"A recipe you rated well that you haven't cooked in a while"}</Fragment>], [<Fragment key={8}><strong key={0}>{"Matches your household's tastes"}</strong></Fragment>, <Fragment key={9}>{"Belongs to the categories (cuisine, dish type, course…) your household reaches for most"}</Fragment>], [<Fragment key={10}><strong key={0}>{"Trending in your household"}</strong></Fragment>, <Fragment key={11}>{"Been on your menus a lot recently compared to its overall history"}</Fragment>], [<Fragment key={12}><strong key={0}>{"Recommended for you"}</strong></Fragment>, <Fragment key={13}>{"A suggestion based on what your household tends to enjoy"}</Fragment>]]} />
        <P>{<Fragment key={14}>{"Suggestions get better the more you use LENA — ratings, meal plans, and recipe picks all feed them (see "}<DocLink key={0} href="/docs/analytics">{"Analytics & Recommendations"}</DocLink>{")."}</Fragment>}</P>
      </Section>
      <Section title={"Time to restock"}>
        <P>{<Fragment key={15}>{"Next to the suggestions, the "}<strong key={0}>{"Time to restock"}</strong>{" card lists pantry items that are at or under the "}<DocLink key={1} href="/docs/pantry">{"minimum quantity"}</DocLink>{" you set — ranked by how often your household actually uses them, so the staples you reach for daily surface first. It links straight to your grocery lists, where the same items appear under a "}<strong key={2}>{"Time to restock"}</strong>{" section with one-tap "}<strong key={3}>{"Add"}</strong>{" buttons."}</Fragment>}</P>
        <P>{<Fragment key={16}>{"Items already on your latest list don't get suggested again — LENA knows you're already planning to buy them."}</Fragment>}</P>
      </Section>
      <Section title={"The notification bell"}>
        <P>{<Fragment key={17}>{"The bell in the top-right of the web app shows how many household notifications are unread. LENA notifies the "}<em key={0}>{"other"}</em>{" members of your household when shared data changes — for example:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={18}>{"A meal plan slot is added, changed, or removed"}</Fragment>,
          <Fragment key={19}>{"A grocery list is generated"}</Fragment>,
          <Fragment key={20}>{"An event is created, updated, or its dishes change"}</Fragment>,
          <Fragment key={21}>{"A household invite is sent or accepted"}</Fragment>,
        ]} />
        <P>{<Fragment key={22}>{"Click the bell to see recent notifications, click one to "}<strong key={0}>{"jump directly to the item"}</strong>{" (opening the recipe, event, or pantry in question), and use "}<strong key={1}>{"mark all read"}</strong>{" to clear the count. The menu's footer links to "}<strong key={2}>{"Notification settings"}</strong>{"."}</Fragment>}</P>
        <Screenshot src={"/images/notification-bell.png"} alt={"The notification bell menu, including an expiring-item reminder"} device="web" />
        <Callout>{<Fragment key={23}>{"The bell quietly checks for new notifications about every 30 seconds, so updates from your family appear without refreshing the page."}</Fragment>}</Callout>
        <P>{<Fragment key={24}>{"On "}<strong key={0}>{"mobile"}</strong>{", the unread count appears as a badge on the People tab, and the feed there shows the same reminders — tapping one opens the recipe, pantry, or event it refers to."}</Fragment>}</P>
      </Section>
      <Section title={"Scheduled reminders"}>
        <P>{<Fragment key={25}>{"Beyond household activity, LENA checks about once an hour and sends reminders to "}<strong key={0}>{"every member of the household"}</strong>{":"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={26}><strong key={0}>{"Protein defrost"}</strong>{" — if a meal plan slot calls for a recipe with protein ingredients, LENA reminds you to start defrosting. Up to 8 lbs means 48 hours ahead; heavier items add 24 hours per extra 4 lbs (10 lbs → 3 days, 20 lbs → 5 days). Which inventory categories count as protein is set by an admin under "}<strong key={1}>{"Inventory → Categories"}</strong>{" (the "}<em key={2}>{"Protein"}</em>{" column)."}</Fragment>,
          <Fragment key={27}><strong key={0}>{"Multi-day prep"}</strong>{" — recipes with a step that takes 24 hours or longer (marinades, ferments, slow cures) get a reminder early enough to start that step."}</Fragment>,
          <Fragment key={28}><strong key={0}>{"Expiring pantry items"}</strong>{" — items whose expiry date is within a few days get a reminder. Its "}<strong key={1}>{"Add to list"}</strong>{" action drops the item onto your current grocery list as a replacement so you remember to restock."}</Fragment>,
        ]} />
        <Callout>{<Fragment key={29}>{"Expiry reminders need an expiry date on the pantry item — set "}<em key={0}>{"Expiry Date"}</em>{" when editing an item on the Inventory Items page."}</Fragment>}</Callout>
      </Section>
      <Section title={"Notification settings"}>
        <P>{<Fragment key={30}>{"Open "}<strong key={0}>{"Notification settings"}</strong>{" from the bell menu (web) or the gear icon in the mobile People tab's top bar to control what reaches you:"}</Fragment>}</P>
        <Screenshot src={"/images/notification-settings.png"} alt={"Per-category notification preferences with mutes"} device="web" />
        <DocList items={[
          <Fragment key={31}>{"Each "}<strong key={0}>{"category"}</strong>{" — Household activity, Food events, Meal reminders, Expiring pantry items — can be turned off entirely with its switch, or "}<strong key={1}>{"muted"}</strong>{" for a preset window (1 hour, 8 hours, 1 day, 1 week). The mute control is labeled — "}<strong key={2}>{"Mute"}</strong>{" on web, "}<strong key={3}>{"Snooze"}</strong>{" on mobile."}</Fragment>,
          <Fragment key={32}><strong key={0}>{"All notifications (global mute)"}</strong>{" pauses "}<em key={1}>{"everything"}</em>{" for a chosen window — muting is the only option here, so muting can never be permanent by accident."}</Fragment>,
          <Fragment key={33}>{"An active mute shows "}<em key={0}>{"Muted until …"}</em>{" and can be cleared early."}</Fragment>,
        ]} />
        <P>{<Fragment key={34}>{"Opting out affects only you — other household members keep receiving their own reminders."}</Fragment>}</P>
        <DocSub>{<Fragment key={35}>{"Push notifications (mobile)"}</Fragment>}</DocSub>
        <P>{<Fragment key={36}>{"Each category also carries an independent "}<strong key={0}>{"Push"}</strong>{" switch — off by default — so a category can reach your phone's notification shade even when its feed row is hidden (or vice versa). The "}<strong key={1}>{"All notifications"}</strong>{" row's Push switch is the master opt-in; muting still silences push and feed together. The mobile app asks for the Android notification permission the first time you sign in, and tapping a push opens the same place a feed tap would — the recipe, the pantry, or the event."}</Fragment>}</P>
        <Callout>{<Fragment key={37}>{"Push delivery needs a Firebase-backed server deployment ("}<code key={0}>{"LENA_PUSH_PROVIDER=fcm"}</code>{"); without credentials the server logs deliveries instead and nothing reaches the phone."}</Fragment>}</Callout>
      </Section>
    </DocPage>
  );
}
