# DESIGN.md — LENA Kitchen & Household Ecosystem

> Source of truth for the shared visual language. Tokens below are measured
> from the shipped themes — `clients/web/app/providers.tsx` (MUI) and
> `clients/mobile/lib/theme.dart` (Flutter). If this doc and code disagree,
> code is authoritative; treat the doc as aspirational only where marked.

## 1. Visual Theme & Philosophy
LENA bridges functional household logistics with an inviting culinary aesthetic. The core visual strategy relies on the high-end, border-constrained structural density of **Linear**, paired with the approachable, minimalist typographic hierarchy of **Notion** and the warm, soft-shadowed layout comfort of **Airbnb**.

- **Density & Cleanliness:** High information density without visual clutter. Rely on crisp 1px borders instead of heavy offsets to separate sections.
- **Tone:** Premium, professional, orderly, yet warm and organic. Never clinical or cold.

---

## 2. Color Palette & Design Tokens
We use an earthy, low-contrast palette designed to look cozy on a laptop screen or a phone in a brightly lit kitchen.

```css
:root {
  /* Core Canvas (as shipped — web MUI palette / mobile lena* constants) */
  --bg-primary: #FAF6EF;       /* Warm cream scaffold background   (lenaCream)  */
  --bg-surface: #FFFDF8;       /* Paper cards, sidebar, sheets     (lenaPaper)  */

  /* Sage — brand accent: top header on web + mobile AppBar, primary color */
  --accent-sage: #7C9473;      /* Primary                          (lenaSage)      */
  --accent-sage-light: #A4B79C;/* Primary light                    (lenaSageLight) */
  --accent-sage-dark: #5F7A57; /* Primary dark, header bottom rule (lenaSageDark)  */

  /* Olive — secondary accent (success/info semantic on web) */
  --accent-olive: #8A9A5B;     /* Secondary                        (lenaOlive)      */
  --accent-olive-dark: #6E7B45;/*                                  (lenaOliveDark)  */
  --accent-olive-light:#A9B87F;/*                                  (lenaOliveLight) */

  /* Warm accents — dashboard suggestion tiles rotate sage/olive/terracotta/wheat */
  --accent-terracotta: #C0876B;/*                                  (lenaTerracotta) */
  --accent-wheat: #D9B26A;     /*                                  (lenaWheat)      */

  /* Text & Hairlines */
  --text-primary: #3E3E34;     /* Warm espresso ink                (lenaInk)       */
  --text-muted: #6B6B5E;       /* Secondary text                   (lenaInkMuted)  */
  --text-on-sage: #FFFDF8;     /* Text/icons over the sage header  (lenaPaper)     */
  --border-subtle: #E5DFD3;    /* 1px hairline dividers            (lenaDivider)   */

  /* Mobile Specific Targets */
  --touch-target-min: 44px;    /* Minimum height/width for tap areas */
}
```

Semantic overrides worth remembering: web maps `success`/`info` to the olive trio so toast/alert styling stays in-palette.

---

## 3. Typography & Hierarchy
The intended family is **Nunito** — rounded, friendly, kitchen-warm — matching the mobile app, which bundles Nunito Regular/Medium/SemiBold/Bold.

> **Gap:** the web theme does not declare a font today and falls back to the MUI default (Roboto stack). Aligning web to Nunito is the known delta.

- **Font Family:** `"Nunito", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;`
- **Scale:**
  - `h1 (Page/AppBar titles)`: 20px | Bold (700) | mobile AppBar uses exactly this
  - `h2 (Section Headings)`: 18px (Desktop) / 16px (Mobile) | Semi-Bold (600)
  - `h3 (Card Headings)`: 14px | Semi-Bold (600)
  - `Body Text`: 14px | Regular | Line-height: 1.5
  - `Small Labels / Pills`: 11–12px | Medium/Semi-Bold | muted ink; mobile `labelSmall` adds `letterSpacing: 1.1`

