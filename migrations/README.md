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
