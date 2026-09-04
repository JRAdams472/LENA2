# ADR 0002: Multi-user data model (global vs per-user classification and target schema)

- Status: Proposed (design/discovery only — no schema or code changes accompany this ADR)
- Date: 2026-09-02
- Supersedes / relates to: Phase 1 authentication (Google JWT bearer validation in `LENA.API/Program.cs`, `NameClaimType = "email"`)

## Context

LENA is a self-hosted single-tenant application. There is currently **no user data model**: identity exists only as free-text `CreatedBy` / `LastUpdatedBy` strings on `AuditableEntity` (`LENA/Entity/Common/AuditableEntity.cs`), stamped by `AuditingBehavior` from `ICurrentUserService.UserName` (which resolves to the Google `email` claim, or the literal `"system"` when anonymous).

Phase 1 added Google JWT validation. The stable per-user identifier Google issues is the `sub` claim; `email` is mutable and must not be used as a join key.

All data access goes through Dapper repositories (`LENA.Application/Repositories/*`) calling `[Schema].[usp_*]` stored procedures. Per-user scoping must therefore ultimately be enforced **inside the stored procedures** via a `@UserID` parameter, not in C# filtering.

This ADR is the authoritative classification of every column in every table as one of:

| Class | Meaning |
|---|---|
| **G** — Global/static | Shared catalog/reference data. Visible to and (for now) editable by every user. No `UserID`. |
| **U** — Per-user | State that belongs to exactly one user. Must be scoped by `UserID`. |
| **A** — Audit/metadata | `CreatedBy`, `CreateDate`, `LastUpdatedBy`, `LastUpdatedDate` (and surrogate keys). Carried on both global and per-user rows; not itself a scoping dimension. |

Surrogate primary keys (`XxxID INT IDENTITY`) are listed as **A** (metadata) unless they are also a foreign key that carries per-user meaning.

---

## 1. Column-level inventory

Source of truth: `LENA.Database/{Inventory,Wine,Recipe,MealPlan}/Tables/*.table.sql` as of this ADR. Every column of every table is listed.

### 1.1 Inventory schema

#### `Inventory.Item` — **MIXED** (catalog + per-user stock)

| Column | Type | Class | Notes |
|---|---|---|---|
| ItemID | INT IDENTITY | A (PK) | Stays as the global catalog key |
| Name | NVARCHAR(200) | G | Catalog |
| BrandID | INT NULL → ItemBrand | G | Catalog |
| UPC12 | NVARCHAR(12) NULL | G | Catalog; `UQ_Item_UPC12` stays global |
| UPC14 | NVARCHAR(14) NULL | G | Catalog; `UQ_Item_UPC14` stays global |
| CategoryID | INT → Category | G | Catalog |
| Unit | NVARCHAR(20) | G | Canonical inventory unit for the catalog item |
| CurrentQuantity | DECIMAL(10,2) | **U** | Move to per-user stock table |
| MinQuantity | DECIMAL(10,2) NULL | **U** | Reorder threshold is a user preference |
| PurchaseDate | DATETIME2 | **U** | Belongs to the user's stock |
| ExpiryDate | DATETIME2 NULL | **U** | Belongs to the user's stock |
| Notes | NVARCHAR(500) NULL | **U** | User's private notes (catalog description, if ever needed, would be a new global column) |
| IsFavorite | BIT | **U** | User preference |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | — | A | |

Constraints: `UQ_Item_Name_BrandID`, `UQ_Item_UPC12`, `UQ_Item_UPC14`, `FK_Item_Category`, `FK_Item_ItemBrand` — all remain **global**.

#### `Inventory.InStock` — currently unused; **candidate home for per-user stock**

| Column | Type | Class | Notes |
|---|---|---|---|
| StockID | INT IDENTITY | A (PK) | |
| ItemID | INT → Item | **U** (FK into per-user row) | |
| QuantityOnHand | DECIMAL(10,2) | **U** | Equivalent of `Item.CurrentQuantity` |
| LastUpdatedDate | DATETIME2 DEFAULT GETUTCDATE() | A | |
| CreatedBy / CreateDate / LastUpdatedBy | — | A | |

No `UserID`, no uniqueness on `ItemID`. There is a domain entity `LENA/Entity/Inventory/InStock.cs` and a frontend type `InStock` but no stored procedures, repository, or features reference the table.

#### `Inventory.Category` — **GLOBAL**

| Column | Class |
|---|---|
| CategoryID (PK) | A |
| CategoryName (`UQ_Category_CategoryName`) | G |
| Description | G |
| IsActive | G |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | A |

#### `Inventory.ItemBrand` — **GLOBAL**

| Column | Class |
|---|---|
| ItemBrandID (PK) | A |
| Name (`UQ_ItemBrand_Name`) | G |

(No audit columns.)

#### `Inventory.flavor_profiles` — **GLOBAL**

| Column | Class |
|---|---|
| flavor_id (PK) | A |
| flavor_name (UNIQUE) | G |
| is_active | G |

#### `Inventory.food_flavors` — **GLOBAL** (catalog attribute of an Item)

| Column | Class |
|---|---|
| food_id (PK, FK → Item, CASCADE) | G |
| flavor_id (PK, FK → flavor_profiles) | G |
| intensity_score (CHECK 1–5) | G |

#### `Inventory.food_nutrients` — **GLOBAL** (catalog attribute of an Item)

| Column | Class |
|---|---|
| food_id (PK, FK → Item, CASCADE) | G |
| nutrient_id (PK, FK → nutrient_types) | G |
| amount_per_serving | G |

#### `Inventory.nutrient_types` — **GLOBAL**

| Column | Class |
|---|---|
| nutrient_id (PK) | A |
| nutrient_name (UNIQUE) | G |
| unit_of_measure | G |

### 1.2 Wine schema

#### `Wine.Bottle` — **MIXED** (wine definition + cellar holding)

