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
Use analytics in all searches to provide more targeted results.
Every search in the app (recipes, items, brands, pantry, wine, etc.) should order results by how likely the user is to actually use them, rather than random, alphabetical, or insertion order.
Signals to rank by should include but not be limited to:
Favorites
Past usage in menus, plans, events, and grocery lists
Items the user has viewed or selected before
Items matching terms the user has searched for previously
Ratings and recommendation scores where they exist
Household usage where the catalog is shared
Ranking should degrade gracefully to a sensible default order when a user has little or no analytics history.

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