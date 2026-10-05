# LEN-50 — P3 Mobile UI Polish — Proof

Phase 3 of the LEN-47 UI polish plan. All 17 mobile findings from
`docs/ui-polish-audit.md` (M-01–M-17) addressed on branch
`len-50-p3-mobile-fixes`, PR #282.

## Finding-by-finding

| ID | Finding | Fix |
|----|---------|-----|
| M-01 | 8 fixed tabs truncate labels | `labelBehavior: onlyShowSelected`, labels shortened ("Home", "People") |
| M-02 | Edit-bottle filled dropdown labels strike through values | Theme-wide `FloatingLabelBehavior.always` — also fixed the same latent defect on edit-item and edit-meal-plan |
| M-03 | Raw ISO `Generated:` timestamp | `fmtIso` → "Generated Oct 4, 2026" |
| M-04 | `Ahold Ahold` dup brand; `Source: mealplan` internal value | `brandedName` dedupe + humanized "From meal plan" |
| M-05 | 35 bare spinners, 0 skeletons; blank boot | `SkeletonList`/`SkeletonForm`/`SkeletonCard` + `LenaSplash` branded boot; inline spinners kept for active actions |
| M-06 | Scan camera permission at first launch | Lazy tab build — IndexedStack children construct on first visit |
| M-07 | `Colors.*`/`fontSize` literals in 8+ files | Mapped to `colorScheme`/`textTheme`/named tokens |
| M-08 | `Day (0-6)` raw fields | Weekday dropdowns; slots render "Sun — breakfast" |
| M-09 | Assistant empty chat has no prompts | Quick-prompt chips mirroring web suggestions |
| M-10 | Allergen warning = lone red triangle | Chip with icon + count |
| M-11 | Two stacked ambiguous FABs on wine | Labeled `Adjust` extended FAB; catalog moved to AppBar action w/ tooltip |
| M-12 | Meal-plan rows repeat date; dual "Active" chips | Single localized date line; chip only when plan is active |
| M-13 | Notification title/body dup; floating save icon | Rows deduped + card-wrapped; save inlined on field; settings gear → AppBar (was unreachable with empty feed) |
| M-14 | Bare ListTile rows vs Card rows elsewhere | Card-wrapped rows on recipes/items/bottles |
| M-15 | `each — Pantry / Ahold` ambiguous separators | `·` separators, unit on quantity |
| M-16 | ~90px airy rows | `dense` tiles / tighter padding |
| M-17 | Bell-z mute icon unclear | Labeled `Snooze` button next to the delivery switch |

## Additional defect found by the capture walk

Every tab FAB shared the default hero tag. With tabs kept alive in
`IndexedStack`, pushing any route after visiting two FAB-bearing tabs threw
"multiple heroes that share the same tag" — a real crash path, not a test
artifact. All FABs now carry unique `heroTag`s.

## Capture harness fixes (in this PR)

- `settle` now pumps frames in a bounded loop — a bare `Future.delayed`
  froze route transitions mid-animation (earlier detail shots caught
  50%-opacity fades).
- Modal/back pops are gated on the tap actually opening a route — a missed
  tap can no longer `handlePopRoute` the root and background the app.
- AppBar-title diagnostics after every nav make the next failure
  diagnosable without a screen dump.
- Grocery detail taps the **last** list tile — the seeded list sorts
  oldest; newest (auto-generated) lists are often empty.

## Verification

- `dart format` — applied
- `flutter analyze` — clean
- `flutter test` — 105/105 pass
- 27-shot `flutter drive` walk on emulator-5554 against the seeded
  lena2shots stack (`e2e-user-1` debug token): every PNG inspected —
  skeletons where expected, seeded data renders, no login fallback, no
  duplicate-hero crash, scan renders the camera gate text.

Evidence: `len-50-mobile-shots.tar.gz` (27 PNGs) attached to LEN-50.
