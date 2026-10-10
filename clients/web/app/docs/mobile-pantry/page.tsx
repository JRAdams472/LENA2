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
  title: "Pantry on Mobile — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Pantry on Mobile"} intro={"The Pantry tab lists your stock with current quantity and the minimum you've set — items below minimum land on grocery lists automatically."}>
        <Screenshot src={"/images/mobile-pantry.png"} alt={"Pantry list"} device="phone" />
        <Screenshot src={"/images/mobile-pantry-dark.png"} alt={"The pantry in dark mode"} device="phone" />
        <DocList items={[
          <Fragment key={0}><strong key={0}>{"Search"}</strong>{" the field at the top to narrow the list."}</Fragment>,
          <Fragment key={1}><strong key={0}>{"Tap an item"}</strong>{" to edit its quantity, minimum, or notes."}</Fragment>,
          <Fragment key={2}><strong key={0}>{"Pull down"}</strong>{" to refresh."}</Fragment>,
        ]} />
        <P>{<Fragment key={3}>{"Fastest way to keep this list honest is the "}<DocLink key={0} href="/docs/mobile-scanning">{"Scan tab"}</DocLink>{" — scan items in when they arrive and out when they're empty. See "}<DocLink key={1} href="/docs/pantry">{"Inventory & Pantry"}</DocLink>{" for the full pantry model."}</Fragment>}</P>
        <P>{<Fragment key={4}>{"← Back to "}<DocLink key={0} href="/docs/mobile-app">{"Using the Mobile App"}</DocLink></Fragment>}</P>
    </DocPage>
  );
}