| Column | Type | Class | Notes |
|---|---|---|---|
| BottleID | INT IDENTITY | A (PK) | Becomes the key of the static wine definition |
| BottleNumber | INT NULL | **U** | A cellar/bin number is a per-user labelling of a holding; several indexes (`IX_Bottle_*_BottleNumber`) sort by it and will need to move to the holding table |
| TypeID | INT → Type | G | Definition |
| CountryID | INT → Country | G | Definition |
| RegionID | INT → Region | G | Definition |
| VintageYear | INT | G | Definition (note: an INT year, not an FK to `Wine.Vintage`) |
| Vineyard | NVARCHAR(200) NULL | G | Definition |
| ABV | DECIMAL(5,2) NULL | G | Definition |
| Acidity | TINYINT NULL | G | Definition (tasting profile) |
| TanninLevel | TINYINT NULL | G | Definition |
| Body | TINYINT NULL | G | Definition |
| Sweetness | TINYINT NULL | G | Definition |
| OakIntegration | BIT NULL | G | Definition |
| BottleSize | NVARCHAR(20) DEFAULT '750ml' | G | Format is part of the product definition (the same wine in 750ml vs magnum is a different SKU). Alternative: treat as per-holding; recommend **G** so a holding = (definition, quantity) |
| Quantity | INT DEFAULT 1 | **U** | Cellar holding |
| PurchaseDate | DATETIME2 | **U** | Cellar holding |
| PurchasePrice | DECIMAL(10,2) NULL | **U** | Cellar holding |
| StorageTemp | DECIMAL(5,1) NULL | **U** | Cellar holding |
| Location | NVARCHAR(100) NULL | **U** | Cellar holding |
| Notes | NVARCHAR(500) NULL | **U** | Cellar holding |
| IsFavorite | BIT | **U** | User preference |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | — | A | |

Indexes in `Wine/Indexes/`: `IX_Bottle_CountryID_BottleNumber`, `IX_Bottle_RegionID_BottleNumber`, `IX_Bottle_TypeID_BottleNumber`, `IX_Bottle_VintageYear_BottleNumber`, `IX_Bottle_Vineyard` (definition-side, stay on Bottle but drop the `BottleNumber` key part), `IX_Bottle_IsFavorite_BottleNumber` (per-user, moves to the holding table).

#### `Wine.BottleFlavorProfile` — **GLOBAL** (attribute of the wine definition)

| Column | Class |
|---|---|
| BottleID (PK, FK → Bottle, CASCADE) | G |
| FlavorProfileID (PK, FK → FlavorProfile, CASCADE) | G |
| CreatedBy / CreateDate | A |

#### `Wine.BottleGrapeVariety` — **GLOBAL** (attribute of the wine definition)

| Column | Class |
|---|---|
| BottleID (PK, FK → Bottle, CASCADE) | G |
| GrapeVarietyID (PK, FK → GrapeVariety, CASCADE) | G |
| Percentage | G |
| CreatedBy / CreateDate | A |

#### `Wine.Country` — **GLOBAL**

| Column | Class |
|---|---|
| CountryID (PK) | A |
| CountryName | G |
| ISOCode (`UQ_Country_ISOCode`) | G |
| Description | G |
| IsActive | G |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | A |

#### `Wine.Region` — **GLOBAL**

| Column | Class |
|---|---|
| RegionID (PK) | A |
| RegionName | G |
| CountryID (FK → Country) | G |
| Description | G |
| IsActive | G |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | A |

#### `Wine.Type` — **GLOBAL**

| Column | Class |
|---|---|
| TypeID (PK) | A |
| TypeName (`UQ_Type_TypeName`) | G |
| Description | G |
| IsActive | G |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | A |

#### `Wine.Vintage` — **GLOBAL**

| Column | Class |
|---|---|
| VintageID (PK) | A |
| Year (`UQ_Vintage_Year`) | G |
| Description | G |
| IsActive | G |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | A |

#### `Wine.GrapeVariety` — **GLOBAL**

| Column | Class |
|---|---|
| GrapeVarietyID (PK) | A |
| GrapeVarietyName (`UQ_GrapeVariety_Name`) | G |
| Description | G |
| IsActive | G |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | A |

#### `Wine.FlavorProfile` — **GLOBAL**

| Column | Class |
|---|---|
| FlavorProfileID (PK) | A |
| FlavorProfileName (`UQ_FlavorProfile_Name`) | G |
| Description | G |
| IsActive | G |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | A |

### 1.3 Recipe schema

#### `Recipe.Recipe` — **GLOBAL except `IsFavorite`**

| Column | Class | Notes |
|---|---|---|
| RecipeID (PK) | A | |
| RecipeName (`UQ_Recipe_RecipeName`) | G | |
| Description | G | |
| Servings | G | |
| PrepTimeMinutes | G | |
| CookTimeMinutes | G | |
| IsActive | G | Soft-delete/publish flag of the shared recipe |
| IsFavorite | **U** | Move to `Recipe.UserRecipePreference` (added ad hoc by `LENA.Database/tmp/AddRecipeIsFavorite.sql`) |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | A | |

#### `Recipe.RecipeItem` — **GLOBAL**

| Column | Class |
|---|---|
| RecipeID (PK, FK → Recipe, CASCADE) | G |
| ItemID (PK, FK → Item) | G |
| Quantity | G |
| UnitOfMeasure | G |
| Notes | G (recipe-authoring note, not a user note) |
| IsOptional | G |

(No audit columns.)

#### `Recipe.RecipeStep` — **GLOBAL**

| Column | Class |
|---|---|
| RecipeStepID (PK) | A |
| RecipeID (FK → Recipe, CASCADE) | G |
| StepNumber (`UQ_RecipeStep_RecipeID_StepNumber`) | G |
| Instruction | G |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | A |

### 1.4 MealPlan schema — **ENTIRELY PER-USER**

Ownership is carried at the aggregate root (`MealPlan`, `GroceryList`) via a `UserID` FK; child rows inherit ownership through their cascading FK and do not need their own `UserID` column (the stored procedures must join up to the root to enforce scope).

#### `MealPlan.MealPlan` — root, gets `UserID`

| Column | Class |
|---|---|
| MealPlanID (PK) | A |
| PlanName | U |
| WeekStartDate | U |
| WeekStartDayOfWeek | U |
| IsActive | U |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | A |
| *(new)* UserID | U — owner |

