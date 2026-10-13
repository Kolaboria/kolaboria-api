# Project editor API

The editor API stores drafts, validates the first publish, and returns the persisted definition for edit. All project responses use the shared `status`, `message`, `data` envelope; list responses add numeric `meta` fields. JSON keys use `snake_case`.

## Routes

| Method | Route | Access | Result |
| --- | --- | --- | --- |
| `POST` | `/api/v1/projects` | Authenticated, active, onboarded actor | Creates a draft (`201`); an identical idempotent retry returns the same project (`200`). |
| `PUT` | `/api/v1/projects/:id` | Project definition editor | Replaces the complete definition; requires the current `version`. |
| `POST` | `/api/v1/projects/:id/publish` | Verified, onboarded project editor | Publishes a complete draft. Personal projects become `open`; organization projects become `awaiting_owner`. |
| `POST` | `/api/v1/projects/:id/apply` | Authenticated, verified, onboarded non-owner | Applies to an open public role with remaining capacity (`201`). |
| `GET` | `/api/v1/projects/:id` | Optional authentication | Returns a visible project by UUID. |
| `GET` | `/api/v1/projects/slug/:slug` | Optional authentication | Returns a visible project by canonical slug. |
| `GET` | `/api/v1/projects` | Public | Lists public published projects. |
| `GET` | `/api/v1/projects/my-projects` | Authenticated | Lists owned and organization-initiated projects in one response. |

`GET` list routes accept `page` (default `1`), `limit` (default `20`, maximum `100`), and `q`. The my-projects route accepts `scope=all|owned|initiated` (default `all`); its summaries include `my_scope` and card role summaries. `pending_application_count` is included only when the current user owns that project. Empty lists return `data: []`, `total: 0`, and `total_pages: 0`.

## Write behavior

Create and update receive `{ "definition": { ... } }`; create also requires a UUID `client_request_id`, while update requires a positive `version`. Publish receives only `{ "version": n }`. Create always stores `visibility: "public"`; update must round-trip the server's existing visibility. Status, owner identity, filled capacity, timestamps, UI keys, custom role titles, root-level tool IDs, and text skill tags are not writable fields. Unknown or missing definition keys are rejected. A PUT is a full replacement: `null`, `false`, empty strings, and empty arrays are persisted as supplied.

The definition includes creation mode and organization, title, summary, description, slug, category, visibility, optional owner contribution role, origin and collaboration context, acknowledgement, dates, availability, hours per week, agreement, and `roles`. Each role contains a nullable existing `id`, nullable master `contribution_role_id`, description, capacity, `skill_ids`, and `tool_ids`. Skills and tools are attached to that role. Drafts may be incomplete, but master UUIDs, limits, enum values, date order, and organization permissions are checked on every write.

Role UUIDs survive edits. Existing role IDs must belong to the project. Omitted roles are archived; a role with active contributors cannot be removed, have its master identity changed, or be reduced below active membership. `skill_ids: []` and `tool_ids: []` clear only those role relations. An empty `owner_contribution_role_id` clears the optional professional role while preserving project ownership and the active owner membership.

Concurrent writes use `version`; stale PUT and publish requests return `409` without mutation. Draft creation is idempotent per actor and `client_request_id`: an identical body returns the original project, while a different body with the same key conflicts. The backend derives all permissions from persisted user, organization, and membership state.

Draft and invite-only reads are hidden from outsiders. Discovery includes public published projects only. Project detail responses expose the persisted owner and organization context, role capacity, and `viewer_application` (`null` for guests or viewers without an active application; otherwise the viewer's pending or accepted application). The client derives owner, organization-initiator, and apply states from those fields and the signed-in account's `initiable_organizations`; the response no longer includes a generic `capabilities` object. The API remains authoritative when an application is submitted: the project must be `open` and `public`, have an owner, the selected role must have remaining contributor capacity, the applicant cannot be the owner or an authorized initiator for that organization, and the applicant cannot already have a pending or accepted application. Project membership checks are deferred. Claim, workspace, and status-transition actions remain outside this API scope.

## Migration and verification

Migration `000009_create_project_editor_tables` adds the project, role, role-skill, role-tool, and membership tables. Migration `000010_create_project_applications` adds role applications and prevents duplicate pending or accepted applications by the same user for a project. These migrations reference the active master catalog and organization context migrations and do not alter or replay deprecated API schema.

For a database explicitly created for this test, run:

```powershell
.\scripts\verify-project-editor.ps1 -TestEnvFile .\tmp\project-editor-test.env
```

The verifier accepts only a loopback database named `project_editor_test_*` using the `project_editor_test` user. It applies pending migrations, tests aggregate persistence and rollback, rolls back only `000010` and `000009`, then reapplies both. Do not point it at an application database.
