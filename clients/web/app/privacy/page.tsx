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
  title: "Privacy Policy — LENA",
  description:
    "LENA's privacy policy: what data we collect, how we use it, and your rights.",
};

export default function PrivacyPage() {
  return (
    <MarketingShell>
      <Box sx={{ px: { xs: 2, md: 4 }, py: { xs: 3, md: 5 }, maxWidth: 1120 }}>
        <DocPage
          title="Privacy Policy"
          intro="This policy explains what LENA collects when you use the website and the mobile companion app, why we collect it, and the choices you have. It applies to lenaskitchen.app and the Lena mobile apps."
        >
          <Section title="What we collect">
            <DocList
              items={[
                <Fragment key="account">
                  <strong>Account information</strong> — your name and email
                  address, provided by the social sign-in provider you choose
                  (Google, Apple, Discord, or Facebook). We never see your
                  provider password.
                </Fragment>,
                <Fragment key="household">
                  <strong>Household content you add</strong> — pantry items,
                  recipes, meal plans, grocery lists, wine cellar entries,
                  allergy and dietary records, and optional profile details
                  such as your birthdate (used only to gate alcohol-related
                  suggestions to adults).
                </Fragment>,
                <Fragment key="push">
                  <strong>Push notification tokens</strong> — if you enable
                  notifications on mobile, we store a device token so we can
                  deliver them.
                </Fragment>,
                <Fragment key="usage">
                  <strong>Usage and activity data</strong> — household
                  notifications, and analytics about how features are used so
                  we can improve recommendations and reliability.
                </Fragment>,
              ]}
            />
          </Section>
          <Section title="How we use it">
            <DocList
              items={[
                <Fragment key="auth">
                  To <strong>authenticate you</strong> and keep your household
                  data synchronized across web and mobile.
                </Fragment>,
                <Fragment key="notify">
                  To send <strong>utility notifications</strong> you asked for
                  — household activity, meal reminders, defrosting alerts, and
                  pantry expiry reminders. Notification categories can be
                  turned off individually, and push can be disabled entirely.
                </Fragment>,
                <Fragment key="operate">
                  To <strong>operate and improve</strong> the features you use,
                  including recipe and restock suggestions.
                </Fragment>,
              ]}
            />
          </Section>
          <Section title="What we never do">
            <DocList
              items={[
                <Fragment key="no-sale">
                  <strong>We do not sell your data to third parties.</strong>{" "}
                  Your information is used to run LENA and nothing else.
                </Fragment>,
                <Fragment key="no-share">
                  We do not share your household&apos;s contents with other
                  households, advertisers, or data brokers.
                </Fragment>,
              ]}
            />
          </Section>
          <Section title="Your choices and rights">
            <DocList
              items={[
                <Fragment key="delete">
                  <strong>Delete your account</strong> — from your Profile
                  (web) or Household (mobile) you can delete your account and
                  all associated data. Deletion is permanent and cannot be
                  undone.
                </Fragment>,
                <Fragment key="notifications">
                  <strong>Control notifications</strong> — every category can
                  be muted or disabled in notification settings, and mobile
                  push can be revoked in your device settings at any time.
                </Fragment>,
                <Fragment key="profile">
                  <strong>Manage your profile</strong> — names, backup email,
                  birthdate, and discoverability are editable in your Profile.
                </Fragment>,
              ]}
            />
          </Section>
          <Section title="Questions">
            <P>
              {
                "Questions about this policy or your data can be raised through the project's issue tracker or by contacting the instance administrator for self-hosted deployments."
              }
            </P>
          </Section>
        </DocPage>
      </Box>
    </MarketingShell>
  );
}
