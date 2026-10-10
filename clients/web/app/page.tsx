import type { Metadata } from "next";
import LandingPage from "@/app/components/marketing/LandingPage";

export const metadata: Metadata = {
  title: "LENA — the household kitchen, kept",
  description:
    "Recipes, pantry, meal plans, grocery lists, and the wine cellar in one place your whole household shares.",
};

export default function RootPage() {
  return <LandingPage />;
}
