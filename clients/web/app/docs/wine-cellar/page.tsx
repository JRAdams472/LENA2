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
  title: "Wine Cellar — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Wine Cellar"} intro={"The Wine section tracks your cellar: what bottles you have, how many of each, and their character."}>
      <Section title={"Bottles (web)"}>
        <P>{<Fragment key={0}><strong key={0}>{"Wine → Bottles"}</strong>{" lists your cellar. Each bottle entry records:"}</Fragment>}</P>
        <DocList items={[
          <Fragment key={1}><strong key={0}>{"Identity"}</strong>{" — type, country, region, vintage year, vineyard"}</Fragment>,
          <Fragment key={2}><strong key={0}>{"Character"}</strong>{" — ABV, acidity, tannin level, body, sweetness, oak integration"}</Fragment>,
          <Fragment key={3}><strong key={0}>{"Logistics"}</strong>{" — bottle size, "}<strong key={1}>{"quantity"}</strong>{" on hand, purchase date, bottle number"}</Fragment>,
        ]} />
        <P>{<Fragment key={4}>{"Adding a bottle ties it to the reference catalogs below — so pick (or create) the region, vintage, and grape varieties it belongs to."}</Fragment>}</P>
        <Screenshot src={"/images/wine-bottles.png"} alt={"Web — the Bottles page with catalog filters and quantity on hand"} device="web" />
      </Section>
      <Section title={"Reference catalogs (web)"}>
        <P>{<Fragment key={5}>{"The other Wine pages are the shared vocabulary bottles are described with — admin-managed on the web:"}</Fragment>}</P>
        <DocTable head={["Page","Use"]} rows={[[<Fragment key={6}><strong key={0}>{"Countries / Regions"}</strong></Fragment>, <Fragment key={7}>{"Where the wine is from"}</Fragment>], [<Fragment key={8}><strong key={0}>{"Types"}</strong></Fragment>, <Fragment key={9}>{"Red, white, sparkling, dessert…"}</Fragment>], [<Fragment key={10}><strong key={0}>{"Vintages"}</strong></Fragment>, <Fragment key={11}>{"Vintage years"}</Fragment>], [<Fragment key={12}><strong key={0}>{"Grape Varieties"}</strong></Fragment>, <Fragment key={13}>{"The grapes in the bottle"}</Fragment>], [<Fragment key={14}><strong key={0}>{"Wine Flavor Profiles"}</strong></Fragment>, <Fragment key={15}>{"Flavor descriptors for pairing"}</Fragment>]]} />
        <Screenshot src={"/images/wine-types.png"} alt={"Web — wine types"} device="web" />
        <Screenshot src={"/images/wine-countries.png"} alt={"Web — countries"} device="web" />
        <Screenshot src={"/images/wine-regions.png"} alt={"Web — regions within each country"} device="web" />
        <Screenshot src={"/images/wine-vintages.png"} alt={"Web — vintages"} device="web" />
        <Screenshot src={"/images/wine-grape-varieties.png"} alt={"Web — grape varieties"} device="web" />
        <Screenshot src={"/images/wine-flavor-profiles.png"} alt={"Web — wine flavor profiles used for pairing"} device="web" />
      </Section>
      <Section title={"On mobile"}>
        <P>{<Fragment key={16}>{"The mobile app's "}<strong key={0}>{"More → Wine"}</strong>{" cellar covers the day-to-day: browsing bottles and adjusting quantities right there in the cellar. See "}<DocLink key={1} href="/docs/mobile-wine">{"Wine Cellar on Mobile"}</DocLink>{"."}</Fragment>}</P>
      </Section>
      <Section title={"Pairings"}>
        <P>{<Fragment key={17}>{"With a "}<DocLink key={0} href="/docs/privacy#age-gate-for-alcohol-features">{"birthdate on your profile"}</DocLink>{" and an AI provider configured, "}<strong key={1}>{"Suggest pairings"}</strong>{" on a recipe page recommends bottles from your cellar (marked "}<em key={2}>{"In your cellar"}</em>{") plus style picks — see "}<DocLink key={3} href="/docs/ai-assistant">{"AI Assistant"}</DocLink>{". Flavor profiles on wines and foods inform those suggestions."}</Fragment>}</P>
      </Section>
    </DocPage>
  );
}
