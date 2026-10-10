// Docs navigation model — one source of truth for the public site's
// sidebar groups and the pages that live under each. `live` flips on when
// the page exists; the docs index renders links only for live pages so the
// shell can ship before the full page set lands.
export interface DocsNavItem {
  label: string;
  href: string;
  live: boolean;
}

export interface DocsNavGroup {
  label: string;
  items: DocsNavItem[];
}

export const DOCS_NAV: DocsNavGroup[] = [
  {
    label: "Start here",
    items: [
      { label: "Overview", href: "/docs", live: true },
      { label: "Getting started", href: "/docs/getting-started", live: true },
      { label: "Tips & FAQ", href: "/docs/tips-faq", live: true },
    ],
  },
  {
    label: "Web app",
    items: [
      { label: "Dashboard & notifications", href: "/docs/dashboard", live: true },
      { label: "Recipes", href: "/docs/recipes", live: true },
      { label: "Meal planning", href: "/docs/meal-planning", live: true },
      { label: "Grocery lists", href: "/docs/grocery-lists", live: true },
      { label: "Food events", href: "/docs/food-events", live: true },
      { label: "Inventory & pantry", href: "/docs/pantry", live: true },
      { label: "Ingredients", href: "/docs/ingredients", live: true },
      { label: "Allergy tracking", href: "/docs/allergy-tracking", live: true },
      { label: "Wine cellar", href: "/docs/wine-cellar", live: true },
      { label: "Households", href: "/docs/households", live: true },
      { label: "AI assistant", href: "/docs/ai-assistant", live: true },
      { label: "Analytics", href: "/docs/analytics", live: true },
      { label: "Privacy & local AI", href: "/docs/privacy", live: true },
      { label: "Themes", href: "/docs/themes", live: true },
    ],
  },
  {
    label: "Mobile app",
    items: [
      { label: "Overview", href: "/docs/mobile-app", live: true },
      { label: "Home", href: "/docs/mobile-dashboard", live: true },
      { label: "Recipes & meal plans", href: "/docs/mobile-recipes", live: true },
      { label: "Cook mode", href: "/docs/mobile-cook-mode", live: true },
      { label: "Grocery lists", href: "/docs/mobile-grocery", live: true },
      { label: "Events", href: "/docs/mobile-events", live: true },
      { label: "Pantry", href: "/docs/mobile-pantry", live: true },
      { label: "Barcode scanning", href: "/docs/mobile-scanning", live: true },
      { label: "Allergy tracking", href: "/docs/mobile-allergies", live: true },
      { label: "Wine cellar", href: "/docs/mobile-wine", live: true },
      { label: "Household & notifications", href: "/docs/mobile-household", live: true },
      { label: "Ask Dot", href: "/docs/mobile-ask-dot", live: true },
      { label: "Tablet & landscape", href: "/docs/mobile-tablet", live: true },
    ],
  },
];
