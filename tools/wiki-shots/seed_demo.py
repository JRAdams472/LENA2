#!/usr/bin/env python3
"""Seed demo content into the lena2shots e2e stack for wiki screenshots.

Idempotent: safe to re-run; existing demo entities are reused by name.
Dates are relative to today so the dashboard "today's meals" shot has
content regardless of when it runs. Override with DEMO_DATE=YYYY-MM-DD.
"""
import json
import os
import urllib.request
from datetime import date

API = os.environ.get("DEMO_API", "http://localhost/graphql")
ISSUER = os.environ.get("DEMO_ISSUER", "http://localhost:8085")
TODAY = os.environ.get("DEMO_DATE") or date.today().isoformat()
# JS getDay(): Sun=0..Sat=6; Python weekday(): Mon=0..Sun=6.
DOW = (date.fromisoformat(TODAY).weekday() + 1) % 7
WEEK_NAME = "Week of " + TODAY


def token(sub, email, name):
    url = f"{ISSUER}/token?sub={sub}&email={email}&name={urllib.parse.quote(name)}"
    return json.load(urllib.request.urlopen(url))["id_token"]


def gql(tok, query, variables=None):
    req = urllib.request.Request(
        API,
        data=json.dumps({"query": query, "variables": variables or {}}).encode(),
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {tok}",
        },
    )
    res = json.load(urllib.request.urlopen(req))
    if res.get("errors"):
        raise RuntimeError(f"GraphQL error: {res['errors']} in {query[:80]}")
    return res["data"]


def find_items(tok, keywords, page_size=2000):
    out = {}
    page = 1
    while len(out) < len(keywords):
        data = gql(
            tok,
            "query($p:Int,$n:Int){ items(page:$p,pageSize:$n){ items { id name } pageInfo { totalCount } } }",
            {"p": page, "n": page_size},
        )
        found_any = False
        for it in data["items"]["items"]:
            name = it["name"].lower()
            for kw in keywords:
                if kw in name and (kw not in out or len(it["name"]) < len(out[kw][1])):
                    out[kw] = (it["id"], it["name"])
        if not data["items"]["items"] or page * page_size >= data["items"]["pageInfo"]["totalCount"]:
            break
        page += 1
    return out


ADMIN = token("e2e-user-1", "e2e@example.com", "E2E User")

# Second member joins the household (creates their user row first). Uses a
# dedicated demo user — e2e specs rely on e2e-user-2 staying unaffiliated.
MEMBER = token("e2e-user-3", "e2e-member@example.com", "E2E Member")
gql(MEMBER, "{ me { id } }")
me2 = gql(MEMBER, "{ me { id } }")["me"]["id"]
try:
    invite = gql(
        ADMIN,
        "mutation($id: ID!) { inviteHouseholdMember(userId: $id) { id } }",
        {"id": me2},
    )["inviteHouseholdMember"]
    gql(
        MEMBER,
        "mutation($id: ID!) { acceptHouseholdInvite(inviteId: $id) { id } }",
        {"id": invite["id"]},
    )
    print("household: member joined")
except RuntimeError as e:
    if "already in your household" in str(e):
        print("household: member already joined")
    else:
        raise

items = find_items(
    ADMIN,
    ["chicken", "pasta", "spaghetti", "olive oil", "onion", "garlic", "potato", "rice", "carrot", "tomato", "milk", "egg", "flour", "butter", "cheese", "salt", "broth", "bread"],
)
print("items found:", sorted(items.keys()))


def ri(kw, qty, unit, optional=None):
    item = {"itemId": items[kw][0], "quantity": qty, "unit": unit}
    if optional:
        item["isOptional"] = True
    return item


def get_ing(name):
    page = gql(
        ADMIN,
        "query($s: String) { ingredients(page: 1, pageSize: 10, search: $s) { items { id name } } }",
        {"s": name},
    )
    hit = next(
        (i for i in page["ingredients"]["items"] if i["name"].lower() == name.lower()),
        None,
    )
    if hit:
        return hit["id"]
    return gql(
        ADMIN,
        "mutation($i: CreateIngredientInput!) { getOrCreateIngredient(input: $i) { id } }",
        {"i": {"name": name}},
    )["getOrCreateIngredient"]["id"]


