# Project editor API

The editor API stores drafts, validates the first publish, and returns the persisted definition for edit. All project responses use the shared `status`, `message`, `data` envelope; list responses add numeric `meta` fields. JSON keys use `snake_case`.

## Routes

| Method | Route | Access | Result |
| --- | --- | --- | --- |
| `POST` | `/v1/projects` | Authenticated, active, onboarded actor | Creates a draft (`201`); an identical idempotent retry returns the same project (`200`). |
| `PUT` | `/v1/projects/:id` | Project definition editor | Replaces the complete definition; requires the current `version`. |
| `PATCH` | `/v1/projects/:id/status` | Current project owner | Changes `open` to `draft` or `archived`; requires the current `version`. |
| `DELETE` | `/v1/projects/:id` | Draft owner or authorized initiator of an unclaimed organization draft | Permanently deletes a draft without collaboration history; requires the current `version`. |
| `POST` | `/v1/projects/:id/publish` | Verified, onboarded project editor | Publishes a complete draft. Personal projects become `open`; organization projects become `awaiting_owner`. |
| `POST` | `/v1/projects/:id/apply` | Authenticated, verified, onboarded non-owner | Applies to an open public role with remaining capacity (`201`). |
| `GET` | `/v1/projects/:id/applicants/summary` | Authenticated project owner | Returns private status totals, role capacity, and review availability. |
| `GET` | `/v1/projects/:id/applicants` | Authenticated project owner | Returns a private, filterable page of applications and applicant profile data. |
| `GET` | `/v1/applications/:id` | Applicant or that project's owner | Returns private application details, portfolio links, and review metadata. |
| `PATCH` | `/v1/applications/:id/review` | Verified project owner | Accepts or rejects one pending application and stores an optional private review note. |
| `GET` | `/v1/applications/my-applications` | Authenticated eligible applicant | Returns the actor's paginated application history and private review notes. |
| `GET` | `/v1/applications/my-applications/summary` | Authenticated eligible applicant | Returns global application totals for the actor. |
| `POST` | `/v1/applications/:id/withdraw` | Applicant who owns a pending application | Withdraws a pending application. |
| `GET` | `/v1/projects/:id` | Optional authentication | Returns a visible project by UUID. |
| `GET` | `/v1/projects/slug/:slug` | Optional authentication | Returns a visible project by canonical slug. |
| `GET` | `/v1/projects` | Public | Lists public published projects. |
| `GET` | `/v1/projects/my-projects` | Authenticated | Lists owned and organization-initiated projects in one response. |

`GET` list routes accept `page` (default `1`), `limit` (default `20`, maximum `100`), and `search`. The my-projects route accepts `scope=all|owned|initiated` (default `all`); its summaries include `my_scope` and card role summaries. `pending_application_count` is included only when the current user owns that project. Empty lists return `data: []`, `total: 0`, and `total_pages: 0`.

## Write behavior

Lifecycle mutations use the same transaction and project row lock as applications and editing. `PATCH /:id/status` receives `{ "version": n, "status": "draft" | "archived" }`; only an `open` project can transition. Returning to draft is rejected if any application (including rejected/withdrawn history) or any non-owner membership has ever been recorded. It clears `published_at` and increments `version`. Archiving hides the project from public discovery and stops new applications/acceptances while retaining all applications, contributors, and publication metadata. Pending applications remain available for rejection or applicant withdrawal.

`DELETE /:id` receives `{ "version": n }`. Non-draft projects and drafts with application/contributor history are rejected; successful deletion removes the draft, owner membership, roles, and role catalog associations atomically. Stale versions and invalid transitions return `409`; unauthorized project access returns `404`. These rules are enforced in the API, independently of the action menus. My-project summaries include `version` for card mutations.

Once an organization project has an assigned owner, that owner controls definition editing and lifecycle changes. An unclaimed draft remains controlled by its authorized creator/organization initiator. Returning a claimed organization project to draft retains its owner; publishing it again returns to `open` rather than `awaiting_owner`.

