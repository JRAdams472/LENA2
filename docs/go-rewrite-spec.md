# LENA — Go + PostgreSQL Ground-Up Rewrite Specification

## 1. Purpose

This document is the master specification for a complete ground-up rewrite of the LENA kitchen-management system in **Go** with a **PostgreSQL** backend. It keeps the existing functional scope (inventory, wine, recipes, meal plans, grocery lists) but re-architects the implementation as a single **modular Go monolith** with an internal **GraphQL BFF**.

## 2. Decisions

| Topic | Decision |
|---|---|
| Physical backend | **Modular monolith that can split later** — one Go binary with internal domain packages and explicit boundaries. |
| BFF | **Internal package (`internal/bff`)** inside the same Go binary, exposing a single `/graphql` endpoint. |
| Frontends | **Flutter mobile** (primary) **plus the existing Next.js web app upgraded to consume GraphQL**. |
| Cross-domain rule | **No cross-domain SQL queries.** Foreign keys are stored as IDs; the BFF hydrates them with module calls. |
| Authentication | **Google now, multi-OIDC ready.** Schema stores `(provider, external_subject)` and the JWT layer accepts a configurable issuer/audience list. |
| Data migration | **Fresh start** — no SQL Server data migration; seed reference data if desired. |
| Go / Postgres stack | `gqlgen`, Echo, `pgx` + `sqlc`, `golang-migrate`, `go-playground/validator`, `rs/zerolog`/`zap`. |

## 3. High-Level Architecture

```
Flutter │ Next.js
   └───────┬───────┘
           │
     Caddy /proxy
           │
     Go monolith (cmd/lena)
   ┌───────┼───────┐
   │       │       │
 BFF   identity  inventory  wine  recipe  mealplan  grocery  userprefs
   │
   └───────┼───────┘
           │
       PostgreSQL 16
```

- **One binary** for easy self-hosting.
- **Domain modules** each own a PostgreSQL schema and a service interface.
- **BFF** is the only package allowed to call multiple domain modules in one request.
- **GraphQL is the single public API**.

## 4. Technology Stack

| Layer | Choice |
|---|---|
| Language | Go 1.23+ |
| HTTP router | Echo v4 (or `net/http` + `chi`) |
| GraphQL | `gqlgen` |
| DB driver | `pgx/v5` |
| DB code gen | `sqlc` |
| Migrations | `golang-migrate` |
| Validation | `go-playground/validator/v10` |
| JWT | `golang-jwt/jwt/v5` or `lestrrat-go/jwx/v2` |
| Logging | `rs/zerolog` or `uber-go/zap` |
| Config | `envconfig` + `.env` |
| Flutter client | `graphql_flutter` |
| Web client | Apollo Client or `urql` |

## 5. Project Layout

```
cmd/lena/                 # main entrypoint
internal/
  bff/                    # GraphQL resolvers + orchestration
  identity/               # users, auth
  inventory/              # item catalog
  wine/                   # wine catalog
  recipe/                 # recipe catalog
  mealplan/               # meal plans
  grocery/                # grocery lists / generation
  userprefs/              # per-user stock / favorites
  platform/               # db, config, logger, validation, pagination
migrations/               # golang-migrate .sql files
docs/                     # specification & planning docs
docker-compose.yml
.env.example
```

## 6. Domain Boundaries

- `identity` — user records, provider/subject mapping.
- `inventory` — global item, brand, category, flavor, nutrient reference data.
- `wine` — global wine definitions, countries, regions, types.
- `recipe` — global recipes, ingredients, steps.
- `userprefs` — `user_item`, `user_bottle`, `user_recipe_preference`.
- `mealplan` — per-user meal plans and slots.
- `grocery` — grocery lists, generation.
- `bff` — GraphQL resolvers that call domain services.

## 7. Implementation Phases

1. **Bootstrap** — `go.mod`, Docker, Postgres, `golang-migrate`, `sqlc`.
2. **Identity + Auth** — users table, JWT middleware, current user context.
3. **Catalog modules** — inventory, wine, recipe CRUD.
4. **User preferences** — `user_item`, `user_bottle`, `user_recipe_preference`.
5. **Meal planning + grocery** — plans, slots, grocery generation.
6. **BFF + GraphQL** — schema, resolvers, orchestration.
7. **Flutter + Next.js** — clients point to GraphQL.
8. **Hardening** — tests, lint, logging, CORS, observability.

## 8. Out of Scope

- Migrating data from the existing SQL Server database.
- Admin roles / RBAC (all authenticated users may mutate global catalog for now).
- Federation / separate microservices.
- Real-time / WebSocket subscriptions.

## 9. Acceptance Criteria

- [ ] Data model separates catalog tables from per-user tables.
- [ ] GraphQL BFF exposes every feature needed by Flutter and Next.js.
- [ ] No SQL joins cross-domain; BFF assembles data by calling modules.
- [ ] Auth supports Google, with multi-provider hooks preserved in schema and code.
- [ ] All `docs/` specification files are complete and consistent.