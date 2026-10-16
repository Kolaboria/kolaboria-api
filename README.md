# Kolaboria API

Backend platform for Kolaboria, built with Go, Gin, GORM, and PostgreSQL.

## Requirements

- Go 1.26+
- PostgreSQL for the API and database migrations
- `task` and `golang-migrate` for database commands
- `swag` for regenerating Swagger files

## Run locally

Copy `.env.example` to `.env`, configure the database and application values, then start the API:

```powershell
Copy-Item .env.example .env
go run ./cmd/api
```

The API listens on `APP_PORT`. The local `.env.example` sets `8081`, so the API base URL is `http://localhost:8081/v1`. Without an override, the server and container default to port `8080`.

All versioned routes share the `/v1` router group. The deployed development base URL is `https://api.dev.kolaboria.com/v1/`. Configure the frontend's `NUXT_PUBLIC_API_BASE_URL` with the full base URL, including `/v1`; endpoint paths remain relative to it. The health endpoint is `GET /health`; Swagger UI is `/swagger/index.html`.

## Master data

Public, read-only catalog endpoints:

```text
GET /v1/master-data/contribution-roles
GET /v1/master-data/skills
GET /v1/master-data/tools
```

Each list accepts `page` (default `1`), `limit` (default `50`, maximum `100`), and `q` (literal case-insensitive substring, maximum 100 Unicode characters). The response uses `data` and `meta` with `page`, `limit`, `total`, and `total_pages`.

Two versioned SQL migrations create the catalog tables and insert the 30 contribution roles, 50 skills, and 80 tools. UUIDs are stable between environments. The `000001` files are a baseline for a fresh database. Check existing migration history and tables before applying this baseline to an existing database.

## Database commands

Configure an explicit database target in `.env` or pass another file through `DB_ENV_FILE`:

```powershell
task migrate:up
task migrate:version
task seed
task migrate:down
```

`task seed` aliases `task migrate:up`. Choose one of them; both apply every pending migration, including schema and seed. `task migrate:down` rolls back one migration. The command wrapper never offers `force`; it does not run automatically during API startup. For a different environment file, set `DB_ENV_FILE` in the process environment before running the task.

Migration `000009_create_project_editor_tables` adds the active project editor schema, `000010_create_project_applications` adds persisted applications, and `000011_application_review_and_claimed_owner` adds private review/withdraw metadata and organization-claim owner compatibility. Project role skills and tools use master-data join tables; project applicants are accessible only to the applicant and persisted project owner. See [Project editor API](docs/project-editor.md) and [Applicants API requirements](docs/applicants-api-requirements.md) for access rules and behavior. The guarded project-editor verifier now recognizes migration 000011 and is restricted to its documented disposable loopback database; it has not been run for this change.

## Swagger and Apidog

Run `task swagger` after changing API annotations. Import `docs/swagger.json` or `docs/swagger.yaml` into Apidog. Swagger paths already include `/v1`, so its server base path stays `/`. For API tools that prepend a server URL to these paths, use the scheme, host, and port only, such as `http://localhost:8081`.
