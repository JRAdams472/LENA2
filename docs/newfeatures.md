# Shipped features

This file is a history of what shipped. The backlog of planned/ideas-stage
work lives in Linear (project **LENA**) — don't add new wish-list items here.

# MVP
## Notification manager
### Manage notifications
~~Opt out of notifications by category~~ ✅ Done — per-category switches on `/notifications` (web) and the mobile settings screen (PRs #181–#183).
~~Opt out of notifications for a time period~~ ✅ Done — preset mute windows per category plus a global mute.
~~add new notification types~~ ✅ Done — `household.notification_type` registry; new kinds are seed rows, not schema changes.
~~Add a protien notification based on weekly meal plan~~ ✅ Done — `protein_defrost` reminders from meal-plan slots; item categories flagged `is_protein`.
~~48 hours before any protien is called for, the notification should be sent to remind the user to remove it from the freezer if needed~~ ✅ Done — hourly sweep, dedup-keyed per household member.
~~If the amount of protien is more than 8 lbs, move the notification. It should estimate 24 hours per 4 lbs, rounded up. So if it is 10 lbs, send 3 days early. 20 lbs should be 5 days early, etc.~~ ✅ Done — `ceil(lbs/4)` days once over 8 lbs.
~~prep notifications on recipes with multi day steps~~ ✅ Done — `meal_prep_advance` fires for recipe steps with duration ≥ 24h.
~~Notifications when things are about to expire~~ ✅ Done — `item_expiring` reminders from pantry `expires_at` within a configurable window (`NOTIFY_EXPIRY_DAYS`, default 3).
~~this particular notification should allow you to add a replacement item onto your weekly grocery list.~~ ✅ Done — `addItemToCurrentGroceryList` adds the item to the latest list as a manual entry.

## Full AI Integration
✅ Done — provider-agnostic `llm.Provider` (Ollama + mock + seam for commercial APIs), MCP-shaped read-only tool registry with a bounded agent loop, household-scoped GraphQL surface (`askAssistant`, `suggestMeals`, `suggestEventFixes`, `suggestPairings`, `suggestCocktails`), web + mobile assistant UIs, and a birthdate-based 21+ gate on all alcohol suggestions (PRs #196–#200).
### Integrate AI into the meal planner
~~AI should evaluate what is in stock, what types of preferences not only the user but the whole household has~~ ✅ Done — `get_household_tastes` feeds analytics-driven household affinity into prompts.
~~Evaluate what might be close to expiration~~ ✅ Done — `get_expiring_items` surfaces pantry items nearing expiry.
~~Suggest recipies that the whole house is likey to enjoy that will consume in stock items with a prefernce towards consuming items about to expire.~~ ✅ Done — `suggestMeals` returns reviewable cards applied via `addMealSlot` (PR #198).
### Meal event integration
~~AI integration for meal events to assisnt in recipe modifications to meet serving times~~
~~Integration should take into account limitations of cooking appliances~~
~~Integration should be able to make suggestions to alter recipie cooking time and temp so multiple dishes can be prepared at the same time.~~
~~The event timeline already detects appliance conflicts between scheduled steps — the AI layer should suggest resolutions for those flagged conflicts (shift serve times, reorder steps, reassign appliances).~~ ✅ Done — `suggestEventFixes` consumes `BuildTimeline` conflicts and proposes shift_serve/set_appliance/set_duration/set_dependency fixes applied through existing mutations (PR #199).
### Sommerlier Integration
~~If the user has a wine collection, the system should be able to, when prompted, suggest wine pairings with dishes~~ ✅ Done — `suggestPairings` on recipe detail (PR #200).
~~If the request happens during meal planning or event planning there should be an option to limit to wine in stock or advise on wines to be purchased.~~ ✅ Done — cellar bottles are marked `inCellar`; style picks carry no bottle id.
~~AI should consider the contents of each recipie in the meal as well as any preferences that have been found in the household.~~ ✅ Done — recipe + household tastes are in the prompt context.
~~We may need to add age of users to weight preferences towards those of legal drinking age.~~ ✅ Done — nullable `birthdate` on profile; server-side 21+ gate on all alcohol suggestions.
### Bartender
~~Add an AI bartender to the system that works simmilarly to the wine adviser, but for cocktails~~ ✅ Done — `suggestCocktails` with an in-stock pantry filter (PR #200).
~~Do we need to add a Liquor flag to items to do this?~~ ✅ Done — the pantry coverage check uses existing ingredient items; no liquor flag needed.
~~Do we need to flag recipe types as drink or cocktail vs meal?~~ ✅ Done — the seeded `Cocktail` dish-type category selects candidates.
### Generic interface
~~Ensure the AI integration is generic enough that Ollama can be swapped out for a commercial ai system like claud or chat gpt with minimal changes.~~ ✅ Done — `llm.Provider` interface with Ollama and mock implementations; a commercial provider is a new adapter + a config value.

## System validations
~~Make sure the entire interface and api is idempotent.~~ ✅ Done — `Idempotency-Key` transport dedup on all mutations (PRs #159–#162, see `docs/idempotency-plan.md`).

## Recipe Categories
~~We need to be able to categorize recipes to make searching easier.~~ ✅ Done — grouped categories with per-group exclusivity, faceted filters, and engagement-ranked search on web + mobile (PRs #164–#166, closeout in p4; see `docs/recipe-categories-plan.md`).
~~Categories should include but not be limited to:~~ ✅ Done — all seeded in migration 0035:
~~Breakfast~~, ~~Lunch~~, ~~Dinner~~ — Course group
~~Cocktail~~, ~~Soup~~, ~~Bread~~, ~~Casserole~~, ~~Single Pan~~ — Dish Type group
~~Low Calorie~~ — Dietary group
~~Difficulty categorization (Easy, Medium, Skilled, Etc)~~ — Difficulty group
~~Main Ingredient, i.e. Chicken, Beef, Fish, Vegetarian~~ — Main Ingredient group (+ Pork)
~~Regional origin, i.e. Italian, Spanish, Southwestern US, Mexican, Etc.~~ — Cuisine group (+ French, American, Asian)
~~Categories of the same type should be exclusive, i.e. it can not both be Mexican and Italian, but recipes should be allowed to belong to multimple categories, i.e. Mexican, Beef, Dinner~~ ✅ Done — exclusivity is per group; a recipe holds one value per exclusive group plus any number from non-exclusive groups.
~~When searching for recipes, either in meal planning or a general recipe search, the user should have the ablilty to filter by category.~~ ✅ Done — filter bar on `/recipes`, category dropdown in the meal-plan slot picker, filter sheet on mobile.
## Analytics-driven search ranking
✅ Done — engagement-ranked search on every catalog and picker surface on web and mobile, analytics-weighted recipe recommendations, meal-type-aware recipe pickers, and engagement-ranked grocery restock suggestions (PRs #186–#190).
~~Use analytics in all searches to provide more targeted results.~~
~~Every search in the app (recipes, items, brands, pantry, wine, etc.) should order results by how likely the user is to actually use them, rather than random, alphabetical, or insertion order.~~ ✅ Done — all list queries rank before pagination: favorites → course boost (recipe pickers) → household/personal usage → viewed → prior search terms → name. The 110k-item catalog is served by a two-phase query (ranked engaged set + index-ordered remainder) so deep pages stay cheap.
Signals to rank by should include but not be limited to:
~~Favorites~~ ✅
~~Past usage in menus, plans, events, and grocery lists~~ ✅ — synchronous interaction events plus a time-decayed score rollup rebuilt by a periodic decay job
~~Items the user has viewed or selected before~~ ✅
~~Items matching terms the user has searched for previously~~ ✅
~~Ratings and recommendation scores where they exist~~ ✅ — `recommendedRecipes` merges ingredient overlap, rating recency, category affinity, and household trending; `suggestedRestockItems` ranks depleted pantry stock by household engagement
~~Household usage where the catalog is shared~~ ✅ — personal > household > global weighting on shared catalogs
~~Ranking should degrade gracefully to a sensible default order when a user has little or no analytics history.~~ ✅ — engagement failures and empty history fall back to alphabetical/name order.

## Web UI/UX polish
✅ Done — not a planned feature; a shipped visual refresh of the web client (PRs #192–#193):
- Card-based layouts on a light gray canvas (`#f8fafc`) with soft shadows and rounded corners, applied app-wide via the MUI theme.
- Full-width top bar with the new cloche + `LENA` logo (also the login screen and favicon); user email in a profile pill; icon-button sign-out.
- Sidebar navigation with pill-style active states, softened icon tint, and increased padding.
- Dashboard "Today's meals" as a 3-column icon grid with dashed-pill "+ Plan a meal" actions; "Suggested for You" recipe reasons as soft-green chips; "Running low" rows with amber status dots and package-size badges (size deduplicated out of item names); suggested/restock cards in a bento grid on wide screens.

# Version 2
## Multi-provider sign-in & account linking
✅ Done (PRs #210, #211). A LENA user can carry multiple provider logins (`identity.user_login` maps `(provider, external_subject)` → user). **Discord OAuth2** ships on web — server-side code exchange (`client_secret` never leaves the server), `state` CSRF check, subject = stable snowflake ID. `GET /auth/identities`, `POST /auth/link`, `DELETE /auth/link` manage logins; linking requires step-up auth and **never auto-merges by email**. Facebook OIDC and mobile Discord are tracked as LEN-18. See `docs/auth-multi-plan.md`.

## Session refresh tokens
✅ Done (PRs #205–#208). LENA-issued sessions on top of the Google credential: short-lived signed access token (`iss=lena`, ~15 min) + rotating opaque refresh token (hashed at rest, ~30-day sliding expiry, theft-detection family revocation). Endpoints `POST /auth/session{,/refresh,/revoke}`; web and mobile refresh transparently and fall back to OIDC-only mode when `LENA_SESSION_SECRET` is unset. See `docs/auth-oidc.md` §8 and `docs/refresh-tokens-plan.md`.

## Semantic recipe search (RAG)
✅ Done (PRs #226, #227). pgvector `recipe.embedding` (768-dim `nomic-embed-text` via Ollama `/api/embed`, `LENA_AI_EMBED_MODEL`), refreshed on every recipe write and healed by a startup+periodic sweep; a deterministic hashing mock embedder covers e2e. `recipes(searchMode: semantic)` ranks embedded recipes by cosine distance + a small engagement bump (filters preserved, `semanticSearchAvailable` gates it); Dot gets the bounded read-only `search_recipes_semantic` tool; the web recipes page has a Semantic toggle with a describe-the-mood hint. "No eggs"-style negations still need `get_recipe_details` verification — embeddings can't exclude.

## Grocery store routing
✅ Done — household-defined stores with ordered aisles; the server computes route groups (`groceryRouteGroups`) and both clients render them verbatim so web and phone always agree. Check-off order is learned into `grocery.item_route` (`checked_seq / list size` folded in per check); explicit user arrangement (`manual_rank`, persisted via `reorderGroceryListItems`) wins over learned order and survives list regeneration. Cross-aisle drags write the aisle move and rank in one mutation; unassigned items get a `suggested` aisle inferred from their nearest learned neighbor (PRs #216–#219).

## Local AI Assistant
✅ Done — Ask Dot and all structured suggestions can run inference on the user's own device, offloading model compute from the server (PRs #235–#238):

- **Server surface (P1).** New GraphQL reads let client agents drive the same pipeline the server agent uses: `assistantTools` (read-only, household-scoped tool specs), `callAssistantTool` (dispatches under the caller's auth scope with a per-user rate limiter and JSON-schema argument validation), `assistantPrompt` (server-owned system prompts), and `prepareAssistantRequest` (server-assembled context + output contract for the structured suggestion flows). Household scope always comes from the auth context, never the request — and the 21+ age gate on alcohol suggestions stays server-enforced.
- **Web Ask Dot (P2).** `lib/ai/` capability chain: Chrome's built-in model → lazy-loaded WebLLM (~5.8 MB async chunk, never in the shared bundle) → server fallback. A JSON tool-call protocol (`{"toolCalls":[…]}` / `{"answer":…}`) with a bounded agent loop, since small local models lack native function calling. Opt-in card with download progress, engine badge ("On this device" / "Via server"), and `auto`/`server` mode persisted in localStorage.
- **Mobile Ask Dot (P3).** Same protocol + agent loop in Dart over `flutter_gemma` 0.13.6, behind a `GemmaBinding` seam so the logic is testable without a device. Model manager with download progress/cancel/delete, opt-in card, server-only toggle; model source configurable via `LENA_LOCAL_MODEL_URL`/`_TOKEN`/`_ID` dart-defines (default is the license-gated HF Gemma3-1B-IT, so builds can point at a self-hosted mirror).
- **Local suggestions (P4).** All four web suggestion surfaces — meal plans, event fixes, recipe pairings, cocktails — go local-first through `prepareAssistantRequest`, then validate output client-side against the candidates already in the context payload before rendering reviewable cards. Suggestions never trigger a model download; they reuse the engine only when it's already ready, else fall back to the server.
- Server stays authoritative throughout: tools are read-only, writes still flow through the existing validated mutations, and local engines fall back transparently to the server provider.
- Also shipped: the Go CI test step dropped from ~9 min to ~4 min by sharing one postgres testcontainer per package instead of launching one per test call (#239).

## Web UI refresh
✅ Done — sage/cream/olive theme and a warmer voice across the web client (PRs #221–#224; see `docs/web-ui-refresh-plan.md`):
- Sage primary `#7C9473` with a solid sage AppBar, warm cream surfaces (`#FAF6EF`/`#FFFDF8`), olive success/info chips, warm text/divider tokens; OAuth brand buttons untouched.
- Two-zone sidebar: core kitchen nav on top, a pinned bottom zone with collapsible "Administration" (admin-only: Users, Pending Items) and "Account" (Household, Profile) captions.
- Dashboard warmth: empty meal slots show a faded icon + "Nothing planned for … yet" + "Plan it →"; recipe suggestions are cards with deterministic accent tiles and category-driven icons, capped at 5; headings and empty states read conversationally ("Delicious ideas for tonight", "Time to restock", "Nothing here yet").
- Bare links pick up the sage palette via `CssBaseline`; the cloche logo and favicon are recolored to match.
- Grocery-list restock rows share the dashboard's `stripSize`/`sizeBadge` helpers, so duplicated brand prefixes and size tokens collapse into a clean name + chip (#223).
- Two-row header: the logo spans both rows, notifications/user/sign-out sit top-right, and the assistant moved out of the sidebar to a sparkles "Ask Dot" link on the second row — the chat page is rebranded to Dot (#224).

## Mobile UI refresh
✅ Done — the web's sage/cream palette and warmer voice ported to the Flutter app (PRs #229–#232):
- App-wide `lenaTheme()` — sage primary, warm cream surfaces, bundled Nunito, themed nav bar/cards/inputs/buttons; "Assistant" renamed to Ask Dot with the sparkles icon.
- Dashboard redesigned to mirror the web: greeting header with date + avatar, "Today's meals" with meal-type icons and a tappable "Plan it →" empty state, a featured recommendation card plus compact suggestion rows with category-colored icon badges, cook-time metadata, friendly reason labels (no more `Reason: category_affinity`), and chevrons tapping through to recipes.
- Fixed a latent bug where Sunday meal plans never rendered (wire `dayOfWeek` 0=Sun vs Dart `weekday` 1=Mon).
- Debug sign-in: `LENA_DEBUG_ID_TOKEN` now works from the login-screen button too, so emulator screenshot/e2e runs skip Google entirely.

## Generic ingredients (unbranded recipe lines)
✅ Done — recipes, meal plans, grocery lists, and imports can reference generic ingredients ("corn") independently of branded catalog items ("Green Giant corn"); inventory stays branded, with a two-level link resolving brand for stock and check-off (PRs #244–#247, plan: `docs/ingredient-layer-plan.md`):
- **Schema + data (P1).** `inventory.item.ingredient_id` global link + `userprefs.household_item_ingredient` override + `userprefs.household_ingredient_item` "usual brand" table; normalized-name unique index on `inventory.ingredient`; 413-ingredient starter list seeded; `recipe_item`/`meal_slot_item`/`event_recipe_item` allow ingredient-keyed lines with `item_id` as optional preferred brand; LLM curation tool (`cmd/ingredientcurate`) emits a reviewable artifact.
- **Backend (P2).** `ingredientId` on all recipe/meal-slot/event/grocery write inputs (itemId optional, ≥1 required); `Item.ingredient`/`householdIngredient`, `GroceryListItem.usualBrand`; `getOrCreateIngredient` (member), `mergeIngredient` + `setItemIngredient` (admin), `setHouseholdItemIngredient`, `checkGroceryItemWithBrand`; grocery needs aggregate by ingredient with pantry stock resolved override→catalog→unlinked and netted across brands; check-off credits bound item → usual brand; nutrition + AI pantry availability resolve ingredient lines to representative items; `EntityIngredient` analytics.
- **Web (P3).** Ingredient-first recipe editor with inline create + preferred-brand picker; ingredient binding in import review; ingredient-primary grocery labels with `usual:` captions and a first-time brand picker on check-off; item create/edit catalog + household ingredient pickers; `/inventory/ingredients` admin page (search/create/edit/deactivate/merge) gated by a fixed child-level `adminOnly` nav filter.
- **Mobile (P4).** Same ingredient-first recipe editor, usual-brand captions + first-time brand pick on grocery check-off, scan hits show resolved ingredient (household override wins) with a "Link ingredient" prompt, meal-plan slot items fall back to ingredient names.

## Security hardening (OWASP Top 10 2021)
✅ Done — ten-finding remediation under LEN-29 (PRs #257–#261, report: `docs/len-29-remediation.md`):
- **Nonce CSP.** `clients/web/proxy.ts` issues a per-request nonce — `script-src 'self' 'nonce-…' 'strict-dynamic'` replaces `'unsafe-inline'`; `style-src` keeps `unsafe-inline` for Emotion/MUI. Caddy keeps all other security headers.
- **PKCE.** OAuth code flows (Discord/Microsoft/Facebook) now send S256 `code_challenge`; the BFF requires `codeVerifier` on session exchange and forwards `code_verifier` to token endpoints.
- **Session hardening.** `LENA_SESSION_SECRET` must be ≥32 bytes or the API refuses to boot; admin deactivation evicts the cached identity and revokes all refresh-token families in the same transaction (immediate ban, no 2-min window).
- **Authorization gaps.** `setRecipeCategories` is admin-only (global catalog); household invites require an active, searchable target with enumeration-proof identical errors and a 10/min per-caller rate limit.
- **Dependency/config.** All 65 OCR image CVEs cleared with a `pip-audit` CI gate preventing regression; grpc bumped (GO-2026-6443); SonarQube bound to localhost.

## SonarQube hygiene (LEN-28)
✅ Done — all 194 open SonarQube findings remediated to zero across six phases (PRs #263–#268; per-phase proofs in `docs/proof/`):
- Zero bugs, vulnerabilities, or security hotspots; every Go and TypeScript function refactored to cognitive complexity ≤15; Python, Docker, and dev-tool findings cleared; one accepted `wontfix` remains (`capture.mjs` PATH search — local screenshot tool).
- Process additions in `AGENTS.md`: a **boy-scout rule** (fix trivial low/info findings in any file being modified) and a **close-out rescan gate** (zero new open issues beyond the wontfix list before a plan may close).

## UI polish pass (LEN-47)
✅ Done — cross-client audit-driven polish (PRs #270–#271, #282; audit: `docs/ui-polish-audit.md`, proofs: `docs/proofs/`):
- **Audit harness (P1).** `integration_test/screenshot_test.dart` + `test_driver/` capture all 27 mobile screens on an emulator against the seeded lena2shots stack; web uses the existing Playwright capture. Screenshots buffer until test end, so the walk gates modal/back pops and pumps frames during waits instead of bare delays.
- **Web (P2).** All 25 findings: `DataTable` gained typed cells (Yes/No, localized dates), inferred audit-field hiding, skeleton loading (`role="status"`), truthful empty states, mobile card layout, and optional action columns; paged brand/food-nutrient catalogs replace unbounded fetches; date-only strings parse as local dates; brand-prefix dedupe via `brandedName`/`brandSuffix`; icon-only Ask Dot at 390px.
- **Mobile (P3).** All 17 findings: shared `SkeletonList`/`SkeletonForm`/`LenaSplash` replace bare spinners; bottom nav fits 8 tabs (selected-label-only) and tabs build lazily so Scan's camera prompt waits for first visit; `FloatingLabelBehavior.always` theme-wide fixes strike-throughs on every async edit form; weekday dropdowns replace `Day 0-6`; labeled `Adjust` FAB + AppBar catalog on wine; labeled `Snooze` in notification settings; unique FAB `heroTag`s fix a duplicate-hero crash on route transitions; `Colors.*`/`fontSize` literals moved to theme tokens.

## Household recipe deltas (LEN-25)
✅ Done — members tweak shared recipes for their household without forking or editing the canonical recipe (PRs #284–#287, plan: `~/.devin/plans/plan-8cda506c3c12b05b.md`):
- **Schema + engine (P1).** `recipe_delta` (one per recipe+household, `base_updated_at` drift marker) + `recipe_delta_item`/`recipe_delta_step` change rows anchored on `recipe_item_id`/`step_id` with `ON DELETE SET NULL` so canonical edits orphan — never silently drop — tweaks. `recipe.ApplyDelta` merges substitute/adjust/remove/add line rows and replace/remove/add step rows; orphans are counted, not applied.
- **BFF (P2).** Deltas apply at every consumer seam — recipe display (incl. allergens + allergy warnings), `ScaledRecipe` (delta first, then scale), `planRecipes` (meal-plan nutrition + grocery generation), `snapshotRecipeContents` (event dishes freeze the household version), and AI lazy paths. `Recipe.householdDelta` exposes stale/orphan state; `items`/`itemSections`/`steps(view: canonical)` serve the untouched base; `setRecipeDelta`/`clearRecipeDelta`/`acknowledgeRecipeDelta` are member-gated (any household member edits the shared delta).
- **Web (P3).** Recipe detail defaults to the effective view with a "Household version" chip and Original recipe toggle; per-line/per-step Tweak controls, delta badges (Swapped/Adjusted/Added/Replaced), a Household tweaks panel with draft→save flow, stale banner + Mark reviewed, and orphan chips; canonical admin edits merge onto the canonical view so household rows never bake into the shared recipe.
- **Mobile (P4).** Same surface on the recipe screen — contents list with badges, bottom-sheet tweak editors, tweaks card, stale banner, view toggle.
