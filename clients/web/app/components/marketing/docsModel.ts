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
      { label: "Getting started", href: "/docs/getting-started", live: false },
      { label: "Tips & FAQ", href: "/docs/tips-faq", live: false },
    ],
  },
  {
    label: "Web app",
    items: [
      { label: "Dashboard & notifications", href: "/docs/dashboard", live: false },
      { label: "Recipes", href: "/docs/recipes", live: false },
      { label: "Meal planning", href: "/docs/meal-planning", live: false },
      { label: "Grocery lists", href: "/docs/grocery-lists", live: false },
      { label: "Food events", href: "/docs/food-events", live: false },
      { label: "Inventory & pantry", href: "/docs/pantry", live: false },
      { label: "Ingredients", href: "/docs/ingredients", live: false },
      { label: "Allergy tracking", href: "/docs/allergy-tracking", live: false },
      { label: "Wine cellar", href: "/docs/wine-cellar", live: false },
      { label: "Households", href: "/docs/households", live: false },
      { label: "AI assistant", href: "/docs/ai-assistant", live: false },
      { label: "Analytics", href: "/docs/analytics", live: false },
      { label: "Privacy & local AI", href: "/docs/privacy", live: false },
      { label: "Themes", href: "/docs/themes", live: false },
    ],
  },
  {
    label: "Mobile app",
    items: [
      { label: "Overview", href: "/docs/mobile-app", live: false },
      { label: "Home", href: "/docs/mobile-dashboard", live: false },
      { label: "Recipes & meal plans", href: "/docs/mobile-recipes", live: false },
      { label: "Cook mode", href: "/docs/mobile-cook-mode", live: false },
      { label: "Grocery lists", href: "/docs/mobile-grocery", live: false },
      { label: "Events", href: "/docs/mobile-events", live: false },
      { label: "Pantry", href: "/docs/mobile-pantry", live: false },
      { label: "Barcode scanning", href: "/docs/mobile-scanning", live: false },
      { label: "Allergy tracking", href: "/docs/mobile-allergies", live: false },
      { label: "Wine cellar", href: "/docs/mobile-wine", live: false },
      { label: "Household & notifications", href: "/docs/mobile-household", live: false },
      { label: "Ask Dot", href: "/docs/mobile-ask-dot", live: false },
      { label: "Tablet & landscape", href: "/docs/mobile-tablet", live: false },
    ],
  },
];