#### `MealPlan.MealSlot` — owned via MealPlanID

| Column | Class |
|---|---|
| MealSlotID (PK) | A |
| MealPlanID (FK → MealPlan, CASCADE) | U (ownership path) |
| DayOfWeek | U |
| MealType | U |
| RecipeID (FK → Recipe) | U (a user's choice of a global recipe) |
| Servings | U |
| ReplacementNote | U |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | A |

Index `UQ_MealSlot_MealPlanID_DayOfWeek_MealType` already scopes uniqueness to a plan and therefore to a user — unchanged.

#### `MealPlan.MealSlotItem` — owned via MealSlotID → MealPlanID

| Column | Class |
|---|---|
| MealSlotItemID (PK) | A |
| MealSlotID (FK → MealSlot, CASCADE) | U (ownership path) |
| ItemID (FK → Item) | U (a user's choice of a global item) |
| Quantity | U |
| UnitOfMeasure | U |
| IsFromRecipe | U |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | A |

#### `MealPlan.GroceryList` — root, gets `UserID`

| Column | Class |
|---|---|
| GroceryListID (PK) | A |
| MealPlanID (FK → MealPlan, SET NULL) | U (must belong to the same user — enforce in the proc) |
| GeneratedDate | U |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | A |
| *(new)* UserID | U — owner (required even though MealPlanID exists, because MealPlanID is nullable) |

#### `MealPlan.GroceryListItem` — owned via GroceryListID

| Column | Class |
|---|---|
| GroceryListItemID (PK) | A |
| GroceryListID (FK → GroceryList, CASCADE) | U (ownership path) |
| ItemID (FK → Item, SET NULL) | U |
| ManualItemName | U |
| QuantityNeeded | U |
| UnitOfMeasure | U |
| Source | U |
| IsChecked | U |
| CreatedBy / CreateDate / LastUpdatedBy / LastUpdatedDate | A |

### 1.5 Classification summary

| Table | Verdict |
|---|---|
| Inventory.Item | Mixed → split |
| Inventory.InStock | Per-user (rework into the split target) |
| Inventory.Category, ItemBrand, flavor_profiles, food_flavors, food_nutrients, nutrient_types | Global |
| Wine.Bottle | Mixed → split |
| Wine.BottleFlavorProfile, BottleGrapeVariety | Global (definition attributes) |
| Wine.Country, Region, Type, Vintage, GrapeVariety, FlavorProfile | Global |
| Recipe.Recipe | Global except IsFavorite → extract |
| Recipe.RecipeItem, RecipeStep | Global |
| MealPlan.MealPlan, GroceryList | Per-user → add UserID |
| MealPlan.MealSlot, MealSlotItem, GroceryListItem | Per-user via parent |

---

## 2. Target schema

### 2.1 `Identity.User`

**Key type decision: `INT IDENTITY`.** Rationale:

- Every existing table already uses `INT IDENTITY` surrogate keys; a `UserID INT` FK on hundreds of thousands of per-user rows costs 4 bytes vs 16 for a GUID and keeps clustered indexes narrow and sequential.
- The external, globally-unique identifier already exists (Google `sub`); there is no need for the internal key to be globally unique or client-generatable.
- Self-hosted single-database deployment: no cross-database merge scenario that would favour GUIDs.

```sql
CREATE SCHEMA [Identity];

CREATE TABLE [Identity].[User] (
    [UserID]          INT IDENTITY(1,1) NOT NULL,
    [ExternalSubject] NVARCHAR(255)     NOT NULL,   -- Google "sub" claim; the only join key to the IdP
    [Provider]        NVARCHAR(50)      NOT NULL CONSTRAINT [DF_User_Provider] DEFAULT N'google',
    [Email]           NVARCHAR(320)     NOT NULL,   -- informational, mutable, refreshed on each login
    [DisplayName]     NVARCHAR(200)     NULL,
    [IsActive]        BIT               NOT NULL CONSTRAINT [DF_User_IsActive] DEFAULT 1,
    [LastLoginDate]   DATETIME2         NULL,
    [CreatedBy]       NVARCHAR(100)     NOT NULL,
    [CreateDate]      DATETIME2         NOT NULL,
    [LastUpdatedBy]   NVARCHAR(100)     NULL,
    [LastUpdatedDate] DATETIME2         NULL,
    CONSTRAINT [PK_User] PRIMARY KEY CLUSTERED ([UserID]),
    CONSTRAINT [UQ_User_Provider_ExternalSubject] UNIQUE ([Provider], [ExternalSubject])
);
CREATE INDEX [IX_User_Email] ON [Identity].[User] ([Email]);   -- lookup only, NOT unique
```

Notes:
- `Email` is deliberately **not** unique and **not** a join key: Google accounts can change primary email; `sub` is stable for the lifetime of the Google account.
- `Provider` is included so a second IdP can be added later without a schema change; the unique key is `(Provider, ExternalSubject)`.
- Stored procedures: `usp_User_GetByExternalSubject`, `usp_User_Upsert` (called on first authenticated request / login to create-or-refresh Email, DisplayName, LastLoginDate).
- The existing `CreatedBy`/`LastUpdatedBy` NVARCHAR audit columns are **kept as-is** across all tables. They remain a human-readable audit trail; `UserID` is the scoping dimension. Replacing them with FKs is out of scope.

### 2.2 Per-user data point → approach

| Per-user data | Approach | Target |
|---|---|---|
| MealPlan.MealPlan.* | (a) add `UserID` FK | `MealPlan.MealPlan.UserID INT NOT NULL` |
| MealPlan.MealSlot.*, MealSlotItem.* | none (inherit via MealPlanID) | procs join to MealPlan for scope |
| MealPlan.GroceryList.* | (a) add `UserID` FK | `MealPlan.GroceryList.UserID INT NOT NULL` |
| MealPlan.GroceryListItem.* | none (inherit via GroceryListID) | procs join to GroceryList for scope |
| Recipe.Recipe.IsFavorite | (b) extract | new `Recipe.UserRecipePreference` |
| Inventory.Item.{CurrentQuantity, MinQuantity, PurchaseDate, ExpiryDate, Notes, IsFavorite} | (b) extract | rework `Inventory.InStock` → `Inventory.UserItem` |
| Wine.Bottle.{BottleNumber, Quantity, PurchaseDate, PurchasePrice, StorageTemp, Location, Notes, IsFavorite} | (b) extract | new `Wine.UserBottle` |

#### 2.2.1 `Inventory.UserItem` (replaces the unused `Inventory.InStock`)

Recommendation: **drop `InStock` and create `UserItem`** rather than altering `InStock` in place. `InStock` has no consumers (no procs, repository, or features), so there is nothing to preserve, and its name (`StockID`, `QuantityOnHand`) doesn't cover favorites/notes. Keeping the DDL file name `InStock.table.sql` would be misleading.

```sql
CREATE TABLE [Inventory].[UserItem] (
    [UserItemID]      INT IDENTITY(1,1) NOT NULL,
    [UserID]          INT           NOT NULL,
    [ItemID]          INT           NOT NULL,
    [CurrentQuantity] DECIMAL(10,2) NOT NULL CONSTRAINT [DF_UserItem_CurrentQuantity] DEFAULT 0,
    [MinQuantity]     DECIMAL(10,2) NULL,
    [PurchaseDate]    DATETIME2     NULL,       -- nullable: a favorite-only row has no purchase
    [ExpiryDate]      DATETIME2     NULL,
    [Notes]           NVARCHAR(500) NULL,
    [IsFavorite]      BIT           NOT NULL CONSTRAINT [DF_UserItem_IsFavorite] DEFAULT 0,
    [CreatedBy]       NVARCHAR(100) NOT NULL,
    [CreateDate]      DATETIME2     NOT NULL,
    [LastUpdatedBy]   NVARCHAR(100) NULL,
    [LastUpdatedDate] DATETIME2     NULL,
    CONSTRAINT [PK_UserItem] PRIMARY KEY CLUSTERED ([UserItemID]),
    CONSTRAINT [UQ_UserItem_UserID_ItemID] UNIQUE ([UserID], [ItemID]),
    CONSTRAINT [FK_UserItem_User] FOREIGN KEY ([UserID]) REFERENCES [Identity].[User] ([UserID]),
    CONSTRAINT [FK_UserItem_Item] FOREIGN KEY ([ItemID]) REFERENCES [Inventory].[Item] ([ItemID]) ON DELETE CASCADE
);
CREATE INDEX [IX_UserItem_ItemID] ON [Inventory].[UserItem] ([ItemID]);
```

Semantics: one row per (user, item). "Item is in my inventory" = row exists. A row with `CurrentQuantity = 0` is "depleted" (feeds `usp_GroceryList_GenerateFromMealPlan`'s `Depleted` source). Absence of a row = user has never stocked/favorited the item. `Inventory.Item` retains only catalog columns; `PurchaseDate` moves and becomes nullable in the new home.

#### 2.2.2 `Wine.UserBottle`

`Wine.Bottle` is reduced to the static wine definition (rename to `Wine.Wine` was considered and rejected — too many artifacts reference `Bottle`; keep the table name, change its meaning to "wine definition").

```sql
CREATE TABLE [Wine].[UserBottle] (
    [UserBottleID]    INT IDENTITY(1,1) NOT NULL,
    [UserID]          INT            NOT NULL,
    [BottleID]        INT            NOT NULL,   -- FK to the static definition
    [BottleNumber]    INT            NULL,       -- user's cellar/bin number
    [Quantity]        INT            NOT NULL CONSTRAINT [DF_UserBottle_Quantity] DEFAULT 1,
    [PurchaseDate]    DATETIME2      NULL,
    [PurchasePrice]   DECIMAL(10,2)  NULL,
    [StorageTemp]     DECIMAL(5,1)   NULL,
    [Location]        NVARCHAR(100)  NULL,
    [Notes]           NVARCHAR(500)  NULL,
    [IsFavorite]      BIT            NOT NULL CONSTRAINT [DF_UserBottle_IsFavorite] DEFAULT 0,
    [CreatedBy]       NVARCHAR(100)  NOT NULL,
    [CreateDate]      DATETIME2      NOT NULL,
    [LastUpdatedBy]   NVARCHAR(100)  NULL,
    [LastUpdatedDate] DATETIME2      NULL,
    CONSTRAINT [PK_UserBottle] PRIMARY KEY CLUSTERED ([UserBottleID]),
    CONSTRAINT [FK_UserBottle_User] FOREIGN KEY ([UserID]) REFERENCES [Identity].[User] ([UserID]),
    CONSTRAINT [FK_UserBottle_Bottle] FOREIGN KEY ([BottleID]) REFERENCES [Wine].[Bottle] ([BottleID]) ON DELETE CASCADE
);
CREATE INDEX [IX_UserBottle_UserID_BottleID] ON [Wine].[UserBottle] ([UserID], [BottleID]);
CREATE INDEX [IX_UserBottle_UserID_IsFavorite_BottleNumber] ON [Wine].[UserBottle] ([UserID], [IsFavorite], [BottleNumber]);
```

Deliberate design choice: **no** unique constraint on `(UserID, BottleID)`. A cellar legitimately holds the same wine bought on different dates at different prices in different locations (lots). If the product decision is "one holding row per wine per user", add `UQ_UserBottle_UserID_BottleID` and drop the `IX_UserBottle_UserID_BottleID` index; the current UI (a single Quantity per bottle row) is compatible with either.

`IsFavorite` is placed on the holding rather than in a separate `UserWinePreference` table because, unlike recipes, a user has no reason to favorite a wine they don't hold; if that changes, extract it later.

Optional: `UQ_UserBottle_UserID_BottleNumber` if bin numbers must be unique within a user's cellar (they are currently not unique at all).

#### 2.2.3 `Recipe.UserRecipePreference`

```sql
CREATE TABLE [Recipe].[UserRecipePreference] (
    [UserID]          INT           NOT NULL,
    [RecipeID]        INT           NOT NULL,
    [IsFavorite]      BIT           NOT NULL CONSTRAINT [DF_UserRecipePreference_IsFavorite] DEFAULT 0,
    -- future per-user preferences (e.g. Rating TINYINT NULL, PersonalNotes NVARCHAR(500) NULL, LastCookedDate DATETIME2 NULL) go here
    [CreatedBy]       NVARCHAR(100) NOT NULL,
    [CreateDate]      DATETIME2     NOT NULL,
    [LastUpdatedBy]   NVARCHAR(100) NULL,
    [LastUpdatedDate] DATETIME2     NULL,
    CONSTRAINT [PK_UserRecipePreference] PRIMARY KEY CLUSTERED ([UserID], [RecipeID]),
    CONSTRAINT [FK_UserRecipePreference_User] FOREIGN KEY ([UserID]) REFERENCES [Identity].[User] ([UserID]),
    CONSTRAINT [FK_UserRecipePreference_Recipe] FOREIGN KEY ([RecipeID]) REFERENCES [Recipe].[Recipe] ([RecipeID]) ON DELETE CASCADE
);
```

Natural composite PK; row absent ⇒ all preferences default. `Recipe.Recipe.IsFavorite` is dropped.

#### 2.2.4 `MealPlan.MealPlan` / `MealPlan.GroceryList` — add `UserID`

```sql
ALTER TABLE [MealPlan].[MealPlan]    ADD [UserID] INT NOT NULL CONSTRAINT [FK_MealPlan_User]    FOREIGN KEY REFERENCES [Identity].[User]([UserID]);
ALTER TABLE [MealPlan].[GroceryList] ADD [UserID] INT NOT NULL CONSTRAINT [FK_GroceryList_User] FOREIGN KEY REFERENCES [Identity].[User]([UserID]);
CREATE INDEX [IX_MealPlan_UserID]    ON [MealPlan].[MealPlan]    ([UserID], [WeekStartDate]);
CREATE INDEX [IX_GroceryList_UserID] ON [MealPlan].[GroceryList] ([UserID], [GeneratedDate]);
```

(Illustrative — the actual `.table.sql` files are declarative `CREATE TABLE` and will be edited to include the column; the backfill migration in §5 handles existing rows.) No `ON DELETE CASCADE` from `User`: deleting a user is an explicit, rare operation and should be handled by a `usp_User_Delete` that removes per-user rows deliberately.

### 2.3 Uniqueness / constraint implications

| Constraint | Before | After |
|---|---|---|
| `UQ_Item_Name_BrandID`, `UQ_Item_UPC12`, `UQ_Item_UPC14` | on Item | **unchanged, global** — a UPC identifies one catalog product for everyone |
| Item favorite uniqueness | implicit (1 row) | `UQ_UserItem_UserID_ItemID` — one preference/stock row per user per item |
| `UQ_Recipe_RecipeName` | on Recipe | unchanged, global |
| Recipe favorite uniqueness | implicit | `PK_UserRecipePreference (UserID, RecipeID)` |
| Bottle identity | none | none on the definition. Consider a future `UQ_Bottle_Definition (TypeID, CountryID, RegionID, VintageYear, Vineyard, BottleSize)` to deduplicate definitions across users — **not** recommended now because `Vineyard` is free text and existing rows may collide only after backfill |
| `IX_Bottle_IsFavorite_BottleNumber` | on Bottle | dropped; replaced by `IX_UserBottle_UserID_IsFavorite_BottleNumber` |
| `IX_Bottle_{CountryID,RegionID,TypeID,VintageYear}_BottleNumber` | on Bottle | keep the leading key, drop `BottleNumber` (column moves) |
| `UQ_MealSlot_MealPlanID_DayOfWeek_MealType` | scoped by plan | unchanged (plan is now user-owned) |
| `Wine.Vintage.UQ_Vintage_Year`, `Country.UQ_Country_ISOCode`, `Type.UQ_Type_TypeName`, `Category.UQ_Category_CategoryName`, `ItemBrand.UQ_ItemBrand_Name`, `flavor_profiles/nutrient_types` UNIQUE names, `FlavorProfile`/`GrapeVariety` name UQs | global | unchanged |
| `GroceryList.MealPlanID` | any plan | procs must assert `MealPlan.UserID = GroceryList.UserID` (cannot be expressed as a plain FK without denormalizing UserID onto the FK — accept proc-level enforcement) |
| `MealSlot.RecipeID`, `MealSlotItem.ItemID`, `GroceryListItem.ItemID`, `RecipeItem.ItemID` | reference global rows | unchanged — pointing at global catalog rows is fine |

Cascade behaviour: deleting a global `Item`/`Bottle`/`Recipe` cascades into the corresponding `User*` table. Because catalog rows become shared, **delete must become an authorization concern** (a user deleting "their" item today would delete it for everyone tomorrow). The initial multi-user phases should either restrict catalog delete to an admin role or convert catalog delete into "remove my UserItem row" (recommended for Items and Bottles from the UI's perspective).

### 2.4 Authorization model for global data (explicit non-goal, stated for clarity)

Global tables stay writable by any authenticated user in the first multi-user release (collaborative shared catalog). Role-based restriction of catalog writes is a separate, later ADR.

---

## 3. Impact map (per domain)

Legend for stored procedures: **S** = add `@UserID` and scope; **R** = rewrite to join/insert the new `User*` table; **U** = unaffected (global). Only the affected artifacts are listed; procedures on purely global tables (Country/Region/Type/Vintage/Category/FlavorProfile/FoodFlavor/FoodNutrient/NutrientType, RecipeItem, RecipeStep) are **U** and are not repeated.

### 3.0 Identity (new)

| Artifact type | Items |
|---|---|
| Tables | `LENA.Database/Identity/Schema.sql`, `LENA.Database/Identity/Tables/User.table.sql` (new) |
| Stored procedures | `usp_User_GetByExternalSubject`, `usp_User_Upsert` (new) |
| Repositories | `UserRepository.cs` + `Contracts/Persistence/IUserRepository.cs` (new) |
| Features | `Features/Identity/Users/Commands/UpsertUserCommand.cs`, `Queries/GetCurrentUserQuery.cs` (new) |
| Entities | `LENA/Entity/Identity/User.cs` (new) |
| API | `LENA.API/Services/HttpContextCurrentUserService.cs`, `Program.cs` (see §4), `Controllers/AuthController.cs` (`/me` endpoint) |
| Frontend | `frontend/lib/types.ts` (`User`), `frontend/lib/api.ts` (`auth.me`) |
| Docker/db-init | the db-init service (PR #15) must apply the new schema folder |

### 3.1 MealPlan / Grocery

| Artifact type | Items |
|---|---|
| Tables | `MealPlan/Tables/MealPlan.table.sql` (+UserID), `MealPlan/Tables/GroceryList.table.sql` (+UserID); `MealSlot`, `MealSlotItem`, `GroceryListItem` unchanged. New indexes `IX_MealPlan_UserID`, `IX_GroceryList_UserID` |
| Stored procedures (S) | `usp_MealPlan_Create`, `usp_MealPlan_Update`, `usp_MealPlan_Delete`, `usp_MealPlan_GetById`, `usp_MealPlan_GetByName`, `usp_MealPlan_ListAll`, `usp_MealPlan_ListAllPaged`, `usp_MealPlan_GetNutrition`, `usp_MealSlot_Create`, `usp_MealSlot_Update`, `usp_MealSlot_Delete`, `usp_MealSlot_GetByMealPlanId`, `usp_MealSlotItem_Create`, `usp_MealSlotItem_Delete`, `usp_MealSlotItem_GetBySlotId`, `usp_GroceryList_Create`, `usp_GroceryList_Delete`, `usp_GroceryList_GetById`, `usp_GroceryList_ListAll`, `usp_GroceryList_GenerateFromMealPlan` (also **R** later: its `Depleted` and on-hand netting read `Item.CurrentQuantity` → `UserItem`), `usp_GroceryListItem_Create`, `usp_GroceryListItem_Delete`, `usp_GroceryListItem_GetByGroceryListId`, `usp_GroceryListItem_ToggleChecked` |
| Repositories | `MealPlanRepository.cs`, `GroceryListRepository.cs`; contracts `IMealPlanRepository.cs`, `IGroceryListRepository.cs` |
| Features | everything under `Features/MealPlan/**` (MealPlans, MealSlots, MealSlotItems, `Queries/GetMealPlanNutritionQuery.cs`) and `Features/Grocery/GroceryLists/**` |
| Entities | `LENA/Entity/MealPlan/MealPlan.cs`, `LENA/Entity/Grocery/GroceryList.cs` (+UserID) |
| Frontend | `types.ts`: `MealPlan`, `GroceryList` (+`userId`, optional to expose); `api.ts`: `mealPlans.*`, `groceryLists.*` — no URL changes needed, server scopes by token |
| Mobile | `mobile/lib/services/api_service.dart` (`GET /api/GroceryList/{id}`) — no contract change beyond auth |

### 3.2 Recipe

| Artifact type | Items |
|---|---|
| Tables | `Recipe/Tables/Recipe.table.sql` (drop IsFavorite), `Recipe/Tables/UserRecipePreference.table.sql` (new); remove `LENA.Database/tmp/AddRecipeIsFavorite.sql` |
| Stored procedures (R) | `usp_Recipe_Create` (stop writing IsFavorite), `usp_Recipe_Update` (same), `usp_Recipe_GetById`, `usp_Recipe_ListAll`, `usp_Recipe_ListAllPaged` (LEFT JOIN UserRecipePreference on @UserID; `@IsFavorite` filter moves to the join), `usp_Recipe_GetByName`; new `usp_Recipe_SetFavorite` (MERGE into UserRecipePreference) |
| Repositories | `RecipeRepository.cs`, `IRecipeRepository.cs` (pass UserID to reads; add SetFavorite) |
| Features | `Features/Recipe/Recipes/Commands/CreateRecipeCommand.cs`, `UpdateRecipeCommand.cs` (remove IsFavorite), new `SetRecipeFavoriteCommand.cs`; `Queries/GetRecipeByIdQuery.cs`, `GetRecipesQuery.cs`, `GetRecipesPagedQuery.cs`; validators `CreateRecipeCommandValidator.cs`, `UpdateRecipeCommandValidator.cs` |
| Entities | `LENA/Entity/Recipe/Recipe.cs` (IsFavorite becomes a projected, read-only per-user field or moves to a `RecipeDto`), new `LENA/Entity/Recipe/UserRecipePreference.cs` |
| API | `RecipeController.cs` — add `PUT /api/Recipe/recipes/{id}/favorite` (mirrors Item) |
| Frontend | `types.ts`: `Recipe.isFavorite` stays on the read model; `api.ts`: `recipes.create/update` payloads drop `isFavorite`, add `recipes.setFavorite(id, isFavorite)`; recipe pages that toggle favorite via `update` must switch |

### 3.3 Inventory

| Artifact type | Items |
|---|---|
| Tables | `Inventory/Tables/Item.table.sql` (drop CurrentQuantity, MinQuantity, PurchaseDate, ExpiryDate, Notes, IsFavorite), delete `Inventory/Tables/InStock.table.sql`, new `Inventory/Tables/UserItem.table.sql`; `Category`, `ItemBrand`, `flavor_profiles`, `food_flavors`, `food_nutrients`, `nutrient_types` unchanged |
| Stored procedures (R) | `usp_Item_Create` (split insert: Item catalog + UserItem for @UserID), `usp_Item_Update` (split update), `usp_Item_Delete` (becomes "remove UserItem" for non-admin, see §2.3), `usp_Item_GetById`, `usp_Item_GetByName`, `usp_Item_ListAll`, `usp_Item_ListAllPaged` (LEFT JOIN UserItem; `@InStock`/`@IsFavorite` filters move to the join), `usp_Item_Search`, `usp_Item_GetBrands` (its `CurrentQuantity > 0` branch → UserItem), `usp_Item_AdjustQuantity` (UPSERT UserItem), `usp_Item_SetFavorite` (UPSERT UserItem). **U**: `usp_Item_AddOrUpdateUPC12`, `usp_Item_AddOrUpdateUPC14`, `usp_Item_ChangeItemCategory`, all `usp_FlavorProfile_*`, `usp_FoodFlavor_*`, `usp_FoodNutrient_*`, `usp_NutrientType_*`. Cross-schema: `MealPlan.usp_GroceryList_GenerateFromMealPlan` (reads `Item.CurrentQuantity`/`MinQuantity`); `MealPlan.usp_MealPlan_GetNutrition` joins `Item` only for catalog columns and is **U** for this split |
| Repositories | `ItemRepository.cs`, `IItemRepository.cs` |
| Features | `Features/Inventory/Items/Commands/CreateItemCommand.cs`, `UpdateItemCommand.cs`, `AdjustItemQuantityCommand.cs`, `SetItemFavoriteCommand.cs`, `DeleteItemCommand.cs`; `Queries/GetItemByIdQuery.cs`, `GetItemByNameQuery.cs`, `GetItemsQuery.cs`, `GetItemsPagedQuery.cs`, `SearchItemsQuery.cs`, `GetItemBrandsQuery.cs`; validators `CreateItemCommandValidator.cs`, `UpdateItemCommandValidator.cs` (PurchaseDate no longer required) |
| Entities | `LENA/Entity/Inventory/Item.cs` (catalog only, or keep as a flattened read model), delete `InStock.cs`, new `UserItem.cs` |
| API | `ItemController.cs` (endpoints keep their routes; `DELETE items/{id}` semantics change) |
| Frontend | `types.ts`: `Item` (per-user fields become nullable when the user has no row), delete `InStock`; `api.ts`: `items.create/update` payload shape, `items.adjustQuantity`, `items.setFavorite` unchanged in URL |
| Mobile | `mobile/lib/services/api_service.dart` (`POST /api/Item/items/{id}/quantity`) — unchanged contract |

### 3.4 Wine

| Artifact type | Items |
|---|---|
| Tables | `Wine/Tables/Bottle.table.sql` (drop BottleNumber, Quantity, PurchaseDate, PurchasePrice, StorageTemp, Location, Notes, IsFavorite), new `Wine/Tables/UserBottle.table.sql`; `Wine/Indexes/IX_Bottle_IsFavorite_BottleNumber.sql` (delete), `IX_Bottle_CountryID_BottleNumber.sql`, `IX_Bottle_RegionID_BottleNumber.sql`, `IX_Bottle_TypeID_BottleNumber.sql`, `IX_Bottle_VintageYear_BottleNumber.sql` (drop BottleNumber key part), new `IX_UserBottle_*`; `BottleFlavorProfile`, `BottleGrapeVariety`, `Country`, `Region`, `Type`, `Vintage`, `GrapeVariety`, `FlavorProfile` unchanged |
| Stored procedures (R) | `usp_Bottle_Create`, `usp_Bottle_Update`, `usp_Bottle_Delete`, `usp_Bottle_GetById`, `usp_Bottle_GetByName`, `usp_Bottle_ListAll`, `usp_Bottle_ListAllPaged`, `usp_Bottle_SearchBottles`, `usp_Bottle_GetFavorites`, `usp_Bottle_GetTotalBottleCount` (sum of the user's `UserBottle.Quantity`), `usp_Bottle_GetAllByCountryId`, `usp_Bottle_GetAllByRegionId`, `usp_Bottle_GetAllByTypeId`, `usp_Bottle_GetAllByVintageYear` (all list procs: INNER JOIN UserBottle on @UserID — a "bottle list" is the user's cellar). **U**: all `usp_Country_*`, `usp_Region_*`, `usp_Type_*`, `usp_Vintage_*` |
| Repositories | `BottleRepository.cs`, `IBottleRepository.cs` |
| Features | all of `Features/Wine/Bottles/**` (Commands `CreateBottleCommand.cs`, `UpdateBottleCommand.cs`, `DeleteBottleCommand.cs`; Queries `GetBottleByIdQuery.cs`, `GetBottlesQuery.cs`, `GetBottlesPagedQuery.cs`, `GetBottlesByCountryIdQuery.cs`, `GetBottlesByRegionIdQuery.cs`, `GetBottlesByTypeIdQuery.cs`, `GetBottlesByVintageYearQuery.cs`, `GetFavoriteBottlesQuery.cs`, `GetTotalBottleCountQuery.cs`, `SearchBottlesQuery.cs`; validators `CreateBottleCommandValidator.cs`, `UpdateBottleCommandValidator.cs`) |
| Entities | `LENA/Entity/Wine/Bottle.cs` (definition only, or flattened read model), new `UserBottle.cs` |
| API | `BottleController.cs` |
| Frontend | `types.ts`: `Bottle` (holding fields nullable / moved to a nested `holding`), `api.ts`: `bottles.*` payload shapes |

### 3.5 Cross-cutting

| Artifact | Change |
|---|---|
| `LENA.Application/Repositories/BaseRepository.cs` | optional helper to inject `UserID` into every parameter bag |
| `LENA.Application/Behaviors/AuditingBehavior.cs` | unchanged (still stamps UserName) |
| `LENA.API.UnitTests`, `LENA.Application.UnitTests`, `LENA.IntegrationTests` | every repository/handler test that constructs commands or fakes `ICurrentUserService` |
| `LENA.Database/tmp/*.sql` | `AddRecipeIsFavorite.sql`, `MigrateToItemBrand.sql`, `GroceryItems.sql` — superseded by the versioned migration in §5 |
| `docker-compose.yml` db-init | apply `Identity/` before `Inventory/`, `Wine/`, `Recipe/`, `MealPlan/` (FK ordering) |

---

## 4. Identity plumbing (specification only)

`LENA.Application/Contracts/Auditing/ICurrentUserService.cs` currently exposes only:

```csharp
string UserName { get; }
```

It must gain:

```csharp
public interface ICurrentUserService
{
    string UserName { get; }             // unchanged: email (NameClaimType) or "system"
    int UserID { get; }                  // internal Identity.User.UserID; throws / is 0 for anonymous
    string? ExternalSubject { get; }     // raw Google "sub" claim, for diagnostics and the upsert path
}
```

Resolution flow (to be implemented in a later phase, in `LENA.API`):

1. JWT middleware validates the Google token (Phase 1). `ClaimTypes.NameIdentifier` / `"sub"` carries the subject.
2. A scoped service (or a middleware placed after `UseAuthentication`) reads `sub` + `email` + `name` from `HttpContext.User`, calls `usp_User_Upsert` (insert if unknown, otherwise refresh `Email`/`DisplayName`/`LastLoginDate`), and caches the resulting `UserID` in `HttpContext.Items` for the lifetime of the request. A short-lived in-memory cache keyed by `sub` avoids one DB round-trip per request.
3. `HttpContextCurrentUserService.UserID` returns that cached value. For unauthenticated requests (only possible if an endpoint is `[AllowAnonymous]`) it must not silently fall back to a default user — throw or return a sentinel that stored procedures reject.
4. Repositories pass `UserID = _currentUser.UserID` in the Dapper parameter object for every scoped procedure. Handlers do **not** accept `UserID` from the request body/route; it is always server-derived.
5. `usp_*` procedures treat `@UserID` as mandatory (`NOT NULL`) and filter/insert with it. Row-level enforcement lives in SQL so that a forgotten filter in C# cannot leak data.

Decision: keep `UserName` (email) for the existing `CreatedBy`/`LastUpdatedBy` audit columns; do not change `AuditableEntity`.

---

## 5. Recommended phase ordering and backfill strategy

Ordered easiest-first; each phase is independently shippable and leaves the system working.

| Phase | Scope | Why this order |
|---|---|---|
| **2** | `Identity` schema + `User` table + upsert on login + `ICurrentUserService.UserID` | Prerequisite for everything; zero impact on existing tables |
| **3** | MealPlan / GroceryList: add `UserID`, scope 24 procs, repositories, features | Pure "add a column + WHERE clause" — no table split, no contract change |
| **4** | Recipe preferences: `UserRecipePreference`, drop `Recipe.IsFavorite`, new SetFavorite endpoint | Small extraction, one column, isolated frontend change |
| **5** | Inventory split: `UserItem`, slim `Item`, rewrite 12 Item procs + `GenerateFromMealPlan` netting | Larger, but Item procs are already parameter-driven and the UI already has quantity/favorite endpoints |
| **6** | Wine split: `UserBottle`, slim `Bottle`, rewrite 14 Bottle procs and 6 indexes | Largest surface area; depends on lessons from Phase 5 |
| **7** | Data backfill (see below) — executed incrementally as part of Phases 3–6, but tracked as its own deliverable with verification queries | |
| **8** | Frontend + Mobile: nullable per-user fields, new favorite endpoint, `/me`, remove `InStock` type | Contracts stabilise only after 3–6 |

### 5.1 Backfill strategy: assign existing rows to a single default user

All current data was created by one operator. The migration for each phase runs inside a transaction and follows this pattern:

1. **Seed the default user (Phase 2).**
   ```sql
   INSERT INTO [Identity].[User] (ExternalSubject, Provider, Email, DisplayName, CreatedBy, CreateDate)
   VALUES (@DefaultSub, N'google', @DefaultEmail, N'Default', N'migration', SYSUTCDATETIME());
   ```
   `@DefaultSub` is the real Google `sub` of the current operator (obtained from one decoded token) so the backfilled data is immediately visible after login. If unknown at migration time, insert a placeholder `sub` (`'legacy-default'`) and update it on the operator's first login by matching `Email`; this is the **only** time email is used to link identities.

2. **Column-add phases (MealPlan, GroceryList).** Add `UserID INT NULL` → `UPDATE ... SET UserID = @DefaultUserID` → `ALTER COLUMN UserID INT NOT NULL` → add FK/index.

3. **Extraction phases (Recipe, Item, Bottle).** Insert one `User*` row per existing parent row, copying the per-user columns verbatim, then drop the columns from the parent:
   ```sql
   INSERT INTO [Inventory].[UserItem] (UserID, ItemID, CurrentQuantity, MinQuantity, PurchaseDate, ExpiryDate, Notes, IsFavorite, CreatedBy, CreateDate, LastUpdatedBy, LastUpdatedDate)
   SELECT @DefaultUserID, ItemID, CurrentQuantity, MinQuantity, PurchaseDate, ExpiryDate, Notes, IsFavorite, CreatedBy, CreateDate, LastUpdatedBy, LastUpdatedDate
   FROM [Inventory].[Item];
   -- then: ALTER TABLE [Inventory].[Item] DROP COLUMN CurrentQuantity, MinQuantity, PurchaseDate, ExpiryDate, Notes, IsFavorite;
   ```
   Same shape for `Wine.Bottle → Wine.UserBottle` (1:1, one holding per definition) and `Recipe.Recipe → Recipe.UserRecipePreference` (only rows where `IsFavorite = 1`, since absent row = default).
   Drop/re-create the affected `IX_Bottle_*` indexes in the same script.

4. **Verification queries** (must return 0 before commit): rows in any scoped table with `UserID IS NULL`; `COUNT(Item) <> COUNT(UserItem)`; `COUNT(Bottle) <> COUNT(UserBottle)`; `SUM(old Bottle.Quantity) <> SUM(UserBottle.Quantity)` (captured before the drop).

5. **Housekeeping.** Retire `LENA.Database/tmp/*.sql`; the db-init service applies the versioned scripts.

### 5.2 Rollback

Each phase's migration ships with a reverse script that re-adds the dropped columns and copies values back from the `User*` table filtered to `@DefaultUserID`. Rollback is only valid while a single user exists; once a second user has data, rollback becomes a data-loss operation and is not supported.

---

## Consequences

- Global catalog data (Items, Bottle definitions, Recipes, all reference tables) becomes a shared collaborative dataset; per-user state is cleanly isolated behind `UserID` enforced in stored procedures.
- Two tables (`Item`, `Bottle`) change meaning from "my stock" to "product definition"; their DTOs will expose the current user's holding as nullable/joined fields to keep the API surface stable.
- Catalog delete needs an authorization decision before Phase 5 (see §2.3).
- Audit columns remain free-text email strings; `UserID` is the only scoping key.
