# LENA — Architecture, Data Flow & Functional Map

*A personal, self-hosted kitchen-management system: pantry, recipes, meal plans, grocery lists, and a wine cellar.*

---

## 1. Executive Summary

LENA is a privacy-first, multi-client, single-tenant web service. The backend is an **ASP.NET Core Web API** built around **Clean Architecture** with **CQRS via MediatR**, **Dapper + SQL Server stored procedures**, and **Google JWT bearer authentication**. It is consumed by a **Next.js 16** admin web app and a **Flutter** mobile app. All data lives in a locally hosted **MS SQL Server** database and is owned by individual Google-authenticated users.

---

## 2. System Context & Technology Stack

### 2.1 Physical / Runtime Deployment

```
┌─────────────────────────────────────────────────────────────┐
│                    Client Browsers / Flutter                │
│                    (Google sign-in → ID token)              │
└───────────────────────┬─────────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────────┐
│  Caddy reverse proxy  │  /api/* → api:8080                  │
│  (localhost in Docker)│  all else → ui:3000                 │
└───────────────────────┬─────────────────────────────────────┘
        │                                │
┌───────▼────────┐              ┌────────▼────────────┐
│ LENA.API       │              │ frontend (Next.js)  │
│ ASP.NET Core   │              │ Material UI,        │
│ net10.0        │              │ React Query         │
└───────┬────────┘              └─────────────────────┘
        │
┌───────▼──────────────┐   ┌──────────────────────────┐
│  LENA.Application    │   │  LENA.Infrastructure     │
│  MediatR, Validators,│   │  Dapper, SqlConnection,  │
│  Repository contracts│   │  concrete repositories   │
└───────┬──────────────┘   └──────────┬───────────────┘
        │                             │
        └─────────────┬───────────────┘
                      │
          ┌───────────▼──────────────┐
          │  LENA.Domain             │
          │  Plain entities          │
          └───────────┬──────────────┘
                      │
          ┌───────────▼──────────────┐
          │  MS SQL Server (Docker)  │
          │  Schemas + stored procs  │
          └──────────────────────────┘
```

### 2.2 Core Technologies

| Layer | Technology |
|---|---|
| Database | MS SQL Server 2022, schema-per-domain, stored-procedure data access |
| Backend | ASP.NET Core 10, MediatR 12, FluentValidation, Dapper, Serilog |
| Frontend | Next.js 16 App Router, TypeScript, Material UI 9, React Query 5, `@react-oauth/google` |
| Mobile | Flutter, `google_sign_in`, `flutter_secure_storage`, `http` |
| Hosting (local) | Docker Compose + Caddy reverse proxy + Seq for log aggregation |
| Auth | Google OAuth 2.0 ID tokens validated as JWT bearer tokens |

---

## 3. Logical Architecture

### 3.1 Backend Layers

The .NET solution follows Clean Architecture:

| Project | Responsibility |
|---|---|
| `LENA` (`LENA.Domain.csproj`) | Plain C# entities; no framework dependencies. |
| `LENA.Application` | MediatR commands/queries, validators, repository **contracts**, pipeline behaviors. |
| `LENA.Infrastructure` | Dapper `DbConnectionFactory`, concrete repositories, schema-scoped stored-procedure callers. |
| `LENA.API` | Controllers, auth wiring, CORS, Swagger/OpenAPI, global exception handling, middleware. |

### 3.2 Dependency Direction

- `LENA.API` → `LENA.Application` and `LENA.Infrastructure`
- `LENA.Infrastructure` → `LENA.Application` and `LENA.Domain`
- `LENA.Application` → `LENA.Domain`

This keeps the Domain and Application free of SQL/Dapper knowledge.

---

## 4. Domain & Data Model

### 4.1 Schemas

The SQL database is organized into domain schemas:

- `Identity` — users and authentication
- `Inventory` — food items, categories, brands, nutrients, flavor profiles
- `Recipe` — recipes, ingredients, steps
- `MealPlan` — meal plans, slots, slot items, grocery lists
- `Wine` — bottles, types, countries, regions, vintages, grape varieties

### 4.2 Multi-User Data Model

User ownership is a first-class concern. The ADR `0002-multi-user-data-model.md` classifies every table as **Global (G)**, **Per-User (U)**, or **Audit (A)**:

