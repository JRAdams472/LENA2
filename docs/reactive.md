The LENA2 web app has partial, inconsistent responsive design. It's not systematically built to be "properly reactive" everywhere, though some parts adapt well to mobile/tablet.

What is responsive
The main navigation shell in AdminLayout.tsx is genuinely responsive. It uses MUI's md breakpoint to switch between a permanent sidebar drawer on larger screens and a temporary, hamburger-toggled drawer on mobile/tablet. AdminLayout.tsx:270-295 The main content area shifts its margin only at md and up, and the menu IconButton is hidden on desktop (display: { md: "none" }). AdminLayout.tsx:47-55 AdminLayout.tsx:240-248

The app is built on Material UI (MUI), whose components (AppBar, Drawer, Table, form controls) carry sensible default styling and touch-friendly sizing. layout.tsx:3-5 Next.js App Router also injects a width=device-width, initial-scale=1 viewport meta tag by default, so basic mobile scaling works even though no viewport is declared explicitly in layout.tsx. layout.tsx:7-10

What is NOT responsive
No custom breakpoints anywhere. A search for @media, max-width, and min-width returns zero matches, and MUI breakpoint helpers (xs/sm/md, theme.breakpoints) are only used in AdminLayout.tsx and one line of meal-plans/[id]/page.tsx. AdminLayout.tsx:145
Data tables don't adapt to small screens. The shared DataTable component renders a standard Table inside a TableContainer, with no column hiding, card-based reflow, or responsive layout. On a phone, wide tables will just overflow / require horizontal scrolling rather than reflowing. DataTable.tsx:146-196
The table header row uses a fixed space-between flex layout for the title and Create button, which works but isn't tuned for narrow widths. DataTable.tsx:124-138
Summary
The app is a MUI/Next.js admin UI where the navigation chrome is properly responsive (mobile drawer + hamburger), but the content — especially the data-heavy tables used across inventory, wine, recipes, and meal planning — relies on MUI defaults and horizontal scroll rather than deliberate mobile/tablet layouts. So it will be usable on a phone or tablet, but it was not comprehensively designed to be fully reactive across all views.