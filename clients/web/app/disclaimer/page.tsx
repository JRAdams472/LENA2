import { Fragment } from "react";
import type { Metadata } from "next";
import Box from "@mui/material/Box";
import MarketingShell from "@/app/components/marketing/MarketingShell";
import {
  DocList,
  DocPage,
  P,
  Section,
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: "Medical & Allergen Disclaimer — LENA",
  description:
    "LENA's medical and allergen disclaimer: informational convenience only, not medical or dietary advice.",
};

export default function DisclaimerPage() {
  return (
    <MarketingShell>
      <Box sx={{ px: { xs: 2, md: 4 }, py: { xs: 3, md: 5 }, maxWidth: 1120 }}>
        <DocPage
          title="Medical & Allergen Disclaimer"
          intro="LENA can flag allergens and dietary conflicts across recipes, meal plans, events, and grocery lists. This disclaimer explains what those features are — and what they are not."
        >
          <Section title="Information, not advice">
            <P>
              {
                "Allergen and dietary information in Lena's Kitchen is provided for convenience only. It is not medical, nutritional, or dietary advice, and it is not a substitute for professional judgment. If you have an allergy, intolerance, or medical condition, consult a qualified healthcare professional."
              }
            </P>
          </Section>
          <Section title="Data may be incomplete or wrong">
            <DocList
              items={[
                <Fragment key="crowd">
                  Allergen flags on ingredients, items, and products are
                  maintained by households and may be <strong>incomplete,
                  inaccurate, or out of date</strong>. Manufacturers change
                  formulations without notice.
                </Fragment>,
                <Fragment key="cross">
                  Warnings do not account for <strong>cross-contamination</strong>,
                  regional product variants, or ingredients not yet catalogued.
                </Fragment>,
                <Fragment key="ai">
                  AI-assisted features (recipe imports, suggestions) can
                  misclassify ingredients and <strong>must not be relied on</strong>
                  {" "}for allergy decisions.
                </Fragment>,
              ]}
            />
          </Section>
          <Section title="Always check the label">
            <P>
              {
                "Before cooking or consuming anything, check the product's actual label and packaging. The label on the physical product is authoritative — an app record is not. When in doubt, do not consume it."
              }
            </P>
          </Section>
          <Section title="Liability">
            <P>
              {
                "To the fullest extent permitted by law, Lena's Kitchen accepts no liability for any illness, injury, allergic reaction, or other harm arising from reliance on allergen or dietary information in the app. You use these features at your own risk."
              }
            </P>
          </Section>
          <Section title="Your records">
            <P>
              {
                "Recording an allergy or dietary preference in your profile requires accepting this disclaimer once. Removing a record is always available without acceptance, and warnings you have already configured keep working either way."
              }
            </P>
          </Section>
        </DocPage>
      </Box>
    </MarketingShell>
  );
}