Create and update receive `{ "definition": { ... } }`; create also requires a UUID `client_request_id`, while update requires a positive `version`. Publish receives only `{ "version": n }`. Create always stores `visibility: "public"`; update must round-trip the server's existing visibility. Status, owner identity, filled capacity, timestamps, UI keys, custom role titles, root-level tool IDs, and text skill tags are not writable fields. Unknown or missing definition keys are rejected. A PUT is a full replacement: `null`, `false`, empty strings, and empty arrays are persisted as supplied.

The definition includes creation mode and organization, title, summary, description, slug, category, visibility, optional owner contribution role, origin and collaboration context, acknowledgement, dates, availability, hours per week, agreement, and `roles`. Each role contains a nullable existing `id`, nullable master `contribution_role_id`, description, capacity, `skill_ids`, and `tool_ids`. Skills and tools are attached to that role. Drafts may be incomplete, but master UUIDs, limits, enum values, date order, and organization permissions are checked on every write.

Role UUIDs survive edits. Existing role IDs must belong to the project. Omitted roles are archived; a role with active contributors cannot be removed, have its master identity changed, or be reduced below active membership. `skill_ids: []` and `tool_ids: []` clear only those role relations. An empty `owner_contribution_role_id` clears the optional professional role while preserving project ownership and the active owner membership.

Concurrent writes use `version`; stale PUT and publish requests return `409` without mutation. Draft creation is idempotent per actor and `client_request_id`: an identical body returns the original project, while a different body with the same key conflicts. The backend derives all permissions from persisted user, organization, and membership state.

Draft and invite-only reads are hidden from outsiders. Discovery includes public published projects only. Project detail responses expose the persisted owner and organization context, role capacity, and `viewer_application` (`null` for guests or viewers without an active application; otherwise the viewer's pending or accepted application). The client derives owner, organization-initiator, and apply states from those fields and the signed-in account's `initiable_organizations`; the response no longer includes a generic `capabilities` object. The API remains authoritative when an application is submitted: the project must be `open` and `public`, have an owner, the selected role must have remaining contributor capacity, the applicant cannot be the owner or an authorized initiator for that organization, and the applicant cannot already have a pending or accepted application. Active project members are also excluded. A prior rejection starts a 72-hour per-project cooldown across roles; withdrawal does not start a new cooldown.

Applicant management is authorized exclusively by the persisted `projects.owner_id`. For an organization-initiated project, the user whose successful claim is recorded in that column can manage applications, even when another user created or initiated the project. Organization administrators, creators, initiators, and contributors do not gain access through those roles alone. Public project visibility does not expose applicant names, motivation, portfolio links, or reviewer notes. Application reads and mutations are private and scoped to the applicant or the current project owner. Role, skill, and tool labels come from the existing `project_roles`, `contribution_roles`, `skills`, and `tools` relations.

## Migration and verification

Migration `000009_create_project_editor_tables` adds the project, role, role-skill, role-tool, and membership tables. Migration `000010_create_project_applications` adds role applications and prevents duplicate pending or accepted applications by the same user for a project. Migration `000011_application_review_and_claimed_owner` stores review/withdraw metadata, adds guarded decision-shape checks and a rejected-history index, and permits a claimed user owner on an organization project. The down migration refuses to remove that compatibility while claimed organization owners still exist. These migrations reference active project and master relations and do not alter or replay deprecated API schema. See [Applicants API requirements](applicants-api-requirements.md) for filters, response fields, privacy, actions, and validation rules.

For a database explicitly created for this test, run:

```powershell
.\scripts\verify-project-editor.ps1 -TestEnvFile .\tmp\project-editor-test.env
```

The verifier accepts only a loopback database named `project_editor_test_*` using the `project_editor_test` user. It applies pending migrations through `000011`, checks the project-editor aggregate persistence flow, rolls back `000011` then `000010` and `000009`, and reapplies all three. The `000011` rollback aborts safely if a claimed organization owner remains in the disposable database. This verifier has not been run for the applicants implementation. Do not point it at an application database.
