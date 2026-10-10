import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  DocList,
  P,
  Section,
  Screenshot,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Themes — LENA docs",
};

export default function Page() {
  return (
    <DocPage title={"Themes"} intro={"LENA has full light and dark themes on web and mobile — warm cream surfaces with sage accents in light, warm-charcoal surfaces with lifted sage accents and hairline borders in dark."}>
      <Section title={"Switching themes"}>
        <DocList items={[
          <Fragment key={0}><strong key={0}>{"Web"}</strong>{" — the moon icon in the header toggles light/dark instantly. "}<strong key={1}>{"Profile → Appearance"}</strong>{" offers the three-way choice: "}<strong key={2}>{"Light"}</strong>{", "}<strong key={3}>{"System"}</strong>{" (follow your device setting), or "}<strong key={4}>{"Dark"}</strong>{". The choice persists per browser."}</Fragment>,
          <Fragment key={1}><strong key={0}>{"Mobile"}</strong>{" — "}<strong key={1}>{"More → Appearance"}</strong>{" gives the same Light / System / Dark choice."}</Fragment>,
        ]} />
        <Screenshot src={"/images/profile.png"} alt={"The Appearance picker on the Profile page"} device="web" />
      </Section>
      <Section title={"Gallery — dark mode"}>
        <P>{<Fragment key={2}>{"<table> <tr> <td><img src=\"images/dashboard-dark.png\" alt=\"Dashboard — dark\"><br>Dashboard</td> <td><img src=\"images/assistant-dark.png\" alt=\"Ask Dot — dark\"><br>Ask Dot</td> </tr> <tr> <td><img src=\"images/grocery-list-dark.png\" alt=\"Aisle-grouped grocery list — dark\"><br>Grocery list</td> <td><img src=\"images/event-timeline-dark.png\" alt=\"Event cooking timeline — dark\"><br>Event timeline</td> </tr> <tr> <td><img src=\"images/recipes-dark.png\" alt=\"Recipes — dark\"><br>Recipes</td> <td><img src=\"images/meal-plan-week-dark.png\" alt=\"Meal plan week — dark\"><br>Meal plan</td> </tr> <tr> <td><img src=\"images/mobile-dashboard-dark.png\" alt=\"Mobile dashboard — dark\"><br>Mobile dashboard</td> <td><img src=\"images/mobile-scan-dark.png\" alt=\"Mobile scanner — dark\"><br>Mobile scanner</td> </tr> <tr> <td><img src=\"images/mobile-grocery-list-dark.png\" alt=\"Mobile grocery list — dark\"><br>Mobile grocery list</td> <td><img src=\"images/mobile-assistant-dark.png\" alt=\"Mobile Ask Dot — dark\"><br>Mobile Ask Dot</td> </tr> <tr> <td><img src=\"images/login-dark.png\" alt=\"Sign-in — dark\"><br>Sign-in</td> <td><img src=\"images/notification-settings-dark.png\" alt=\"Notification settings — dark\"><br>Notification settings</td> </tr> </table>"}</Fragment>}</P>
      </Section>
      <Section title={"Notes"}>
        <DocList items={[
          <Fragment key={3}>{"Dark mode isn't an inverted palette — shadows are swapped for subtle borders so lists and dialogs stay readable."}</Fragment>,
          <Fragment key={4}>{"The scanner and camera viewfinder are already dark surfaces; the dark theme affects the surrounding chrome."}</Fragment>,
          <Fragment key={5}>{"The whole screenshot set is regenerated from the isolated e2e stack — see the runbook in "}<code key={0}>{"docs/wiki-screenshots.md"}</code>{" for how the dark pass works."}</Fragment>,
        ]} />
      </Section>
    </DocPage>
  );
}