def ing(name, qty, unit, optional=None, brand=None):
    # Ingredient-keyed recipe line; `brand` also binds a preferred catalog item.
    line = {"ingredientId": get_ing(name), "quantity": qty, "unit": unit}
    if brand:
        line["itemId"] = items[brand][0]
    if optional:
        line["isOptional"] = True
    return line


def step(n, instr, dur=None, typ=None, passive=None, dep=None, appl=None):
    s = {"stepNumber": n, "instruction": instr}
    if dur is not None:
        s["durationMinutes"] = dur
    if typ:
        s["stepType"] = typ
    if passive is not None:
        s["isPassive"] = passive
    if dep is not None:
        s["dependsOnStepNumber"] = dep
    if appl:
        s["appliance"] = appl
    return s


existing_recipes = {
    r["name"]: r
    for r in gql(ADMIN, "{ recipes(page:1,pageSize:200){ items { id name servings } } }")["recipes"]["items"]
}


def get_or_create_recipe(input_):
    hit = existing_recipes.get(input_["name"])
    if hit:
        print("recipe exists:", hit["name"])
        return hit
    rec = gql(
        ADMIN,
        "mutation($input: CreateRecipeInput!) { createRecipe(input: $input) { id name servings } }",
        {"input": input_},
    )["createRecipe"]
    print("recipe:", rec["name"])
    return rec


roast = get_or_create_recipe(
    {
            "name": "Herb Roast Chicken with Vegetables",
            "description": "A Sunday-style roast with crisp vegetables.",
            "servings": 4,
            "prepTimeMinutes": 20,
            "cookTimeMinutes": 75,
            "items": [
                ri("chicken", 1, "each"),
                ri("potato", 2, "lb"),
                ri("carrot", 1, "lb"),
                ri("onion", 1, "each"),
                ing("olive oil", 3, "tbsp"),
                ri("salt", 1, "tsp"),
                ri("garlic", 4, "each", optional=True),
            ],
            "steps": [
                step(1, "Heat oven to 425°F and prep the vegetables", 20, "prep", False, None, "oven"),
                step(2, "Season the chicken and place it on the vegetables", 10, "prep", False),
                step(3, "Roast until the skin is crisp and juices run clear", 75, "cook", True, None, "oven"),
                step(4, "Rest the chicken 15 minutes before carving", 15, "rest", True),
                step(5, "Carve and serve with the roasted vegetables", 10, "serve", False),
            ],
        }
)

pasta = get_or_create_recipe(
    {
            "name": "Garlic Butter Pasta",
            "description": "Quick weeknight pasta.",
            "servings": 2,
            "prepTimeMinutes": 5,
            "cookTimeMinutes": 15,
            "items": [
                ri("pasta", 1, "lb"),
                ri("butter", 4, "tbsp"),
                ing("garlic", 3, "each"),
                ri("cheese", 0.5, "cup"),
                ing("salt", 1, "tsp", brand="salt"),
            ],
            "steps": [
                step(1, "Boil salted water and cook the pasta", 12, "cook", True, None, "stovetop"),
                step(2, "Melt butter with garlic until fragrant", 5, "cook", False, None, "stovetop"),
                step(3, "Toss pasta in the garlic butter with cheese", 3, "prep", False),
                step(4, "Serve immediately", 2, "serve", False),
            ],
        }
)