- **Global**: reference catalogs (`Category`, `ItemBrand`, `Wine.Country`, `Wine.Type`, `Recipe`, etc.) shared across users.
- **Per-User**: `MealPlan`, `GroceryList`, `UserItem`, `UserBottle`, and user-specific preferences.
- **Audit**: `CreatedBy`, `CreateDate`, `LastUpdatedBy`, `LastUpdatedDate` on all entities.

### 4.3 Key Entities

| Entity | What it models |
|---|---|
| `Item` / `UserItem` | Food catalog + a user’s on-hand quantity, min threshold, expiry, notes, favorite |
| `Recipe` / `RecipeItem` / `RecipeStep` | Recipe header, scaled ingredients, instructions |
| `MealPlan` / `MealSlot` / `MealSlotItem` | Weekly plan, daily slots, planned recipes and ad-hoc items |
| `GroceryList` / `GroceryListItem` | Auto-generated shopping list from a meal plan, with check-off state |
| `Bottle` / `UserBottle` | Wine catalog definition + a user’s cellar holding |
| `User` | Google `sub` → internal `UserID` mapping |

### 4.4 Stored-Procedure-First Data Access

All persistence goes through `[Schema].[usp_*]` stored procedures. Repositories map entities to Dapper `DynamicParameters` and call the database.

---

## 5. Backend Components Deep Dive

### 5.1 ASP.NET Core API Entry Point

`Program.cs` is the composition root:

- OpenAPI / Swagger (gated to authenticated users outside dev)
- CORS with explicit allowed origins
- JWT bearer auth via Google (`accounts.google.com`)
- Fallback authorization (secure-by-default)
- FluentValidation and MediatR with pipeline behaviors
- Infrastructure service registration

### 5.2 Authentication & User Resolution

1. Client obtains a Google ID token.
2. Browser / mobile sends `Authorization: Bearer <id_token>`.
3. API validates the JWT (issuer `accounts.google.com` or `https://accounts.google.com`, audience = configured Google Client ID).
4. `UserResolutionMiddleware` upserts the `Identity.User` row (cached by Google `sub`) and stores the internal `UserID` in `HttpContext.Items`.
5. `HttpContextCurrentUserService` exposes `UserName`, `UserID`, and `ExternalSubject` to handlers.

### 5.3 MediatR Command/Query Pipeline

Every feature is a vertical slice: a record (command or query) + handler + validator.

Cross-cutting concerns are handled by four MediatR pipeline behaviors, executed in order:

1. `CachingBehavior` — short-circuits `ICacheableQuery` via `IMemoryCache`
2. `LoggingBehavior` — logs request type and result
3. `ValidationBehavior` — runs FluentValidation validators
4. `AuditingBehavior` — stamps `CreatedBy` / `CreateDate` / `LastUpdatedBy` / `LastUpdatedDate` on `AuditableEntity` commands

### 5.4 Repositories & Dapper

Concrete repositories live in `LENA.Infrastructure.Persistence` and implement contracts defined in `LENA.Application.Contracts.Persistence`. Each repository:

- Inherits `BaseRepository<T>` with helpers for stored-procedure list, single, paged, and execute operations.
- Uses `IDbConnectionFactory` to create `SqlConnection`s per request.
- Receives `ICurrentUserService` to pass `UserID` into stored procedures.

### 5.5 Global Exception Handling

Unhandled exceptions are converted to RFC 7807 `ProblemDetails` with appropriate status codes:

| Exception | HTTP Status |
|---|---|
| `ValidationException` | 400 |
| `NotFoundException` | 404 |
| `UnauthenticatedUserException` | 401 |
| `OperationCanceledException` / client closed | 499 |
| Other | 500 |

### 5.6 Controllers as Thin Adapters

Controllers bind request/response DTOs, dispatch MediatR messages, and return HTTP responses.

---

## 6. Frontend Architecture

### 6.1 Stack & Responsibilities

- **Next.js 16 App Router**: routes under `frontend/app/`
- **Material UI (MUI)**: admin-style responsive drawer, tables, dialogs
- **React Query (`@tanstack/react-query`)**: server-state, caching, retries
- **Google OAuth (`@react-oauth/google`)**: sign-in and ID token
- **Custom `lib/api.ts`**: thin, typed fetch wrapper

