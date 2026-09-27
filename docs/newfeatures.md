# MVP
## Notification manager
### Manage notifications
Opt out of notifications by category
Opt out of notifications for a time period
add new notification types
Add a protien notification based on weekly meal plan
48 hours before any protien is called for, the notification should be sent to remind the user to remove it from the freezer if needed
If the amount of protien is more than 8 lbs, move the notification. It should estimate 24 hours per 4 lbs, rounded up. So if it is 10 lbs, send 3 days early. 20 lbs should be 5 days early, etc.
prep notifications on recipes with multi day steps
Notifications when things are about to expire 
this particular notification should allow you to add a replacement item onto your weekly grocery list.

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
Make sure the entire interface and api is idempotent.

## Recipe Categories
We need to be able to categorize recipes to make searching easier.
Categories should include but not be limited to:
Breakfast
Lunch
Dinner
Cocktail
Soup
Bread
Casserole
Single Pan
Low Calorie
Difficulty categorization (Easy, Medium, Skilled, Etc)
Main Ingredient, i.e. Chicken, Beef, Fish, Vegetarian
Regional origin, i.e. Italian, Spanish, Southwestern US, Mexican, Etc.
Categories of the same type should be exclusive, i.e. it can not both be Mexican and Italian, but recipes should be allowed to belong to multimple categories, i.e. Mexican, Beef, Dinner
When searching for recipes, either in meal planning or a general recipe search, the user should have the ablilty to filter by category.

# Version 2
## TokTok Integration 
Give the app the ability to link a TikToc cooking video.
Should be able to build the recipe from the video
On the day the recipe is to be made, a link to the video should appear along with the recipe in the dashboard
Evaluate if it makes sense to add Youtube and Reels

## Nutrition tracker
Nutrition is already built into the items. Expand this to give nutrution breakdowns for each recipe and meal
Allow uesers to track daily and weekly nutrition iformation
Show nutrition tracking compared to health recomendations. Proably mostly European until the US gets their health agencies rebuilt.
evaluate integration with samsum health, whatever applese rip off of samsung health is and any other popular health apps.

## Kitchen appliance
Create an updated version of the mobil interface, or update the exiting one for use on large tablets
This should include the barcode scanner
this shoudl include the abilitty build the meal plan, which is curently limited to web version
This should have a new recpipe interface that scrolls through the steps so people can read as they prep
This should include the ability to watch a tiktok or youtube video in an embedded window if possible.

## Instacart or other shopper integration
It would be cool if the application could validate the grocery list against a store or stores in instacart to send the order in automatically

## Grocery store routing
Order the grocery list by the aisles in the grocery store.
This can take store input from the user
It would be great if we could use the users data logs to track as well. 
Keep track of the order items are checked off the grocery list.
Make educated guess on what order the list should be in for the future based on past logs.