soup = get_or_create_recipe(
    {
            "name": "Hearty Vegetable Soup",
            "description": "Brothy vegetable soup with rice.",
            "servings": 6,
            "prepTimeMinutes": 15,
            "cookTimeMinutes": 40,
            "items": [
                ri("broth", 6, "cup"),
                ri("carrot", 0.5, "lb"),
                ri("onion", 1, "each"),
                ri("potato", 1, "lb"),
                ri("rice", 1, "cup"),
                ri("tomato", 2, "each"),
            ],
            "steps": [
                step(1, "Sauté onion and carrot until softened", 10, "cook", False, None, "stovetop"),
                step(2, "Add broth, potato, and tomato; simmer 25 minutes", 25, "cook", True, None, "stovetop"),
                step(3, "Stir in rice and cook until tender", 15, "cook", True, None, "stovetop"),
                step(4, "Season and ladle into bowls", 5, "serve", False),
            ],
        }
)

# Categories — look up IDs by name and assign (idempotent: set replaces).
cat_by_name = {
    c["name"]: c["id"]
    for g in gql(ADMIN, "{ recipeCategoryGroups { categories { id name } } }")["recipeCategoryGroups"]
    for c in g["categories"]
}


def categorize(recipe, names):
    ids = [cat_by_name[n] for n in names if n in cat_by_name]
    gql(
        ADMIN,
        "mutation($r: ID!, $c: [ID!]!) { setRecipeCategories(recipeId: $r, categoryIds: $c) { id } }",
        {"r": recipe["id"], "c": ids},
    )
    print("categorized:", recipe["name"], "->", names)


categorize(roast, ["Dinner", "Chicken", "Medium", "American", "Casserole"])
categorize(pasta, ["Dinner", "Easy", "Italian", "Vegetarian"])
categorize(soup, ["Lunch", "Soup", "Easy", "Vegetarian", "Low Calorie"])

# Meal plan starting today — day 0 is today so the dashboard shows a meal.
existing_plans = gql(ADMIN, "{ mealPlans(page:1,pageSize:50){ items { id name } } }")["mealPlans"]["items"]
plan = next((p for p in existing_plans if p["name"] == WEEK_NAME), None)
glist = None
if plan:
    plan_id = plan["id"]
    print("plan exists:", plan["name"])
else:
    plan = gql(
        ADMIN,
        "mutation($input: CreateMealPlanInput!) { createMealPlan(input: $input) { id name } }",
        {"input": {"name": WEEK_NAME, "weekStartDate": TODAY, "weekStartDayOfWeek": DOW}},
    )["createMealPlan"]
    plan_id = plan["id"]
    print("plan:", plan["name"])

    for day, meal, recipe, servings in [
        (0, "breakfast", pasta["id"], 2),
        (0, "dinner", soup["id"], 6),
        (1, "dinner", pasta["id"], 4),
        (2, "dinner", roast["id"], 4),
    ]:
        gql(
            ADMIN,
            "mutation($input: AddMealSlotInput!) { addMealSlot(input: $input) { id } }",
            {"input": {"mealPlanId": plan_id, "dayOfWeek": day, "mealType": meal, "recipeId": recipe, "servings": servings}},
        )
    print("slots added")

    glist = gql(
        ADMIN,
        "mutation($id: ID!) { generateGroceryList(mealPlanId: $id) { id } }",
        {"id": plan_id},
    )["generateGroceryList"]
    print("grocery list:", glist["id"])

if not glist:
    lists = gql(ADMIN, "{ groceryLists(page:1,pageSize:10){ items { id } } }")["groceryLists"]["items"]
    glist = lists[0] if lists else {"id": None}
    print("grocery list existing:", glist["id"])

# Demo store with a walk-ordered aisle layout; the grocery list is routed
# through it so the detail page shows route groups.
existing_stores = gql(ADMIN, "{ groceryStores { id name } }")["groceryStores"]
store = next((s for s in existing_stores if s["name"] == "Corner Market"), None)
if store:
    store_id = store["id"]
    print("store exists:", store["name"])
