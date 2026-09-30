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

Mobile push notifications are deferred; delivery is in-app feed only for now.
## Full AI Integration
### Integrate AI into the meal planner
AI should evaluate what is in stock, what types of preferences not only the user but the whole household has
Evaluate what might be close to expiration
Suggest recipies that the whole house is likey to enjoy that will consume in stock items with a prefernce towards consuming items about to expire.
### Meal event integration
AI integration for meal events to assisnt in recipe modifications to meet serving times
Integration should take into account limitations of cooking appliances
Integration should be able to make suggestions to alter recipie cooking time and temp so multiple dishes can be prepared at the same time.
The event timeline already detects appliance conflicts between scheduled steps — the AI layer should suggest resolutions for those flagged conflicts (shift serve times, reorder steps, reassign appliances).
### Sommerlier Integration
If the user has a wine collection, the system should be able to, when prompted, suggest wine pairings with dishes
If the request happens during meal planning or event planning there should be an option to limit to wine in stock or advise on wines to be purchased.
AI should consider the contents of each recipie in the meal as well as any preferences that have been found in the household.
We may need to add age of users to weight preferences towards those of legal drinking age.
### Bartender
Add an AI bartender to the system that works simmilarly to the wine adviser, but for cocktails
Do we need to add a Liquor flag to items to do this?
Do we need to flag recipe types as drink or cocktail vs meal?
### Generic interface
Ensure the AI integration is generic enough that Ollama can be swapped out for a commercial ai system like claud or chat gpt with minimal changes.

