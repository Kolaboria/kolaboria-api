# Database migrations

The paired SQL files are the schema and master-data authority. The `000001` baseline is intended for a fresh database and must not be applied to a database with an unknown migration history without reconciliation.

Configure `DB_HOST`, `DB_PORT`, `DB_NAME`, and `DB_USER` explicitly in `.env` (or set `DB_ENV_FILE` to another environment file). The Go wrapper rejects a missing configuration file and passes the password to the migration process through `PGPASSWORD`; the connection URI does not contain the password.

```powershell
task migrate:up
task migrate:version
task seed
task migrate:down
```

`seed` is an alias for `migrate:up`; both apply pending schema and seed migrations. `migrate:down` reverts one migration. There is no force or database reset command in this workflow. Do not run migration commands against an existing database until its target and migration history have been checked.

The test fixture `test/testdata/master_data.json` is generated from the approved manifest. UUID v5 uses namespace `6f6f8eec-493b-5f70-baa8-1b07a7a2da44` and key `<table>:<slug>`.

## Account and organization context (000008)

Apply `000008_user_organization_context` before deploying the updated auth service. It adds `users.system_role` (`user` or `admin`, default `user`), `organizations`, and `organization_members`. Existing users retain ordinary access. The Kolaboria organization is seeded, but no account is promoted and no membership is granted.

`GET /v1/auth/me` includes `id`, `username`, `system_role`, and `initiable_organizations` (always an array). Only an active, non-deleted admin with an active membership whose `can_initiate_projects` is true can see an active organization in this list. Admin without such a relationship receives `[]`. This consumer list exposes no admin management capabilities.

Provisioning or revoking roles and memberships is an explicit administrative operation; no public endpoint in this milestone can perform it. Organization choices are queried from current persisted state rather than embedded in tokens. Login, refresh, and Google login use the persisted system role when issuing access tokens.

The consumer create editor reads this context, while project save/publish still use local simulation. A future project mutation endpoint must recheck organization authorization from the database; a selected organization ID or the `/auth/me` list is never sufficient authorization by itself. Admin UI and project claim remain future milestones.

The integration test `TestInitiableOrganizationsAgainstPostgres` requires an explicitly configured disposable `KOLABORIA_TEST_DATABASE_URL` with migrations through 000008 applied. Fixtures and revocations are rolled back. Without that configuration the test skips; unit tests do not prove migration replay or live PostgreSQL behavior.