else:
    store_id = gql(
        ADMIN,
        "mutation($name: String!) { createStore(name: $name) { id } }",
        {"name": "Corner Market"},
    )["createStore"]["id"]
    for pos, name in enumerate(["Produce", "Dairy", "Pantry Staples"]):
        gql(
            ADMIN,
            "mutation($storeId: ID!, $name: String!, $position: Int!) { createStoreAisle(storeId: $storeId, name: $name, position: $position) { id } }",
            {"storeId": store_id, "name": name, "position": pos},
        )
    print("store + aisles:", store["name"] if store else "Corner Market")

if glist["id"]:
    gql(
        ADMIN,
        "mutation($groceryListId: ID!, $storeId: ID) { setGroceryListStore(groceryListId: $groceryListId, storeId: $storeId) { id } }",
        {"groceryListId": glist["id"], "storeId": store_id},
    )
    aisles = gql(ADMIN, "{ groceryStores { id aisles { id } } }")["groceryStores"]
    demo_aisles = next(s["aisles"] for s in aisles if s["id"] == store_id)
    rows = gql(
        ADMIN,
        "query($id: ID!) { groceryList(id: $id) { items { id ingredient { id } item { id } manualItemName } } }",
        {"id": glist["id"]},
    )["groceryList"]["items"]
    # Round-robin assignments — enough to show two aisles plus unassigned.
    for i, row in enumerate(rows):
        if i % 3 == 2:
            continue  # leave some items unassigned
        ident = {
            "storeId": store_id,
            "aisleId": demo_aisles[i % 2]["id"],
            "itemId": (row.get("item") or {}).get("id"),
            "ingredientId": (row.get("ingredient") or {}).get("id"),
            "manualItemName": row.get("manualItemName"),
        }
        gql(
            ADMIN,
            "mutation($storeId: ID!, $aisleId: ID, $itemId: ID, $ingredientId: ID, $manualItemName: String) { assignItemToAisle(storeId: $storeId, aisleId: $aisleId, itemId: $itemId, ingredientId: $ingredientId, manualItemName: $manualItemName) }",
            ident,
        )
    print("list routed:", len(rows), "items")

    # Usual-brand fixture — check off the ingredient-only garlic line with a
    # branded catalog item so the row shows a remembered "usual:" brand.
    garlic_ing = get_ing("garlic")
    garlic_row = next(
        (r for r in rows if (r.get("ingredient") or {}).get("id") == garlic_ing),
        None,
    )
    if garlic_row and items.get("garlic"):
        gql(
            ADMIN,
            "mutation($id: ID!, $itemId: ID!) { checkGroceryItemWithBrand(groceryListItemId: $id, itemId: $itemId) { id isChecked } }",
            {"id": garlic_row["id"], "itemId": items["garlic"][0]},
        )
        print("usual brand: garlic ->", items["garlic"][1])

# Food event on today's date — targetTime's date must match eventDate.
existing_events = gql(ADMIN, "{ foodEvents(page:1,pageSize:50){ items { id name } } }")["foodEvents"]["items"]
event = next((e for e in existing_events if e["name"] == "Autumn Dinner Party"), None)
if event:
    ev_id = event["id"]
    print("event exists:", event["name"])
else:
    event = gql(
        ADMIN,
        "mutation($input: CreateFoodEventInput!) { createFoodEvent(input: $input) { id name } }",
        {"input": {"name": "Autumn Dinner Party", "eventDate": TODAY, "slotGranularityMinutes": 30}},
    )["createFoodEvent"]
    ev_id = event["id"]
    for recipe, meal, t, servings in [
        (soup["id"], "dinner", "18:30", 8),
        (roast["id"], "dinner", "19:00", 8),
    ]:
        gql(
            ADMIN,
            "mutation($input: AddEventRecipeInput!) { addEventRecipe(input: $input) { id } }",
            {"input": {"foodEventId": ev_id, "recipeId": recipe, "mealType": meal, "targetTime": f"{TODAY}T{t}:00Z", "servings": servings}},
        )
    gql(
        ADMIN,
        "mutation($input: AddEventRecipeInput!) { addEventRecipe(input: $input) { id } }",
        {"input": {"foodEventId": ev_id, "recipeId": None, "mealType": "other", "targetTime": f"{TODAY}T19:30:00Z", "notes": "Store-bought rolls"}},
    )
    print("event:", event["name"])