### 6.2 Auth Flow

1. `LoginScreen` uses the Google login button to obtain a credential.
2. `AuthProvider` decodes the JWT payload, validates expiry, stores the token in `localStorage` under `lena_id_token`.
3. `lib/api.ts` attaches the token as `Authorization: Bearer <token>` to every request.
4. On 401, `onUnauthorized` fires and the user is signed out.

### 6.3 Provider Composition

The root layout wraps the app in:

- `AuthProvider`
- `GoogleOAuthProvider`
- `QueryClientProvider`
- `ThemeProvider` / `CssBaseline`
- `AdminLayout` with navigation drawer

### 6.4 Frontend Pages (Functional Surface)

| Route | Domain |
|---|---|
| `/` | Dashboard |
| `/inventory/items` | Food inventory |
| `/inventory/flavor-profiles` | Flavor profile reference data |
| `/inventory/food-flavors` | Item–flavor associations |
| `/inventory/food-nutrients` | Item–nutrient associations |
| `/inventory/nutrient-types` | Nutrient reference data |
| `/wine/bottles` | Wine cellar |
| `/wine/countries`, `/wine/regions`, `/wine/types`, `/wine/vintages` | Wine reference data |
| `/recipes` | Recipe management |
| `/meal-plans` | Weekly meal planning |
| `/grocery-lists` | Shopping lists |

---

## 7. Mobile Architecture

The mobile client is a Flutter app that replaces a previous native Android app. It currently supports:

- Google sign-in with `flutter_secure_storage` token persistence
- A minimal API service that sends `Authorization: Bearer` headers and handles 401
- A grocery-list screen for parity with the old Android app

---

## 8. Data Flow

### 8.1 Typical HTTP Request Lifecycle

```
Browser/Flutter
    │
    ▼
Caddy (Docker) routes /api/* → API
    │
    ▼
JWT Bearer auth → Google JWT validation
    │
    ▼
UserResolutionMiddleware
  ├─ cache lookup by "sub"
  └─ on miss: Mediator.Send(UpsertUserCommand)
    │
    ▼
Authorization (fallback policy)
    │
    ▼
Controller action
    │
    ▼
IMediator.Send(Command | Query)
    │
    ▼
MediatR Pipeline
  ├─ CachingBehavior
  ├─ LoggingBehavior
  ├─ ValidationBehavior
  └─ AuditingBehavior
    │
    ▼
Handler
    │
    ▼
Repository (Dapper) → [Schema].[usp_*]
    │
    ▼
SQL Server
```

### 8.2 Example: Generating a Grocery List

1. Frontend calls `POST /api/GroceryList/generate?mealPlanId=...`
2. `GroceryListController` dispatches `GenerateGroceryListCommand`
3. Handler sets `GeneratedDate` and calls `IGroceryListRepository.GenerateFromMealPlanAsync`
4. Repository executes `[MealPlan].[usp_GroceryList_GenerateFromMealPlan]`
5. Stored procedure:
   - Inserts a `GroceryList` row
   - Aggregates required recipe ingredients scaled by `MealSlot.Servings`
   - Adds selected optional ingredients
   - Adds ad-hoc `MealSlotItem` entries
   - Nets off current `UserItem` stock
   - Adds “Depleted” items below minimum quantity
   - Returns the list and its items

### 8.3 Example: Meal Plan Nutrition

`GET /api/MealPlan/plans/{id}/nutrition` flows through `GetMealPlanNutritionQuery` to `MealPlanRepository.GetMealPlanNutritionAsync`, which executes `[MealPlan].[usp_MealPlan_GetNutrition]`. The procedure scales recipe ingredient quantities by slot servings, joins to `food_nutrients`, and emits per-meal and daily nutrient totals.

---

## 9. Functional Mapping

### 9.1 Inventory

| Feature | Backend | Frontend | Mobile |
|---|---|---|---|
| Manage food catalog (`Item`) | `ItemController` + `Item` features | `/inventory/items` | Partial |
| Track per-user stock | `UserItem` via `usp_Item_AdjustQuantity` | Quantity adjustment in items grid | `POST /api/Item/items/{id}/quantity` |
| Flavor/nutrient attributes | `ItemController` sub-routes | `/inventory/food-flavors`, `/inventory/food-nutrients` | — |
| Reference data (categories, brands, nutrient types, flavor profiles) | `ItemController` | Reference CRUD pages | — |

