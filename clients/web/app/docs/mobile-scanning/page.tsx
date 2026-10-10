import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  DocLink,
  DocList,
  P,
  Screenshot,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Barcode Scanning — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Barcode Scanning"} intro={"The Scan tab is the mobile app's superpower — it turns the phone's camera into a pantry scanner."}>
        <P>{<Fragment key={0}>{"The camera permission prompt waits until you open this tab for the first time — it never fires at app launch."}</Fragment>}</P>
        <Screenshot src={"/images/mobile-scan.png"} alt={"The scan screen"} device="phone" />
        <DocList ordered items={[
          <Fragment key={1}>{"Point the camera at a product barcode."}</Fragment>,
          <Fragment key={2}>{"LENA looks the item up by UPC — 12-, 13-, and 14-digit codes are normalized to GTIN-14 before lookup."}</Fragment>,
          <Fragment key={3}><strong key={0}>{"Found?"}</strong>{" Enter a quantity and tap add or remove — the pantry count updates instantly. The result card shows the item's resolved "}<DocLink key={1} href="/docs/ingredients">{"ingredient"}</DocLink>{" when one is linked, marked "}<em key={2}>{"(household)"}</em>{" when your household has overridden the catalog link."}</Fragment>,
          <Fragment key={4}><strong key={0}>{"Not found?"}</strong>{" Fill in the item's name and details to submit it — it goes to the "}<strong key={1}>{"Pending Items"}</strong>{" review queue on the web and joins the catalog once approved."}</Fragment>,
          <Fragment key={5}><strong key={0}>{"Found but unlinked?"}</strong>{" LENA offers to pick the generic ingredient the item belongs to, so future recipe lines and grocery lists resolve it."}</Fragment>,
        ]} />
        <Screenshot src={"/images/mobile-scan-found.png"} alt={"Found item — quantity adjust and add/remove"} device="phone" />
        <Screenshot src={"/images/mobile-scan-added.png"} alt={"After adding — the confirmation stays on the card"} device="phone" />
        <Screenshot src={"/images/mobile-scan-unlinked.png"} alt={"Unlinked item offering the Link ingredient flow"} device="phone" />
        <Screenshot src={"/images/mobile-scan-link-ingredient.png"} alt={"The ingredient search — pick what the item generically is"} device="phone" />
        <Screenshot src={"/images/mobile-scan-submit.png"} alt={"Unknown UPC — the submit-for-approval form"} device="phone" />
        <Screenshot src={"/images/mobile-scan-dark.png"} alt={"The scan screen in dark mode"} device="phone" />
        <P>{<Fragment key={6}>{"Use it when you unpack groceries, when you finish a product, or to check whether you already own something while standing in the store aisle."}</Fragment>}</P>
        <P>{<Fragment key={7}>{"Scans feed the same pantry counts you see on the "}<DocLink key={0} href="/docs/mobile-pantry">{"Pantry tab"}</DocLink>{", and items below their minimum automatically land on the next "}<DocLink key={1} href="/docs/mobile-grocery">{"grocery list"}</DocLink>{"."}</Fragment>}</P>
        <P>{<Fragment key={8}>{"← Back to "}<DocLink key={0} href="/docs/mobile-app">{"Using the Mobile App"}</DocLink></Fragment>}</P>
    </DocPage>
  );
}
