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
  title: "Grocery Lists — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Grocery Lists"} intro={"Grocery lists turn a meal plan into a shopping trip — generated automatically, checked off as you shop. No plan? A list works on its own too."}>
      <Section title={"Creating a list"}>
        <P>{<Fragment key={0}>{"Three ways:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={1}>{"From a "}<strong key={0}>{"weekly meal plan"}</strong>{" page — "}<strong key={1}>{"Generate Grocery List"}</strong>{" (web)"}</Fragment>,
          <Fragment key={2}>{"From the "}<strong key={0}>{"Grocery Lists"}</strong>{" page — "}<strong key={1}>{"Create"}</strong>{" and pick a plan from the dropdown (web and mobile both offer the same picker)"}</Fragment>,
          <Fragment key={3}>{"From the "}<strong key={0}>{"Grocery Lists"}</strong>{" page — "}<strong key={1}>{"Create"}</strong>{" and leave the default "}<strong key={2}>{"Manual list (no meal plan)"}</strong>{" selected"}</Fragment>,
        ]} />
        <P>{<Fragment key={4}>{"A manual list starts empty — type staples, a hardware run, whatever isn't food on a plan. Manual and generated lists sit side by side on the index, labeled by their source plan's name or \"Manual list\"."}</Fragment>}</P>
        <Screenshot src={"/images/grocery-create-dialog.png"} alt={"The create dialog — pick a plan or stay on the default manual option"} device="web" />
        <P>{<Fragment key={5}>{"Generating for a plan that already has a list "}<strong key={0}>{"updates that list in place"}</strong>{" — the generated lines are recomputed against the current plan and pantry stock, and anything you added manually stays. You won't end up with two lists for the same week. Your store layout and manual ordering also survive regeneration — they live outside the list's rows."}</Fragment>}</P>
        <Screenshot src={"/images/grocery-lists.png"} alt={"The Grocery Lists index — manual and plan-sourced lists side by side"} device="web" />
        <P>{<Fragment key={6}>{"Each line keeps a "}<em key={0}>{"source"}</em>{" note — whether it came from the meal plan, a restock suggestion you tapped "}<strong key={1}>{"Add"}</strong>{" on, or something you typed in yourself — but the list's grouping comes from store routing (below), not the source."}</Fragment>}</P>
        <Screenshot src={"/images/grocery-list.png"} alt={"A grocery list routed through a store — aisle groups with the store picker and Edit Aisles button"} device="web" />
      </Section>
      <Section title={"Store routing"}>
        <P>{<Fragment key={7}>{"Pick a "}<strong key={0}>{"store"}</strong>{" in the header and the list groups itself into that store's aisles, in walk order — the route you'd naturally take through the store. Items with no aisle land in "}<strong key={1}>{"Other items"}</strong>{" at the bottom."}</Fragment>}</P>
        <DocList items={[
          <Fragment key={8}><strong key={0}>{"Edit aisles"}</strong>{" (web) — name and order the store's aisles to match how you actually walk it."}</Fragment>,
          <Fragment key={9}><strong key={0}>{"Rearrange"}</strong>{" — web: drag the handle on the left of a row; mobile: drag within an aisle, or use the row's menu to move between aisles. Dropping an item into a different aisle's group assigns it there. Your arrangement is saved on the server and survives list regeneration — the phone shows the same order as the web."}</Fragment>,
          <Fragment key={10}><strong key={0}>{"Learning"}</strong>{" — the order you check items off is remembered per store, so over time the generated order converges on your real route. Items the system has only "}<em key={1}>{"guessed"}</em>{" an aisle for show a \"suggested aisle\" hint; dragging or assigning them makes it permanent."}</Fragment>,
        ]} />
        <Screenshot src={"/images/grocery-store-picker.png"} alt={"The store picker — which layout to route through"} device="web" />
        <Screenshot src={"/images/grocery-edit-aisles.png"} alt={"The aisle layout editor — rename, reorder, add, delete"} device="web" />
        <Screenshot src={"/images/grocery-move-aisle.png"} alt={"Moving an item to a different aisle from its row menu"} device="web" />
        <P>{<Fragment key={11}>{"New lists inherit the store your household used most recently — set it once and routing just applies."}</Fragment>}</P>
      </Section>
      <Section title={"Time to restock"}>
        <P>{<Fragment key={12}>{"Below the list proper, the "}<strong key={0}>{"Time to restock"}</strong>{" card proposes pantry items that are at or under their "}<DocLink key={1} href="/docs/pantry">{"minimum quantity"}</DocLink>{", ranked by how often your household uses them. Tap "}<strong key={2}>{"Add"}</strong>{" to drop one onto the list — it lands in the unassigned group, and since it's now on the list it stops being suggested."}</Fragment>}</P>
        <P>{<Fragment key={13}>{"Items you never engage with don't clutter the section: a suggestion needs both low stock "}<em key={0}>{"and"}</em>{" household usage signals."}</Fragment>}</P>
      </Section>
      <Section title={"Shopping with the list"}>
        <P>{<Fragment key={14}>{"Open the list and check items off as you go — each check saves immediately, so if your partner is holding the same list on their phone, they see the check appear."}</Fragment>}</P>
        <DocList items={[
          <Fragment key={15}><strong key={0}>{"Check/uncheck"}</strong>{" — tap the checkbox"}</Fragment>,
          <Fragment key={16}><strong key={0}>{"Delete a line"}</strong>{" — trash icon, for things you decided not to buy"}</Fragment>,
          <Fragment key={17}><strong key={0}>{"Add a manual item"}</strong>{" — name, quantity, and unit; lands in the unassigned group until you give it an aisle"}</Fragment>,
        ]} />
        <P>{<Fragment key={18}>{"If you're both shopping at once, keep the list open — checks sync in real time."}</Fragment>}</P>
      </Section>
      <Section title={"Shop with Instacart"}>
        <P>{<Fragment key={19}>{"When the server has a shopping provider configured, a "}<strong key={0}>{"Shop with Instacart"}</strong>{" button appears in the list header. Tap it and LENA pushes the list's "}<em key={1}>{"unchecked"}</em>{" lines to Instacart, then shows you a shareable link — open it in a new tab to pick a store and check out on Instacart's site, or copy it to send to whoever's doing the shop. Nothing is silently added to a cart: the link opens a shopping list on the provider's side, and the purchase decision stays yours."}</Fragment>}</P>
        <Screenshot src={"/images/grocery-list-shop.png"} alt={"A grocery list with the Shop with Instacart button in the header"} device="web" />
        <P>{<Fragment key={20}>{"Lines named by a catalog item or remembered usual brand carry UPCs and a brand filter, so the link lands on the right product more often than not. Checked lines are left off the link by default — if your list had any when you pressed the button, the dialog says so."}</Fragment>}</P>
        <Screenshot src={"/images/grocery-shop-link.png"} alt={"The shopping-link dialog with copy and open actions"} device="web" />
        <P>{<Fragment key={21}>{"The button only renders when "}<code key={0}>{"shopperProviders"}</code>{" reports a configured provider (server-side "}<code key={1}>{"LENA_INSTACART_API_KEY"}</code>{"), so deployments without a key never show a dead action."}</Fragment>}</P>
      </Section>
      <Section title={"Ingredients and usual brands"}>
        <P>{<Fragment key={22}>{"Lines that came from "}<DocLink key={0} href="/docs/ingredients">{"ingredient-keyed recipe lines"}</DocLink>{" display the ingredient name rather than a product — "}<em key={1}>{"garlic"}</em>{" instead of "}<em key={2}>{"Ahold Garlic Salt"}</em>{". Lines with a remembered brand show a \"usually buys …\" caption so you know which product to grab."}</Fragment>}</P>
        <P>{<Fragment key={23}>{"The first time you check off an ingredient-only line, LENA asks "}<strong key={0}>{"which brand you bought"}</strong>{" — pick it and the pantry is credited with that item, and it becomes your household's usual brand for that ingredient. After that, the line checks straight off. You can also check off without a brand to stay generic."}</Fragment>}</P>
        <Screenshot src={"/images/grocery-brand-picker.png"} alt={"The brand picker on first check-off of an ingredient line"} device="web" />
        <P>{<Fragment key={24}>{"Lines that conflict with a household member's "}<DocLink key={0} href="/docs/allergy-tracking">{"allergy records"}</DocLink>{" carry a warning chip too — check it before something risky ends up in the cart."}</Fragment>}</P>
      </Section>
      <Section title={"What's on the list vs. in the pantry"}>
        <P>{<Fragment key={25}>{"The generator already subtracts your "}<DocLink key={0} href="/docs/pantry">{"pantry stock"}</DocLink>{" from each recipe's needs — if the plan needs 2 lb of potatoes and you have 2 lb on hand, the line never appears. So the more honest your pantry counts are, the shorter and more accurate your list gets. Stock recorded in a different unit is converted when the units are compatible; when they aren't, the full amount stays on the list."}</Fragment>}</P>
      </Section>
      <Section title={"After the trip"}>
        <P>{<Fragment key={26}>{"Checked items stay checked as a record of the trip. When you unpack, open the mobile app's "}<strong key={0}>{"Scan"}</strong>{" tab and scan barcodes to add purchases to pantry stock — that's what keeps the stock subtraction accurate next week."}</Fragment>}</P>
      </Section>
    </DocPage>
  );
}
