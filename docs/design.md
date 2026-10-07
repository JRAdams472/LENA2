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

### Light mode (as shipped)

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

### Dark mode (v2 spec)

Dark mode is **warm charcoal, never pure black** — the palette stays in the
same warm-brown family as cream so both modes feel like the same kitchen.

```css
:root.dark {
  --bg-primary-dark: #211F1A;      /* Warm charcoal canvas                  */
  --bg-surface-dark: #2B2922;      /* Cards, sidebar, sheets                */
  --bg-elevated-dark: #35322A;     /* Dialogs, menus, popovers              */

  /* Accents lift (not saturate) for contrast on dark */
  --accent-sage-darkmode: #93A88B;     /* Primary on dark (~6.7:1 on canvas) */
  --accent-sage-darkmode-dark: #7C9473;/* Filled-button sage stays brand     */
  --accent-olive-darkmode: #A9B87F;    /* Secondary                          */
  --accent-terracotta-darkmode: #D09B7F;
  --accent-wheat-darkmode: #E3C584;

  /* Text & Hairlines */
  --text-primary-dark: #EFEAE0;    /* Warm paper ink                       */
  --text-muted-dark: #B5AE9D;      /* Secondary text                       */
  --text-on-sage-dark: #211F1A;    /* Text/icons over sage fills           */
  --border-subtle-dark: #3A372E;   /* Hairline dividers                    */
  --error-dark: #D08A77;           /* Warm terracotta-red, softened        */
}
```

Rules for dark mode:

- Elevation is expressed with **borders + surface steps**, not heavier shadows —
  the light-mode ambient shadow becomes a 1px `--border-subtle-dark` hairline.
- Selected/active tints keep sage at ~14% alpha over `--bg-surface-dark`.
- The sage AppBar stays the signature band in dark mode but deepens to
  `--accent-sage-dark` (`#5F7A57`, ~4.7:1 with paper text) with a
  `--border-subtle-dark` bottom rule.

---

## 3. Typography & Hierarchy
The family is **Nunito** — rounded, friendly, kitchen-warm — on **both**
clients. Mobile bundles Nunito Regular/Medium/SemiBold/Bold as TTF assets;
web self-hosts the same TTFs via `next/font/local` (`--font-nunito`).

- **Font Family:** `"Nunito", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;`
- **Scale (shared ramp):**

| Slot | Size | Weight | Notes |
|---|---|---|---|
| Display / page titles | 20px | 700 | web `h4`, mobile AppBar title |
| Section headings | 18px (web `h5`) / 16px (mobile) | 600 | |
| Card / sub headings | 14px (web `h6`) | 600 | mobile `titleMedium` |
| Body | 14px | 400, lh 1.5 | web `body1`/`body2` |
| Secondary body | 13px (web `subtitle2`) | 500 | mobile `bodySmall` |
| Small labels / pills | 11–12px | 500–600 | muted ink; mobile `labelSmall` adds `letterSpacing: 1.1` |

- **Density rule:** type does the hierarchy work — don't add borders or
  heavier weight where a size step already separates levels.

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
- **Background:** `--bg-surface` (`#FFFDF8`) / `--bg-surface-dark` (`#2B2922`)
- **Border:** `1px solid var(--border-subtle)` where separation is needed
- **Radius:** `12px` on mobile cards; `10px` on web (the MUI theme radius everything derives from)
- **Shadow:** Extremely diffused ambient only — web uses `0 4px 12px rgba(62,62,52,0.06)`; the header itself is shadowless. In dark mode the ambient shadow is replaced by the `--border-subtle-dark` hairline (borders carry elevation).

### Spacing rhythm
- **4px grid** — all padding/margins land on multiples of 4 (8/12/16/24/32).
- Desktop page padding `32px`, collapsing toward `16px` on mobile.
- Card internal padding `16px`; section gaps `24px`; page sections `32px`.

### Restock Items & Labels (Linear-inspired Detail Lines)
- Item lists use `1px` hairline row dividers in `--border-subtle`.
- Pill tags (package counts like `3pk`, `10 Ct`): paper background, `--text-muted` text, `1px solid var(--border-subtle)` border, small radius.

### Radius scale (one system, both clients)
| Element | Radius |
|---|---|
| Web surfaces/buttons/inputs | 10px |
| Mobile cards | 12px |
| Chips / pills | 8px |
| Small badges | full pill |

### Loading states
- **Skeletons, never lone spinners** — list/table loads render skeleton rows
  shaped like the final layout; skeletons crossfade into content (~150ms).
  A bare `CircularProgress`/`CircularProgressIndicator` is only acceptable
  for short inline actions (button spinners, pull-to-refresh).

### Empty states
- Composed, never blank: leading icon at ~40px rendered at 35% accent alpha
  (sage by default), one muted line of second-person copy
  ("Nothing planned for lunch yet"), optional CTA link/button.

---

## 7. Motion & Interaction

Motion exists to orient, not decorate. Both clients share one spec.

| Slot | Duration | Use |
|---|---|---|
| Micro | 150ms | hover, press, color/opacity changes, skeleton→content fade |
| Standard | 250ms | sheets, dialogs, drawers, tab/section transitions |
| Emphasis | 400ms max | page-level transitions only — used sparingly |

- **Easing:** standard decelerate — `cubic-bezier(0.2, 0, 0, 1)`
  (web CSS / MUI `transitions.easing.easeOut`; mobile `Curves.easeOutCubic`,
  which is close enough to share the spec).
- **Pressed feedback:** `scale(0.98)` or `translateY(1px)` on `:active` /
  `InkWell` — controls should feel physical.
- **Skeleton → content:** crossfade, never a hard pop.
- **Theme switching** animates nothing — mode changes apply instantly (a
  crossfade of the whole app reads as flicker).
- **Reduced motion:** honor `prefers-reduced-motion` / platform reduce-motion
  by collapsing durations to ~0; no essential information may ride on motion.

---

## 8. Mobile Feature-Specific Components

1. **Barcode Scanner UI:** Clean camera overlay — dark translucent mask (`rgba(0,0,0,0.4)`), center square cutout with sage corner accents, and a bottom sheet for instant match attributes.
2. **Swipe Actions:** Grocery and pantry rows may support right-to-left swipe to delete/decrement, mimicking premium native interfaces. *(Aspirational — verify current implementation before citing as shipped.)*

---

## 9. Do's and Don'ts

### DO:
- Keep text contrasts high using espresso ink (`#3E3E34`) over cream/paper surfaces.
- Anchor floating mobile actions (Scan Barcode, Add Item) near the bottom-right thumb zone.
- Use explicit icons next to sidebar and tab items.
- Keep the sage identity on the **top header** — it's the app's signature band on both clients.
- In dark mode, keep warm neutrals warm — charcoal surfaces, not blue-greys.

### DON'T:
- Do not paint the sidebar sage — it is a light surface; sage belongs to the top header and to accent/tint states.
- Do not mix corner radiuses casually: `12px` mobile cards / `10px` web surfaces; smaller only for pills/badges.
- Do not hide critical features behind multi-layer dropdowns on mobile; rely on simple vertical scrolling.
- Do not introduce highly vibrant or saturated blues, purples, or neon accents. Stick to the sage/olive/terracotta/wheat ecosystem.
- Do not use pure black (`#000`) or pure white (`#FFF`) surfaces — the palette's warmth comes from always carrying a little brown.