### 9.2 Wine

| Feature | Backend | Frontend |
|---|---|---|
| Bottle (catalog + cellar) | `BottleController` | `/wine/bottles` |
| Countries / regions / types / vintages | `CountryController`, `RegionController`, `WineTypeController`, `VintageController` | `/wine/countries`, `/wine/regions`, `/wine/types`, `/wine/vintages` |
| Favorites | `usp_Bottle_SetFavorite` | Favorite toggles |

### 9.3 Recipes

| Feature | Backend | Frontend |
|---|---|---|
| Recipe CRUD | `RecipeController` | `/recipes` |
| Recipe ingredients | `RecipeItems` commands | Recipe detail page |
| Recipe steps | `RecipeSteps` commands | Recipe detail page |

### 9.4 Meal Planning

| Feature | Backend | Frontend |
|---|---|---|
| Weekly meal plans | `MealPlanController` | `/meal-plans` |
| Daily meal slots | `MealSlot` commands | Plan detail page |
| Nutrition summary | `usp_MealPlan_GetNutrition` | Plan nutrition view |

### 9.5 Grocery Lists

| Feature | Backend | Frontend | Mobile |
|---|---|---|---|
| Generate from meal plan | `GenerateGroceryListCommand` | `/grocery-lists` | — |
| Add / delete items | `GroceryListItem` commands | List detail | Grocery list screen |
| Check off items | `ToggleGroceryListItemCheckedCommand` | Item row | Check-off |

### 9.6 Identity / Auth

| Feature | Component |
|---|---|
| Google sign-in | `AuthProvider` (web), `AuthService` (mobile) |
| Token validation | `AddJwtBearer` in `Program.cs` |
| Current user | `AuthController.Me` → `GetCurrentUserQuery` |
| User upsert | `UserResolutionMiddleware` → `UpsertUserCommand` |

---

## 10. Deployment & Operations

### 10.1 Local Development

```bash
dotnet run --project LENA.API          # API on http://localhost:5059
cd frontend && npm run dev             # UI on http://localhost:3000
```

### 10.2 Docker Compose

- `db` — SQL Server 2022
- `db-init` — one-shot database provisioning (`init.sh`)
- `api` — ASP.NET Core (port 8080 internally)
- `ui` — Next.js (port 3000 internally)
- `proxy` — Caddy on 80/443
- `seq` — log aggregation

### 10.3 Database Provisioning

`LENA.Database/init.sh` applies SQL fragments idempotently:

1. Schemas
2. Tables (with retry for FK dependencies)
3. Indexes
4. Seed data
5. Migrations
6. Stored procedures (`CREATE OR ALTER`)
7. Security: `lena_app` login with `GRANT EXECUTE ON SCHEMA` only

### 10.4 CI

`.github/workflows/ci.yml` runs `dotnet` build/test, `npm` lint/typecheck/test/build, and Docker builds.

---

## 11. Security & Hardening

| Concern | Implementation |
|---|---|
| Authentication | Google ID tokens as JWT bearer; `NameClaimType = "email"` |
| Authorization | Fallback policy requires authenticated user on every endpoint unless marked `[AllowAnonymous]` |
| CORS | Explicit `Cors:AllowedOrigins` list; fail-to-start if empty |
| Data isolation | `UserID` passed into stored procedures; per-user scoping enforced in SQL |
| Database access | Least-privilege `lena_app` login with `EXECUTE` only on domain schemas |
| Secrets | No hard-coded credentials; `.env` for `GOOGLE_CLIENT_ID`, `MSSQL_SA_PASSWORD`, `LENA_DB_PASSWORD` |
| Logging | Serilog → Console + File + Seq; request logging; no PII in production by default |

---

## 12. Ongoing Architecture Remediation

The project contains an `architecture-remediation-plan.md` with a five-phase roadmap: secrets hardening, correctness/data-isolation fixes, Clean Architecture boundary enforcement, scaling/best practices, and continuous quality enforcement (analyzers, `dotnet format`, SonarCloud). Several phases are already reflected in the current code (e.g., `LENA.Infrastructure` project, request/response DTOs, user resolution caching, `TimeProvider` usage, and fallback authorization).