# Wine — grab reference ids then add two bottles.
cellar_count = gql(ADMIN, "{ userBottles(page:1,pageSize:1){ pageInfo { totalCount } } }")["userBottles"]["pageInfo"]["totalCount"]
if cellar_count == 0:
    refs = gql(
        ADMIN,
        "{ countries { id name } types { id name } }",
    )
    country = region = None
    for c in refs["countries"]:
        regions = gql(
            ADMIN,
            "query($c: ID!) { regions(countryId: $c) { id name } }",
            {"c": c["id"]},
        )["regions"]
        if regions:
            country, region = c["id"], regions[0]["id"]
            break
    wtype = refs["types"][0]["id"]
    assert region, "no wine region found in reference data"
    for vineyard, year, qty in [("Silver Oak", 2019, 2), ("Domaine Serene", 2021, 1)]:
        bottle = gql(
            ADMIN,
            "mutation($input: CreateBottleInput!) { createBottle(input: $input) { id } }",
            {"input": {"typeId": wtype, "countryId": country, "regionId": region, "vintageYear": year, "vineyard": vineyard, "bottleSize": "750ml"}},
        )["createBottle"]
        gql(
            ADMIN,
            "mutation($id: ID!, $q: Int!) { adjustUserBottle(bottleId: $id, quantity: $q) { id } }",
            {"id": bottle["id"], "q": qty},
        )
    print("bottles added")
else:
    print("cellar already stocked")

# Pantry stock — a few items on hand.
pantry_count = gql(ADMIN, "{ userItems(page:1,pageSize:1){ pageInfo { totalCount } } }")["userItems"]["pageInfo"]["totalCount"]
if pantry_count == 0:
    for kw, qty in [("milk", 1), ("egg", 12), ("flour", 5), ("butter", 2), ("rice", 3), ("olive oil", 1)]:
        if kw in items:
            gql(
                ADMIN,
                "mutation($id: ID!, $q: Float!) { adjustUserItem(itemId: $id, quantity: $q) { id } }",
                {"id": items[kw][0], "q": qty},
            )
    print("pantry stocked")
else:
    print("pantry already stocked")

# Allergen demo data — member records plus curated item flags so the warning
# surfaces (recipe detail, meal plan, grocery list) and the profile allergy
# editor have content. The set* mutations are upsert-shaped, so re-running
# is a no-op.
allergen_by_name = {
    a["name"].lower(): a["id"]
    for a in gql(ADMIN, "{ allergens { id name } }")["allergens"]
}
gql(
    ADMIN,
    "mutation($a: ID!, $k: MemberAllergyKind!, $on: Boolean!) { setMyAllergy(allergenId: $a, kind: $k, on: $on) }",
    {"a": allergen_by_name["milk"], "k": "allergy", "on": True},
)
gql(
    MEMBER,
    "mutation($a: ID!, $k: MemberAllergyKind!, $on: Boolean!) { setMyAllergy(allergenId: $a, kind: $k, on: $on) }",
    {"a": allergen_by_name["peanuts"], "k": "allergy", "on": True},
)
for kw, allergen, kind in [
    ("milk", "milk", "contains"),
    ("butter", "milk", "contains"),
    ("cheese", "milk", "contains"),
    ("pasta", "wheat", "contains"),
    ("bread", "wheat", "may_contain"),
]:
    if kw in items:
        gql(
            ADMIN,
            "mutation($i: ID!, $a: ID!, $k: AllergenFlagKind) { setItemAllergen(itemId: $i, allergenId: $a, kind: $k) }",
            {"i": items[kw][0], "a": allergen_by_name[allergen], "k": kind},
        )
print("allergen demo data seeded")

print(json.dumps({"planId": plan_id, "groceryListId": glist["id"], "eventId": ev_id}))