---

## 4. Desktop Layout Architecture
- **Top Header (AppBar):** Full-width, sage (`--accent-sage`) background, paper-white foreground, **no shadow** — separated from content by a `1px` bottom rule in `--accent-sage-dark`. Carries the wordmark, Ask Dot link, notifications, and account.
- **Sidebar (Nav Drawer):** Width fixed at `260px`. Full height. Uses the **light surface** (`--bg-surface`) — not sage. Items are a vertical stack; the active item gets a sage fill at ~10% alpha with sage text/icon, `border-radius: 10px`, `margin: 0 12px`, `padding: 10px 16px`.
- **Main Canvas:** Liquid grid; screenshots target `1440px` width, `32px` desktop padding.

---

## 5. Mobile Layout & Responsive Architecture (Airbnb App Pattern)
Mobile is a separate Flutter app with a bottom tab bar — and the web drawer collapses to a temporary drawer below MUI `md` (**900px**).

- **The Navigation Switch:** On phones, navigation is a bottom tab bar (paper background, sage-dark selected label/icon, muted unselected, 12px labels).
- **The 8 Tabs** (do not collapse these — each is a distinct task surface):
  1. **Dashboard** — today's meals and ideas
  2. **Grocery** — shopping lists
  3. **Events** — food events + timelines
  4. **Scan** — barcode scanner
  5. **Pantry** — stock quantities
  6. **Household** — members, invites, notification feed (unread badge)
  7. **Ask Dot** — assistant chat
  8. **More** — recipes, meal plans, wine cellar, item catalog, and the rest
- **Card Adaptation:** Multi-column desktop grids collapse to a single scrolling column; padding drops from `32px` toward `16px`.
- **Touch Targets:** Interactive rows and controls scale to a minimum of `var(--touch-target-min)`.

---

## 6. Component Styling Guidelines

### Cards (Airbnb-inspired Warm Surfaces)
- **Background:** `--bg-surface` (`#FFFDF8`)
- **Border:** `1px solid var(--border-subtle)` where separation is needed
- **Radius:** `12px` on mobile cards; `10px` on web (the MUI theme radius everything derives from)
- **Shadow:** Extremely diffused ambient only — web uses `0 4px 12px rgba(62,62,52,0.06)`; the header itself is shadowless

### Restock Items & Labels (Linear-inspired Detail Lines)
- Item lists use `1px` hairline row dividers in `--border-subtle`.
- Pill tags (package counts like `3pk`, `10 Ct`): paper background, `--text-muted` text, `1px solid var(--border-subtle)` border, small radius.

---

## 7. Mobile Feature-Specific Components

1. **Barcode Scanner UI:** Clean camera overlay — dark translucent mask (`rgba(0,0,0,0.4)`), center square cutout with sage corner accents, and a bottom sheet for instant match attributes.
2. **Swipe Actions:** Grocery and pantry rows may support right-to-left swipe to delete/decrement, mimicking premium native interfaces. *(Aspirational — verify current implementation before citing as shipped.)*

---

## 8. Do's and Don'ts

### DO:
- Keep text contrasts high using espresso ink (`#3E3E34`) over cream/paper surfaces.
- Anchor floating mobile actions (Scan Barcode, Add Item) near the bottom-right thumb zone.
- Use explicit icons next to sidebar and tab items.
- Keep the sage identity on the **top header** — it's the app's signature band on both clients.

### DON'T:
- Do not paint the sidebar sage — it is a light surface; sage belongs to the top header and to accent/tint states.
- Do not mix corner radiuses casually: `12px` mobile cards / `10px` web surfaces; smaller only for pills/badges.
- Do not hide critical features behind multi-layer dropdowns on mobile; rely on simple vertical scrolling.
- Do not introduce highly vibrant or saturated blues, purples, or neon accents. Stick to the sage/olive/terracotta/wheat ecosystem.