## System validations
~~Make sure the entire interface and api is idempotent.~~ ✅ Done — `Idempotency-Key` transport dedup on all mutations (PRs #159–#162, see `docs/idempotency-plan.md`).

## Recipe Categories
~~We need to be able to categorize recipes to make searching easier.~~ ✅ Done — grouped categories with per-group exclusivity, faceted filters, and engagement-ranked search on web + mobile (PRs #164–#166, closeout in p4; see `docs/recipe-categories-plan.md`).
~~We need to be able to categorize recipes to make searching easier.~~
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
## TokTok Integration 
Give the app the ability to link a TikToc cooking video.
Should be able to build the recipe from the video
On the day the recipe is to be made, a link to the video should appear along with the recipe in the dashboard
Evaluate if it makes sense to add Youtube and Reels
How "TikTok Recipe Integration" Actually WorksBecause the official TikTok for Developers portal focuses almost entirely on user login, video publishing, or marketing analytics, apps that extract recipes usually rely on one of two methods:AI-Powered Transcription and Parsing (Most Common):An app takes a TikTok URL from the user, uses a basic web scraping method to read the video metadata/description, or grabs the automated subtitles/audio. It then passes that raw text into a Large Language Model (like GPT-4 or Gemini) via an API with a strict structured output instruction. The AI effortlessly structures it into standard recipe JSON (e.g., separating ingredients and steps).Third-Party Pre-Built APIs:Some micro-SaaS developers have built ready-made wrappers like the TikTok Recipe Extractor API on RapidAPI or open-source self-hosted scripts like Pick-a-Recipe that do the scraping and AI restructuring for you in a single API call

## Nutrition tracker
Nutrition is already built into the items. Expand this to give nutrution breakdowns for each recipe and meal
Allow uesers to track daily and weekly nutrition iformation
Show nutrition tracking compared to health recomendations. Proably mostly European until the US gets their health agencies rebuilt.
Evaluate integration with android Health and iOS HealthKit
Using Flutter for a cross-platform mobile app simplifies the integration process. Instead of writing separate native Swift and Kotlin code, a single unified wrapper can bridge iOS HealthKit and Android Health Connect into a single Dart API.The industry standard for this is the open-source health package on pub.dev. It handles permission dialogs, data translation, and writes to both native health vaults using a single set of commands.

## Kitchen appliance
Create an updated version of the mobil interface, or update the exiting one for use on large tablets
This should include the barcode scanner
this shoudl include the abilitty build the meal plan, which is curently limited to web version
This should have a new recpipe interface that scrolls through the steps so people can read as they prep
This should include the ability to watch a tiktok or youtube video in an embedded window if possible.

## Instacart or other shopper integration
It would be cool if the application could validate the grocery list against a store or stores in instacart to send the order in automatically
### Yes, Instacart publishes the Instacart Developer Platform API (IDP), which allows third-party app developers to generate shoppable lists. However, it does not let an external app silently or automatically inject items directly into a user’s active Instacart cart or account background storage via API credentials.
### How the Instacart Developer Platform API Works
Create Shopping List Endpoint: You send a POST request with line items to the API (/idp/v1/products/products_link).
Returns a URL: The API responds with a unique, shareable URL pointing to a pre-populated shopping list page on the Instacart Marketplace.
User Hand-off: The user clicks that link from your application. It opens Instacart where they choose their local store, review available products, and move the items into their active cart before checking out.
### Key Limitations for a Shopping App
No Direct Cart Injection: For privacy, security, and store inventory variances, external third-party apps cannot programmatically push lists straight into a logged-in user's live cart without the user clicking through the generated web/marketplace link first.
Authentication: The workflow relies on handing the user over to the Instacart web or app interface via the deep link, where their existing account status is recognized if they are already logged in on that device/browser.

## Grocery store routing
Order the grocery list by the aisles in the grocery store.
This can take store input from the user
It would be great if we could use the users data logs to track as well. 
Keep track of the order items are checked off the grocery list.
Make educated guess on what order the list should be in for the future based on past logs.

## Allergy information and risk detection
Track allergen information for recipes so users can spot risks before cooking.
Users (or household members) should be able to record their allergies and dietary restrictions, i.e. peanuts, tree nuts, shellfish, dairy, gluten, eggs, soy.
Recipes and items should surface which allergens they contain or may contain.
Warn the user when a recipe on their plan, event, or search results conflicts with a recorded allergy.
Possibly use AI to analyze a recipe's ingredients and flag likely allergy risks automatically, i.e. "may contain traces of nuts" or hidden sources like Worcestershire sauce containing fish.
AI-detected flags should be reviewable/overridable since allergen detection can be wrong in both directions.

## Local AI Assistant
Don’t sweat the Dart setup—you do not need to rewrite your mobile app in Kotlin or Swift, and yes, your web app can absolutely use local desktop GPUs!
The modern ecosystem has evolved rapidly to support cross-platform local AI. You can keep your Dart codebase intact and run models entirely inside the client’s browser tab using WebGPU.
Here is how you can pull off both feats without starting your code from scratch.
------------------------------
You do not need to abandon Dart. To run local Small Language Models (SLMs) right on the user's phone, you can bypass the native Swift/Kotlin requirement entirely using cross-platform tools: [1] 

* 
* Google MediaPipe LLM Inference API: Google officially supports a MediaPipe Flutter plugin. It acts as a Dart wrapper that targets the device's native hardware under the hood (CoreML on iOS or AICore/NPU on Android). You feed it a highly compact model like Gemma 2B or Llama 3.2 1B/3B, and Dart executes it completely on-device. [1, 2, 3] 
* The Ollama-Style Sidecar: Alternatively, you can use packages like flutter_llama (which wraps llama.cpp in a clean Dart FFI package). It compiles straight down to ARM64, turning your Flutter app into its own self-contained local AI client. [4] 
* 

------------------------------
This is one of the coolest frontend paradigms available. Your web app can leverage the user’s dedicated graphics card (like an Nvidia RTX or Apple M-Series chip) straight inside a standard browser tab with zero extensions or installations required. [5, 6] 
## Option A: WebLLM & WebGPU (The High-Performance Path)
You can integrate an open-source library called [WebLLM](https://webllm.mlc.ai/) into your frontend Javascript/Typescript bundle. [7, 8] 

* 
* How it works: When a user opens the LENA web dashboard, WebLLM pulls an optimized, highly compressed model (like a quantized Llama 3.2 3B or Qwen 2.5) straight from a CDN and caches it natively in the browser. [7, 9] 
* Hardware Acceleration: It connects straight to the client's graphics card via the native WebGPU API. Benchmarks show it runs incredibly fast—delivering around 30 to 50 tokens per second purely inside a browser tab while your server costs stay exactly at $0! [2, 6, 7] 
* 

## Option B: Google Chrome’s Built-In Gemini Nano (The Zero-Download Path)
Instead of forcing your web app to stream a 1.5GB model file into the browser cache, you can tap into the actual browser engine. [10] 

* 
* How it works: Desktop versions of Google Chrome ship with a built-in instance of Gemini Nano.
* The Code: You can access it directly via a native JavaScript API window prompt:

// Call Chrome's local NPU engine nativelyconst session = await window.ai.createTextSession();const result = await session.prompt("Suggest a wine pairing for Salmon...");
console.log(result);

* Why it fits LENA: It runs locally, keeping your user's meal history completely private, and offloads all computation away from your hosting environment. [10, 11] 
* 

------------------------------
Because your backend architecture relies on the Model Context Protocol (MCP), setting this up is clean and modular.
When your application initiates an AI task (like resolving a kitchen appliance conflict), your client-side gateway router can execute a quick three-step capability check:

   1. Check Browser: Is window.ai available or does the desktop support WebGPU? If yes, execute the prompt locally using the client's desktop GPU.
   2. Check Mobile: Is the app running on mobile via MediaPipe? If yes, execute using the phone’s on-board NPU.
   3. Cloud Fallback: If the client is using a weak, older device with no local GPU acceleration, only then do you route the request out to your Serverless Ollama cloud node.

This layout keeps your project beautifully scalable, lightning fast for modern devices, and ensures your infrastructure bills stay close to a net-zero baseline.
Would you like to explore a code snippet for implementing WebLLM into your web dashboard layout, or should we look at how to structure the MediaPipe Flutter bridge inside your active Dart app?

[1] [https://www.aimagicx.com](https://www.aimagicx.com/blog/on-device-ai-models-local-llm-guide-2026)
[2] [https://www.youtube.com](https://www.youtube.com/watch?v=1mix7WnuEK0&t=36)
[3] [https://ai-tldr.dev](https://ai-tldr.dev/learn/local-open-models/running-models-locally/run-llms-on-a-phone/)
[4] [https://dev.to](https://dev.to/alichherawalla/how-to-run-local-ai-on-your-android-phone-in-2026-no-cloud-no-account-5cbp)
[5] [https://www.youtube.com](https://www.youtube.com/watch?v=CmXHTo5vmmE&t=775)
[6] [https://medium.com](https://medium.com/codetodeploy/javascript-for-real-time-ai-on-device-llms-with-webgpu-webnn-0daaaea2a2fb)
[7] https://webllm.mlc.ai
[8] [https://github.com](https://github.com/mlc-ai/web-llm)
[9] [https://www.youtube.com](https://www.youtube.com/watch?v=AWylz6JtT5M&t=62)
[10] [https://www.youtube.com](https://www.youtube.com/watch?v=CjpZCWYrSxM&t=6)
[11] [https://medium.com](https://medium.com/google-cloud/get-started-with-chrome-built-in-ai-access-gemini-nano-model-locally-11bacf235514)
