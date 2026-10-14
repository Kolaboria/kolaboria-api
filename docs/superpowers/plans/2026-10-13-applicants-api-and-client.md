# Applicants API and Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menyediakan pengelolaan pelamar persisted oleh owner personal maupun owner hasil claim organisasi, serta alur lihat keputusan/withdraw/reapply yang memakai kontrak dan referensi master terbaru.

**Architecture:** Tambahkan ApplicationService/Repository/Handler yang fokus dalam package project existing; pertahankan ProjectService untuk editor/read/apply. Read memakai scoped query, SQL aggregates, dan batch hydration; commands memakai transaksi dengan urutan actor → project → role → application. Frontend memakai DTO canonical baru dan composable khusus server pagination, bukan model/fixture API legacy.

**Tech Stack:** Go 1.26.4 sesuai go.mod, Gin, GORM, PostgreSQL, golang-migrate dan Swagger existing; Nuxt 4/Vue 3/TypeScript, Node built-in test dan Vue runtime helper existing. Tidak menambah dependency product/testing baru.

**Spec:** [applicants-api-requirements.md r2](../../applicants-api-requirements.md), khususnya bagian 3.4, 4.1, 6–10, 13, dan 15. Executor membaca spec dan plan ini bersama.

**Status:** Implementasi tersedia pada branch codex/applicants-management-api dan codex/applicants-management-client. Catatan hasil, batas verifikasi, dan item yang belum dibuktikan ada di ledger .superpowers/sdd/2026-10-13-applicants-api-and-client/progress.md. Perubahan terbaru pengguna pada my-applications.vue dipertahankan.

## Global Constraints

- Keputusan terbaru pengguna → requirement r2 dan plan ini → kontrak aktif terbaru → kode aktif sebagai starting point. Jika berbeda, ubah kode sesuai requirement terbaru.
- `kolaboria-api-go-deprecated` hanya referensi historis, bukan standar baku; tidak mengubah repo deprecated atau menjalankan migrasinya.
- Owner personal dan claimant organisasi mempunyai hak manage yang sama berdasarkan `projects.owner_id`; creator/admin/initiator/contributor tidak otomatis memperoleh hak.
- Data role/skills/tools berasal dari `contribution_roles`, `skills`, `tools` melalui relasi existing. Tidak menambah master alternatif, label persistence duplikat, atau dummy production.
- Response HTTP memakai helper existing dan snake_case; error `{ status, message, data: { trace_id } }`. Tidak menambah machine error code atau generic capabilities project tanpa perubahan kontrak terpisah.
- Owner list: limit default 5; My Applications: limit default 6; maximum limit 100; q maximum 200 Unicode chars; note maximum 2.000 Unicode chars.
- Rejection cooldown tepat 72 jam per applicant/project lintas role; withdraw tidak memulai cooldown baru. Satu application pending/accepted per actor/project.
- Capacity berasal dari contributor membership active. Accept atomik membuat member dan final decision; project tetap open; acceptance menaikkan projects.version.
- Accept hanya open; reject pending pada open/in_progress/completed/archived; application final tidak dibuka kembali. Retry status+normalized note identik no-op 200, berbeda 409.
- Read actor harus active/onboarded/talent; review/apply email verified; withdraw hanya actor pemilik pending dan tidak mensyaratkan verifikasi tambahan.
- Array JSON selalu array; nullable metadata legacy tetap nullable, tidak menciptakan reviewed_at/reviewer identity palsu.
- Preserve perubahan working tree pengguna. Tidak reset file atau mengganti layout terbaru dengan salinan lama untuk mempermudah wiring.
- Endpoint claim terpisah; compatibility schema dan otorisasi claimant wajib di scope ini. Jangan klaim journey claim UI/API sudah selesai sampai dependency claim nyata tersedia.
- Verifikasi DB hanya pada loopback disposable `project_editor_test_*`, user `project_editor_test`, melalui verifier yang memvalidasi target. Tidak menjalankan migration/rollback pada DB aplikasi.
- Scope tidak menambah notifications/workspace/status commands/invitation/bulk review/undo/ranking/export/skill matching/custom titles.

## Review Focus

1. Creator A organisasi, claimant B owner: B boleh manage tanpa keanggotaan organisasi, A ditolak; dipastikan Task 1, 3, 4, 7, 10, 12.
2. Legacy final rows tanpa metadata asli: masih terbaca, tidak dibackfill palsu, cooldown unknown tidak dibypass; Task 1, 5, 8.
3. User berganti account/project/filter ketika request lama selesai: tidak ada data privat/stale response yang menimpa state baru; Task 9–11.
4. Role/master berubah atau archived, profil hilang, beberapa skills cocok: history dan counts tetap benar tanpa duplikasi; Task 3, 4, 8, 9.
5. Timeout setelah commit atau race accept/withdraw/editor: satu efek persisted, retry aman, UI tidak mengklaim sukses atas request gagal; Task 4, 6, 8, 10, 11.

## 1. Rekonsiliasi rencana lama

| Arahan yang tidak lagi dipakai | Pengganti r2 |
| --- | --- |
| Buat tabel applications di migration 000010 dan Apply baru | Extend baseline existing melalui migration berikutnya; perluas `ProjectService.Apply`. |
| Reintroduce project generic capabilities | Ownership/viewer_application existing; permissions privat application. |
| Guard CreatorID dari API deprecated | OwnerID persisted untuk kedua creation_mode. |
| Copy entity text[]/custom title/role status legacy | JSONB existing; master role/skills/tools join tables; derived capacity/status. |
| Fetch seluruh applications lalu filter/count client | Owner/actor lists server-side; summary global SQL; bounded batch hydration. |
| Testing hanya copy fungsi ke file test | Import dan jalankan modul/setup nyata memakai helper frontend existing; PostgreSQL untuk race/constraints. |
| Dummy fallback empty/error dan swallow withdraw error | Empty/error/loading terpisah; success hanya dari canonical response. |
| Verifier yang selalu latest=10 | Verifier applicants 10→11→10→11, editor baseline tetap kompatibel secara eksplisit. |

Rencana 9 Oktober disupersede untuk scope applicants/applications yang tumpang tindih. Aturan claim di dokumen lama adalah konteks untuk dependency claim, bukan instruksi menambah claim ke paket ini atau mengembalikan kontrak lama.

## 2. Kontrak final untuk baseline rencana

### Routes

Semua route berikut authenticated kecuali project detail existing yang tetap optional-auth.

| Route | Data / akses |
| --- | --- |
| GET `/api/v1/projects/:id/applicants/summary` | Owner; project mini/capacity/stats/roles/permissions. |
| GET `/api/v1/projects/:id/applicants` | Owner; paginated `ApplicantListItemResponse`. |
| GET `/api/v1/applications/:id` | Owner proyek OR applicant application itu; `ApplicationDetailResponse`. |
| PATCH `/api/v1/applications/:id/review` | Owner verified; accepted/rejected + optional reviewer_note; canonical detail 200. |
| GET `/api/v1/applications/my-applications/summary` | Actor eligible; global own stats. |
| GET `/api/v1/applications/my-applications` | Actor eligible; paginated `MyApplicationListItemResponse`. |
| POST `/api/v1/applications/:id/withdraw` | Applicant; pending→withdrawn; canonical detail 200. |
| POST `/api/v1/projects/:id/apply` | Existing; same body/receipt, ditambah membership/cooldown/URL validation. |

Owner filter: page/limit/q/statuses CSV/project_role_ids CSV/sort. Own filter: page/limit/q/statuses CSV/origins CSV/sort. Semua pagination validation dan OR/AND mengikuti spec; malformed CSV/repeated keys/unknown enum/UUID zero → 400. Role UUID tidak ada atau lintas project → 400 sesudah owner gate. newest=`applied_at DESC,id ASC`; oldest=`applied_at ASC,id ASC`.

`ApplicationDetailResponse` memakai fields canonical `applicant_user_id`, `applicant_profile`, `project`, `project_role.skills/tools`; portfolio_links dan review/withdraw metadata lengkap. Permission `can_accept/can_reject` berlaku untuk viewer owner saja. `project` mini mencakup id/title/slug/status/summary/origin/creation_mode/owner_profile/initiator_organization untuk kedua halaman. Own list memuat review note/timestamps/retry_after yang terlihat di kartu terbaru. Tidak ada completion_score/custom_title palsu.

My summary data=`{ stats: { total, pending, accepted, rejected, withdrawn } }`. Owner stats dan own stats global tidak mengikuti filter/page. meta.total selalu hasil filter; empty list data=[]/total_pages=0. Frontend origin labels dipetakan dari enum keys, author mengikuti owner_profile lalu organisasi fallback.

### Batas response/version/clock

ReviewedBy/ReviewedAt dan WithdrawnAt dari server. Legacy final metadata null tetap terbaca; latest rejected yang reviewed_at null memakai updated_at sebagai estimasi waktu keputusan (fallback applied_at), mengembalikan retry_after_estimated=true dan tidak memblokir tanpa batas. Viewer memilih active application, jika tidak latest history; retry_after dari latest rejection independen, bahkan jika viewer latest withdrawn. Apply tetap mensyaratkan cooldown 72 jam terlewati dan proyek masih open. API errors tidak diubah menjadi field capabilities atau teks raw untuk UI.

Clock application: injected `func() time.Time` pada ApplicationService. ProjectService menerima variadic option `WithProjectClock` dengan default time.Now agar semua caller lama tetap valid; Apply menggunakan clock itu untuk boundary test. Gunakan UTC untuk persistence/output.

## 3. Pemetaan file dan batas komponen

Path API relatif `kolaboria-api/`, path client relatif `kolaboria-app/apps/client/`.

| Area | File baru / diubah | Tanggung jawab |
| --- | --- | --- |
| Schema | `migrations/000011_application_review_and_claimed_owner.{up,down}.sql` | Metadata review/withdraw, conditional checks/index, org owner constraint compatibility. |
| Existing project | `internal/modules/project/entity.go`, `repository.go`, `service.go`, `validation.go`, `read_repository.go`, `read_service.go`, `dto.go` | Persistence fields; pending usage; role change protection; Apply guard/cooldown; owner read/viewer fields. |
| Application contracts | `internal/modules/project/application_domain.go`, `application_dto.go`, `application_validation.go`, `application_read_model.go`, `application_mapper.go` | Typed statuses/filter/policies/errors, DTO, read projections, pure mapping. |
| Application reads | `application_repository.go`, `application_read_repository.go`, `application_read_service.go` | Scoped owner/actor read, consistent read snapshots/counts, batch hydration. |
| Application commands | `application_service.go`, `application_command_repository.go` | Review/withdraw orchestration dan narrow transaction adapter. |
| HTTP | `application_handler.go`, `application_routes.go`, `internal/http/router.go`, `internal/bootstrap/application.go` | HTTP/query parser, registration, constructor wiring. |
| API tests | `application_*_test.go` pada package project; `test/application_routes_test.go`, `test/application_workflow_integration_test.go`, existing project/editor tests | Policy/DTO/routes, scoped relational reads, migration/race/rollback. |
| DB verification | `scripts/verify-applicants.ps1`, existing `scripts/verify-project-editor.ps1` | Guarded up/down/replay dan regression baseline. |
| Client contracts | `app/types/application.ts`, existing `types/project-editor.ts`, `types/project.ts`, `constants/api-endpoints.ts` | Canonical typed requests/responses/viewer fields, URLs. |
| Client transport | `services/application.service.ts`, `services/project.service.ts`, `composables/useProjects.ts` | Typed envelope/pagination; remove unused legacy application wrappers after consumer migration. |
| Presentation | `app/utils/application-presentation.ts`, existing `utils/project-presentation.ts`, `constants/error-messages.ts` bila mapping lokal diperlukan | Pure labels/fallbacks, ownership/retry state, safe client copy. |
| Owner page state | `app/composables/useProjectApplicants.ts`, `applicants.vue`, `ApplicantDetailModal.vue` | Fetch generations, summary/list/detail/review, current modal state. |
| Own page state | `app/composables/useMyApplications.ts`, `useApplicationRefresh.ts`, existing `useMyProjectList.ts`, `pages/projects/my-applications.vue`, application counters pada `pages/home.vue` | Server filters/page/detail/withdraw, preserve latest layout, summary counts, actor-scoped refresh signal. |
| Detail integration | `pages/projects/[slug]/index.vue`, existing project/my-project pending count tests | Ownership CTA dan cooldown tanpa generic capabilities. |
| Client tests | `tests/application-contract.test.mjs`, `application-api.test.mjs`, `project-applicants-state.test.mjs`, `project-applicants-page.test.mjs`, `my-applications-state.test.mjs`, `my-applications-page.test.mjs`, `home-application-summary.test.mjs`, updated project copy/count tests | Import real modules/setup; no copied implementation tests. |
| Documentation | Current spec/plan, `docs/project-editor.md`, annotations/generated Swagger, README verifier | Latest source-of-truth links dan runtime status. |

Jika nomor migration 11 sudah terpakai ketika eksekusi dimulai, pilih nomor berikutnya setelah memeriksa migration list dan sesuaikan verifier/plan references. Jangan rewrite migration yang sudah diterapkan.

## 4. Interface bersama antar-task

Semua backend signature berikut berada pada package project. Katalog ini menetapkan nama, parameter, dan return type; body diimplementasikan pada task pemilik.

```go
func NewApplicationRepository(db *gorm.DB) ApplicationRepository
func NewApplicationService(repo ApplicationRepository, now func() time.Time) *ApplicationService
func NewApplicationHandler(service *ApplicationService) *ApplicationHandler
func RegisterApplicationRoutes(v1 gin.IRouter, handler *ApplicationHandler, jwtService jwt.JWTService)

// Project constructor tetap source-compatible.
type ProjectServiceOption func(*ProjectService)
func WithProjectClock(now func() time.Time) ProjectServiceOption
func NewProjectService(repo ProjectRepository, options ...ProjectServiceOption) *ProjectService

// Public application use cases, DTO bukan entity.
func (s *ApplicationService) GetApplicantSummary(ctx context.Context, actorID, projectID uuid.UUID) (*ApplicantSummaryResponse, error)
func (s *ApplicationService) ListProjectApplicants(ctx context.Context, actorID, projectID uuid.UUID, filter ApplicationListFilter) ([]ApplicantListItemResponse, ProjectPaginationMeta, error)
func (s *ApplicationService) GetApplicationDetail(ctx context.Context, actorID, applicationID uuid.UUID) (*ApplicationDetailResponse, error)
func (s *ApplicationService) ListMyApplications(ctx context.Context, actorID uuid.UUID, filter ApplicationListFilter) ([]MyApplicationListItemResponse, ProjectPaginationMeta, error)
func (s *ApplicationService) GetMyApplicationSummary(ctx context.Context, actorID uuid.UUID) (*MyApplicationSummaryResponse, error)
func (s *ApplicationService) ReviewApplication(ctx context.Context, actorID, applicationID uuid.UUID, request ReviewApplicationRequest) (*ApplicationDetailResponse, error)
func (s *ApplicationService) WithdrawApplication(ctx context.Context, actorID, applicationID uuid.UUID) (*ApplicationDetailResponse, error)
```

`ApplicationListFilter`: Page/Limit int, Search string, Statuses []ApplicationStatus, ProjectRoleIDs []uuid.UUID, Origins []string, Sort ApplicationSort. Filter transport berbeda default owner/actor; domain validators `validateOwnerApplicationFilter` dan `validateMyApplicationFilter` menolak fields yang bukan untuk endpointnya. Parser `parseOwnerApplicationFilter`/`parseMyApplicationFilter` menghasilkan filter dengan limit default 5/6 dan membedakan omitted parameter dari invalid explicit zero.

Berikut interface akhir lintas-task. Task3 memperkenalkan read subset owner/identity, Task4 menambah WithApplicationTransaction bersama adapter, dan Task6 menambah GetMySummary/ListApplicantApplications bersama query own history. Jangan menambahkan method interface sebelum implementasinya tersedia lalu mengisi stub sementara hanya agar compile.

```go
type ApplicationRepository interface {
    FindActorForRead(ctx context.Context, id uuid.UUID) (*ProjectActor, error)
    FindOwnerProject(ctx context.Context, actorID, projectID uuid.UUID) (*Project, error)
    CountProjectRoleIDs(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) (int64, error)
    GetOwnerSummary(ctx context.Context, actorID, projectID uuid.UUID) (*ApplicationManagementReadModel, error)
    ListOwnerApplications(ctx context.Context, actorID, projectID uuid.UUID, filter ApplicationListFilter) ([]ApplicationReadModel, int64, error)
    GetVisibleApplication(ctx context.Context, actorID, applicationID uuid.UUID) (*ApplicationReadModel, error)
    GetMySummary(ctx context.Context, actorID uuid.UUID) (*ApplicationStatsReadModel, error)
    ListApplicantApplications(ctx context.Context, actorID uuid.UUID, filter ApplicationListFilter) ([]ApplicationReadModel, int64, error)
    FindApplicationIdentity(ctx context.Context, applicationID uuid.UUID) (*ApplicationIdentity, error)
    WithApplicationTransaction(ctx context.Context, fn func(ApplicationTx) error) error
}
```

ApplicationIdentity berisi ProjectID/ProjectRoleID/ApplicantUserID UUID dan merupakan lookup hint untuk urutan lock, bukan authorization result. ApplicationStatsReadModel berisi Total/Pending/Accepted/Rejected/Withdrawn int64. ApplicationManagementReadModel membawa Project context, stats, role capacity/options; ApplicationReadModel membawa entity/application/profile/project/role/capacity serta state eligibility untuk policy detail. Definisikan read model structs pada Task2, SQL hydration pada Task3/6; DTO tidak digunakan sebagai repository return type.

Service menerima read models/collections bertipe dan mapper mengubahnya ke DTO. Root reads memakai scoped actor/project predicates di query serta snapshot konsisten untuk multi-query response; empty owner list tetap harus membedakan unauthorized resource dari authorized empty project. Recheck owner predicate dalam snapshot ListOwnerApplications/GetOwnerSummary, bukan mengandalkan lookup owner sebelum query sebagai satu-satunya proteksi.

```go
// Narrow command adapter berbagi implementation dengan gormProjectTx
// untuk actor/usage/member helpers; tidak mengekspos *gorm.DB ke service.
type ApplicationTx interface {
    FindActor(ctx context.Context, id uuid.UUID) (*ProjectActor, error)
    FindActorForRead(ctx context.Context, id uuid.UUID) (*ProjectActor, error)
    LockProjectState(ctx context.Context, id uuid.UUID) (*Project, error)
    LockRoleForApplication(ctx context.Context, projectID, roleID uuid.UUID) (*ProjectRole, error)
    LockApplication(ctx context.Context, id uuid.UUID) (*ProjectApplication, error)
    CountRoleUsage(ctx context.Context, roleID uuid.UUID) (RoleUsage, error)
    IsActiveProjectMember(ctx context.Context, projectID, userID uuid.UUID) (bool, error)
    IsAuthorizedOrganizationInitiator(ctx context.Context, userID, organizationID uuid.UUID) (bool, error)
    CreateContributor(ctx context.Context, member *ProjectMember) error
    UpdateApplication(ctx context.Context, application *ProjectApplication, from ApplicationStatus) error
    BumpProjectVersion(ctx context.Context, projectID uuid.UUID, updatedAt time.Time) error
}
```

LockRoleForApplication memuat archived juga; hanya accept menolaknya. UpdateApplication menulis kolom whitelist dan memakai expected status/RowsAffected, bukan Save seluruh model yang menimpa field lain. Eligibility applicant/organization setelah project lock menggunakan read helper tanpa user/member UPDATE lock baru; tidak memanggil FindActor(applicant) yang dapat deadlock dengan Apply.

ProjectTx menambah `FindLatestRejectedApplication(ctx, projectID, applicantID uuid.UUID) (*ProjectApplication,error)` untuk Apply; metadata lookup ini juga dipakai read implementation untuk viewer retry. Pilih latest rejected attempt dengan `applied_at DESC,id ASC`, bukan query reviewed_at yang diam-diam menghilangkan record metadata-null; timestamp cooldown tetap dari reviewed_at record itu. Existing CountRoleUsage diperbaiki menjadi active contributor count + pending application count.

Shared policy helpers pada application_domain/validation: `requireApplicationReader(actor *ProjectActor) error`, `requireApplicationWriter(actor *ProjectActor) error`, `requireApplicationManager(actorID uuid.UUID, project *Project) error`, `normalizeReviewRequest(request ReviewApplicationRequest) (ReviewApplicationRequest,error)`, `reviewPermissions(context ApplicationReviewContext) ApplicationPermissionsResponse`. ApplicationReviewContext berisi viewer/project/application/role/applicant/usage/member/initiator state yang sudah dibaca; mapper/policy tidak melakukan query.

Read projection ApplicationReadModel membawa application entity + applicant profile + project context + role master/skills/tools/capacity; summary read model membawa global stats/capacity/role options. Repository tidak mengembalikan HTTP DTO. Methods tidak memakai API legacy entities atau alias profiles/projects.

## 5. Urutan dan dependency

```text
T1 schema → T2 domain/DTO → T3 owner reads → T4 review
                     └→ T5 apply/viewer → T6 own reads/withdraw
T3–T6 → T7 HTTP/wiring → T8 PostgreSQL verifier
T2/T7 contracts → T9 client types/services → T10 owner UI + T11 own UI/home
T5/T10/T11 → T12 detail integration → T13 full verification/docs
```

Tidak perlu menjalankan subagent/parallel execution untuk menyusun rencana ini. Pemilihan metode eksekusi dilakukan ketika implementasi diminta; interfaces di atas tetap sama.

### Task 1: Metadata review dan compatibility owner organisasi

**Files:** Create migration 000011 up/down dan `test/application_workflow_integration_test.go`; modify `entity.go`. **Consumes:** schema 000001–000010. **Produces:** review/withdraw fields nullable dan organization project dengan owner dapat disimpan.

- [ ] **1. Tulis failing tests** `TestApplicationMigrationMetadataAndClaimedOwner`: insert project organisasi created by A, owner B dan active owner member B; application final dengan metadata; assert success. `TestApplicationMigrationRejectsInvalidNewDecision` menolak final baru tanpa reviewer/time, pending dengan metadata, note >2000. `TestApplicationLegacyFinalHistoryRemainsReadable` menyimpan legacy final sebelum upgrade dan memastikan tidak ada synthetic backfill.
- [ ] **2. Jalankan test** `go test ./test -run '^TestApplicationMigration' -count=1 -v` pada target disposable melalui verifier candidate; missing schema harus FAIL, bukan skip yang dihitung pass.
- [ ] **3. Implementasikan migration up dalam transaksi SQL eksplisit** tambah reviewer_note/reviewed_by FK/reviewed_at/withdrawn_at; new-row shape/note checks NOT VALID untuk legacy; indeks latest rejected `(project_id,applicant_user_id,applied_at DESC,id)` partial status rejected. Ubah cabang org creation identity agar owner boleh null atau UUID. Pertahankan personal branch, org identity/FKs dan member unique indexes; tidak mengubah 000009/000010.
- [ ] **4. Implementasikan down dalam transaksi SQL eksplisit** pemeriksaan adanya org owner sebelum semua DROP/ALTER; jika ada raise exception dan jangan menghapus data. Jika aman, restore exact original identity constraint lalu drop hanya kolom/constraint/index baru. Verifier harus membuktikan failure preflight tidak meninggalkan partial schema change.
- [ ] **5. Tambah entity nullable fields dan verifikasi** jalankan tests Task 1 serta replay up/down/up. Assert before/after row counts dan legacy metadata sama. Test down ketika claimed owner ada harus FAIL aman tanpa partial schema loss, kemudian fixture dilepas secara terkontrol dan down dapat PASS.
- [ ] **6. Checkpoint** `git diff --check`; diff hanya migration baru/entity/tests task ini. Catat evidence DB target/version dan usulan commit `feat: persist application reviews and support claimed project owners`.

### Task 2: Typed contracts, query validation, dan owner policy

**Files:** Create application_domain.go, application_dto.go, application_validation.go, application_read_model.go, application_mapper.go dan tests pasangan. **Consumes:** Task 1 entities/current master DTO. **Produces:** types/filter/policy/signatures pada bagian 4.

- [ ] **1. Tulis failing tests** `TestApplicationOwnerPolicyUsesClaimedOwner` dengan project organisasi creator A/owner B: owner guard B nil, A ErrNotFound; unknown/inactive/talent absent reader ErrForbidden, unverified reader nil/writer ErrForbidden. `TestApplicationFilterValidation` untuk defaults 5/6, max100, q200/201 runes, sort/status/origin invalid, role nil UUID, duplicates, owner-filter origins disallowed, own-filter project_role_ids disallowed. CSV/repeated-key transport tests dimiliki Task7, tidak membuat Task2 bergantung handler yang belum ada.
- [ ] **2. Jalankan** `go test ./internal/modules/project -run '^TestApplication(OwnerPolicy|FilterValidation|DTO|Mapping|ReviewNormalization)' -count=1` → FAIL karena types/functions baru belum ada.
- [ ] **3. Implementasikan types dan policies** DTO mengikuti spec/r2; stats integer counts, arrays initialized, timestamps nullable. Gunakan typed status/sort constants dan domain errors wrapping shared errors. Note normalize blank→nil, max2000 runes; invalid unknown fields ditolak parser handler Task 7.
- [ ] **4. Implementasikan pure mapping** read model → list/detail/summary. Profile null fallback untuk UI; role archived tetap punya refs/status archived. Accepted count dan filled capacity berbeda; no DB calls di mapper. Permission applicant-only selalu false.
- [ ] **5. Verifikasi tests** assert recursively snake_case JSON tags, array [] bukan null, nullable review legacy, no completion_score/custom_title/legacy aliases, exact owner/own project mini fields. Run command Langkah 2 → PASS.
- [ ] **6. Checkpoint** `git diff --check`; review bahwa DTO tidak mengekspos account flags/email/entity JSON. Usulan commit `feat: define canonical applicant contracts and policies`.

### Task 3: Owner summary/list/detail dari relasi authoritative

**Files:** Create application_service.go (base struct/constructor), application_repository.go (read interface subset), application_read_repository.go, application_read_service.go dan read tests; modify project read_service.go untuk eligible owner history read. **Consumes:** Task 2 projections/policies. **Produces:** NewApplicationService/GetApplicantSummary/ListProjectApplicants/GetApplicationDetail owner path dan repository scoped reads. WithApplicationTransaction ditambahkan ke ApplicationRepository bersama implementasi adapter Task4 agar Task3 tetap compile/test mandiri.

- [ ] **1. Tulis failing tests** `TestApplicationOwnerReadsAreScoped`: A creator tidak mendapat summary/list/detail P ketika B owner; B mendapat record, unauthorized empty P tetap 404. `TestApplicationReadMasterReferences` mengganti nama master dengan UUID tetap, dan label/search/options berubah; profile null/archived role/history tetap dihitung. `TestOwnerCanResolveClosedPrivateProject` owner boleh resolve slug closed/private, outsider tetap hidden.
- [ ] **2. Jalankan** `go test ./internal/modules/project -run '^TestApplication(OwnerReads|ReadMaster|FilteredCount|StablePagination)' -count=1` dan PostgreSQL read test melalui verifier Task 8 candidate → FAIL sebelum implementation.
- [ ] **3. Implementasikan read repository** actor/project scoped predicates, CountProjectRoleIDs termasuk archived untuk validation. Owner summary menggunakan SQL global counts/capacity/options dalam read snapshot; list count/filter/order/offset/limit lalu batch-load hanya role/profile/master pada page. Gunakan EXISTS skills/tools search; escape literal wildcard. Count/list tidak duplikat application.
- [ ] **4. Implementasikan service owner read/detail** actor reader policy dan actual owner guard; validate scope UUID sesudah owner gate. Detail memuat latest capacity plus permission context. ProjectDetailForActor mengizinkan owner eligible membaca project terlepas dari editable status, tanpa memberikan hak edit baru atau private application payload ke public detail.
- [ ] **5. Verifikasi** empty data=[]/total_pages=0, out-of-range page 200 empty, applied_at tie sort ID, OR/AND filters, q di semua field fixture, slot owner tidak dihitung, archived history options. Query count untuk 5 vs 100 applicants tidak bertambah per applicant; grouped hydration mengikuti page size.
- [ ] **6. Checkpoint** read unit/PostgreSQL tests → PASS, `git diff --check`. Usulan commit `feat: add scoped applicant reads and relational summaries`.

### Task 4: Atomic review, idempotency, dan editor safeguards

**Files:** Extend application_service.go/application_repository.go; create application_command_repository.go (ApplicationTx interface/adapter), application_service_test.go; modify repository.go CountRoleUsage dan validation.go; add validation tests. **Consumes:** Task 1–3 contracts/scoped identity/policy. **Produces:** ReviewApplication, WithApplicationTransaction/ApplicationTx adapter, correct pending role usage.

- [ ] **1. Tulis failing tests** `TestApplicationReviewCreatesOneContributor` asserts pending→accepted/member active/master ID/version+1/project still open; `TestApplicationReviewRetryPreservesDecisionTime` retry sama status+note no insert/timestamp/version bump, different note/status 409. `TestApplicationRejectClosedProject` reject pending on allowed states. `TestPendingApplicationsProtectRoleIdentityAndArchive` pending count mencegah archive/master identity change.
- [ ] **2. Jalankan** `go test ./internal/modules/project -run '^(TestApplicationReview|TestApplicationReject|TestPendingApplications)' -count=1` → FAIL. Fakes transaction meniru state failures tetapi tidak dianggap bukti race PostgreSQL.
- [ ] **3. Implementasikan transaction adapter/service** actor lock sebelum project; project→role→application; locked state/ownership/status di-check ulang. Applicant eligibility/initiator read tidak menambah user lock setelah project. Acceptance create contributor + review metadata + version/updated_at dalam tx yang sama; rejection metadata saja. Unique violation/RowsAffected mismatch→409; rollback callback returns error.
- [ ] **4. Implementasikan shared usage/editor protection** CountRoleUsage SQL counts active contributors AND pending applications; identity change jika pending>0 atau active>0 ditolak; archive existing loop memakai count nyata. Capacity boleh turun tidak di bawah active members; pending tidak reserve slot. Role archived boleh dibaca/ditolak tetapi tidak accepted.
- [ ] **5. Verifikasi tests** acceptance impossible full/member/owner/inactive/unverified/archived/non-open; reject tidak memerlukan slot tersedia atau applicant eligible. Retry auth tetap memeriksa owner terkini sebelum replay success. Failpoint setelah member insert sebelum decision/version rolls back semua; actual race di Task 8.
- [ ] **6. Checkpoint** tests project termasuk existing editor → PASS, `git diff --check`. Usulan commit `feat: review applications atomically and protect pending roles`.

### Task 5: Extend Apply dan viewer state untuk cooldown

**Files:** Modify service.go/ProjectTx repository.go/read_repository.go/read_service.go/dto.go; add application_eligibility_test.go dan existing project read tests. **Consumes:** persisted reviewed_at, shared reader/writer policy dan usage Task 4. **Produces:** extended Apply existing + viewer DTO, clock injection source-compatible.

- [ ] **1. Tulis failing tests** `TestApplicationCooldownBoundary`: now reviewed_at+71h59m59s denied, exactly+72h allowed; same applicant/project role lain denied, different project/actor allowed. `TestApplicationWithdrawDoesNotEraseRejectionCooldown` latest viewer withdrawn tetap blocked bila rejection lain belum expire. Legacy rejection without reviewed_at estimates retry_after from updated_at and marks it estimated.
- [ ] **2. Jalankan** `go test ./internal/modules/project -run '^TestApplication(Cooldown|WithdrawDoesNotErase|LegacyRejection|ApplyMembership|PortfolioURL|ViewerState)' -count=1` → FAIL.
- [ ] **3. Extend constructor/clock dan Apply existing** default time.Now, WithProjectClock option; active member guard; FindLatestRejectedApplication lookup applicant+project; revalidate active duplicate/role/capacity eligibility under current locks. Validate portfolio links absolute http/https, max10/max2048 existing; tidak menerima project_id atau owner/master identities dari client.
- [ ] **4. Extend viewer read** active pending/accepted preferred; otherwise latest history applied_at/id. reviewed_at/withdrawn_at/retry_after/retry_after_estimated milik viewer sendiri; latest rejected lookup independen. Guest null; no private notes of other people/pending global count. Preserve ownership DTO tanpa capabilities.
- [ ] **5. Verifikasi** old CreateDraft/Update/Publish callers compile tanpa constructor rewrite; portfolio javascript/data/file/relative ditolak. Viewer pending/accepted tetap tidak reapply; withdrawn tanpa cooldown boleh apply lagi; accepted member left tetap blocked sesuai existing unique policy. Run command Langkah 2 dan existing project tests → PASS.
- [ ] **6. Checkpoint** `git diff --check`; usulan commit `feat: enforce application membership and rejection cooldown`.

### Task 6: Own list/summary/detail dan withdrawal persisted

**Files:** Extend application_read_repository.go/read_service.go/service.go/command repository; add own read/withdraw tests. **Consumes:** Task 2–5 types, reader policy, clock, tx. **Produces:** ListMyApplications/GetMyApplicationSummary/WithdrawApplication dan applicant detail read.

- [ ] **1. Tulis failing tests** `TestMyApplicationReadsOnlyOwnHistory` actor A tidak mendapat B; default6/contexts origin keys/q owner/project fields; global summary beda dari page total. `TestApplicationWithdrawPersistsOnlyPending` own pending success, other's 404, accepted/rejected409, repeat withdrawn no-op waktu tetap.
- [ ] **2. Jalankan** `go test ./internal/modules/project -run '^(TestMyApplication|TestApplicationWithdraw|TestApplicantDetailPermissions)' -count=1` → FAIL.
- [ ] **3. Implementasikan own reads** grouped summary actor tanpa pagination; own paginated list scoped applicant_user_id, origins CSV/query spec r2; project mini/role refs batch. q author berdasarkan current owner_profile lalu organization fallback, bukan creator legacy. Note/timestamps/retry_after pada own list sesuai kartu.
- [ ] **4. Implementasikan applicant detail dan withdraw** detail owner OR applicant authorized; non-owner applicant permissions review false. Withdraw memakai reader eligibility, no extra verification; tx actor→project→role(include archived)→application; mark withdrawn_at/updated_at, review metadata tidak ditulis, no member/version changes. Final repeat semantics sama spec.
- [ ] **5. Verifikasi** owner review vs applicant withdraw satu pemenang via actual race Task 8; closed/archived project tidak mencegah withdrawal pending actor sendiri. Own legacy/profile missing/project visibility berubah tetap dapat membaca application miliknya dengan field minimum aman. No cross-user note leak.
- [ ] **6. Checkpoint** tests tasks1–6 → PASS; `git diff --check`. Usulan commit `feat: add personal application history and withdrawal`.

### Task 7: HTTP routes, strict request parsing, dan wiring

**Files:** Create application_handler.go/application_routes.go/application_handler_test.go dan test/application_routes_test.go; modify existing project handler.go Apply parsing, router.go/bootstrap/application.go/test/project_routes_test.go/test/master_data_integration_test.go. **Consumes:** ApplicationService Task 3–6. **Produces:** seluruh routes pada bagian 2, authenticated dan standar response.

- [ ] **1. Tulis failing route/handler tests** `TestApplicationRoutesAndPrivacy`: daftar route exact, guest401, B claimed owner200, creator A404, self-applicant detail200 tetapi review404, UUID invalid400, empty list200 dengan data=[]/meta. `TestReviewRequestRejectsServerOwnedFields` unknown keys/trailing JSON/status withdrawn/note2001→400. `TestApplicationHTTPQueryParsing` memastikan duplicate CSV dinormalisasi dan empty tokens/repeated keys/limit explicit0/page0/invalid UUIDs→400; `TestApplicationHTTPEncodedBodyLimit` valid payload sampai batas dan body melebihi192KiB ditolak aman.
- [ ] **2. Jalankan** `go test ./test -run '^TestApplicationRoutes' -count=1` dan `go test ./internal/modules/project -run '^TestApplicationHTTP' -count=1` → FAIL, endpoint belum registered.
- [ ] **3. Implementasikan handler/parser** static own summary/list routes jelas sebelum dynamic id; owner query defaults5 dan own6; repeated query/invalid CSV/explicit page0 tidak default diam-diam. Tambah helper `decodeApplicationRequest(c *gin.Context, target any) error` khusus transport dengan strict JSON decode satu object/EOF, batas encoded body192KiB sebelum decode agar batas field Apply existing tetap tertampung meskipun JSON escaped; helper transport ini bukan DTO domain map. Gunakan pada review dan Apply handler existing agar unknown server-owned fields ditolak, termasuk project_id dalam body Apply. Withdraw body kosong; body bermuatan JSON field tidak diterima. Gunakan c.Request.Context serta required actor helper; error helpers dan safe messages, tanpa raw validation SQL.
- [ ] **4. Wire constructor/router** bootstrap ApplicationRepository→ApplicationService(repo,time.Now)→ApplicationHandler. Tambah parameter `applicationHandler *project.ApplicationHandler` pada NewRouter dan update tiga callsites existing yang ditemukan: bootstrap, project_routes_test, master_data_integration_test. RegisterApplicationRoutes hanya bila handler non-nil. Existing project routes/constructor behavior tetap valid.
- [ ] **5. Verifikasi** privat success headers `Cache-Control: private, no-store`; routes negative method405/noRoute404 tetap envelope existing. Test blocked permissions, unauthorized summary tidak membocorkan counts, body parsed read identity server. Run commands Langkah 2 + project/master route regression → PASS.
- [ ] **6. Checkpoint** `git diff --check`; usulan commit `feat: expose authenticated application workflow routes`.

### Task 8: PostgreSQL concurrency, migration replay, dan bounded-query proof

**Files:** Extend test/application_workflow_integration_test.go; create scripts/verify-applicants.ps1; modify scripts/verify-project-editor.ps1; add script contract tests ke test/application_verifier_test.go. **Consumes:** Tasks 1–7 persisted behavior. **Produces:** evidence nyata invariants dan guarded schema replay.

- [ ] **1. Tulis failing integration cases** `TestApplicationLastSlotRaceAgainstPostgres`, `TestApplicationAcceptWithdrawRaceAgainstPostgres`, `TestApplicationReviewRollbackAgainstPostgres`, `TestApplicationEditorReviewRaceAgainstPostgres`, `TestApplicationReadBatchQueriesAgainstPostgres`. Gunakan separate DB connections/concurrent goroutines dan controlled barriers; jangan mengganti race dengan dua sequential mock calls. Rollback fault injection memakai trigger test scoped ke UUID fixture: satu case menggagalkan UPDATE application setelah member insert, satu case menggagalkan project version update setelah decision. Trigger/function diberi nama unik, dibuat hanya pada disposable DB, dan dibuang cleanup; tidak menambah production failpoint/config flag.
- [ ] **2. Tulis verifier guard cases** unknown/highest migration>11, non-loopback DB, wrong user/name, missing settings→refuse sebelum mutation. Guard tests tidak membutuhkan DB nyata. Jalankan `go test ./test -run '^TestApplicationVerifier' -count=1` → FAIL sebelum script rules ada.
- [ ] **3. Implementasikan verify-applicants** same target guard existing; set KOLABORIA_TEST_DATABASE_URL tanpa log password/URL; migration up→11, test suite application+editor baseline, fixture cleanup, down hanya11→10, re-up→11, rerun required application tests. Assert actual test executed, bukan skipped output. Seluruh created fixtures memakai UUID unik dan cleanup FK-safe.
- [ ] **4. Sesuaikan editor verifier secara terbatas** sebelum mutation periksa supported repo migrations/version hanya10/11. Pada schema11, rollback11 aman→10 terlebih dahulu setelah suite cleanup; lanjut replay baseline10→9→8 dan up→11. Pada schema10, alur baseline existing tetap sama. Unknown migration/version atau down preflight failure berhenti tanpa loop rollback otomatis tambahan. Editor verifier dan applicants verifier memakai DB disposable yang memang khusus replay, bukan reused application database.
- [ ] **5. Jalankan verification** dari API `./scripts/verify-applicants.ps1 -TestEnvFile ./tmp/applicants-test.env` dengan file target yang lolos guard. Assert last-slot exactly1 success/1 conflict/1 member; accept-vs-withdraw one terminal result; failpoint no partial member/status/version; editor capacity/archive race respects invariants; query count tidak linear terhadap applicants dan EXISTS tidak double count. Migration rollback rejected with persisted organization owner mempertahankan schema/data, lalu controlled cleanup memungkinkan replay.
- [ ] **6. Checkpoint** catat DB/schema/test executed dan query count evidence, `git diff --check`. DB tidak tersedia berarti hasil integration **belum terverifikasi**, bukan PASS. Usulan commit `test: verify application concurrency and migration safety`.

### Task 9: Canonical client DTO, services, presentation, dan invalidation

**Files:** Create app/types/application.ts/app/utils/application-presentation.ts/app/composables/useApplicationRefresh.ts dan tests/application-contract.test.mjs/application-api.test.mjs; modify types/project-editor.ts/types/project.ts/constants/api-endpoints.ts/services/application.service.ts/services/project.service.ts. **Consumes:** contract Task 2/7. **Produces:** typed gateways dan shared actor-scoped refresh signal.

Client service signatures:

```ts
ApplicationService.getProjectApplicants(projectId: string, query: OwnerApplicationQuery, signal?: AbortSignal): Promise<ApiPaginatedResponse<ApplicantListItemResponse>>
ApplicationService.getApplicantSummary(projectId: string, signal?: AbortSignal): Promise<ApiResponse<ApplicantSummaryResponse>>
ApplicationService.getApplicationDetail(id: string, signal?: AbortSignal): Promise<ApiResponse<ApplicationDetailResponse>>
ApplicationService.getMyApplications(query: MyApplicationQuery, signal?: AbortSignal): Promise<ApiPaginatedResponse<MyApplicationListItemResponse>>
ApplicationService.getMyApplicationSummary(signal?: AbortSignal): Promise<ApiResponse<MyApplicationSummaryResponse>>
ApplicationService.reviewApplication(id: string, request: ReviewApplicationRequest): Promise<ApiResponse<ApplicationDetailResponse>>
ApplicationService.withdrawApplication(id: string): Promise<ApiResponse<ApplicationDetailResponse>>
```

OwnerApplicationQuery uses page/limit/q/statuses/project_role_ids/sort; MyApplicationQuery uses page/limit/q/statuses/origins/sort. CSV serializer trims/dedupes/orders values consistently for request keys; enum labels tidak dikirim ke backend. Request options signal passed to useApi.$api; no custom auth/refresh implementation.

- [ ] **1. Tulis failing tests** canonical fixture DTO actual backend fields→presentation labels; null profile/archived role/no tools safely rendered; no `as any`/dummy completion score/custom title required. Legacy portfolio javascript/data/file/relative URL tidak menjadi href aktif; absolute http/https valid tetap tampil. Service tests assert exact URL/body/query/status envelope/meta; note payload cannot spoof reviewer. One page not mistaken for global stats.
- [ ] **2. Jalankan** dari client `node --test tests/application-contract.test.mjs tests/application-api.test.mjs` → FAIL modules/types/services belum tersedia.
- [ ] **3. Implementasikan typed services/canonical types** pakai ApiPaginatedResponse untuk list dan ApiResponse untuk detail/summary/mutations. ApplicantGateway pada app/types/application.ts mempunyai getProjectApplicants/getApplicantSummary/getApplicationDetail/reviewApplication signatures tepat seperti katalog Task9; MyApplicationGateway mempunyai getMyApplications/getMyApplicationSummary/getApplicationDetail/withdrawApplication. ApplicationService memenuhi kedua interface; tests inject partial gateway masing-masing. ProjectService application helper lama tidak dipakai untuk new pages. Add viewer timestamps/retry fields, keep existing project adapter; no backend alias fields introduced. Presentational helpers memformat values DTO/master dan `getSafePortfolioLinks(links: string[]): string[]` hanya mempertahankan URL absolute http/https max2048 untuk anchor, tanpa fetch URL atau mengubah jawaban stored. Kedua modal menggunakan helper ini dan rel=noopener noreferrer.
- [ ] **4. Implementasikan `useApplicationRefresh(accountId: Ref<string|null>)`** return `revision: ComputedRef<number>` dan `notifyApplicationMutation(actorId: string): void`. Simpan revision counters pada Nuxt useState actor-scoped; bukan application data/event bus/dependency baru. Notification setelah canonical success sekali saja; failed request tidak notify. Data loading composables clear buffers/abort ketika account berubah/logout, tidak memakai global static key `my-applications` tanpa actor.
- [ ] **5. Verifikasi** signal forwarded, stale-account response ignored by consumers Task10/11, cross-account revisions tidak memicu akses akun lain. UI error mengikuti utils/error existing; assert 500/raw SQL tidak ditampilkan dan trace_id tidak hilang. Run tests Langkah2 plus existing error/project mapper tests → PASS.
- [ ] **6. Checkpoint** `git diff --check`; usulan commit `feat: add canonical application client contracts`.

### Task 10: Owner composable, applicants page, dan modal review

**Files:** Create app/composables/useProjectApplicants.ts/tests/project-applicants-state.test.mjs/tests/project-applicants-page.test.mjs; modify pages/projects/[slug]/applicants.vue/components/project/ApplicantDetailModal.vue. **Consumes:** Task9 gateway/refresh signal, API Task7. **Produces:** server-backed owner UX tanpa local fixture/review increments.

Interface: `useProjectApplicants(accountId: Ref<string|null>, projectId: Ref<string|null>, gateway: ApplicantGateway)`; default page injects ApplicationService methods. Returns summary/items/meta, filters, pending/error/loadMoreError/detail/submitting, `refresh()`, `loadPage(page:number)`, `loadMore()`, `openDetail(id:string)`, `review(status:'accepted'|'rejected',note:string)`. ApplicantGateway signatures identik owner methods Task9; tests inject controllable promise gateway.

- [ ] **1. Tulis failing behavioral tests** mounted real composable/setup: multiple status/roles serialization AND/OR, filter change resets1, search200ms, page5, mobile append dedupe, page beyond end clamp, summary global independent page, same timestamp records, detail stale pending→accepted refetch. Delayed old query/account/project response tidak menimpa state baru. Error initial/append/detail bukan empty.
- [ ] **2. Jalankan** `node --test tests/project-applicants-state.test.mjs tests/project-applicants-page.test.mjs` → FAIL sebelum wiring; import helpers/frontend-runtime.mjs dan mount real module/setup. Jangan menulis copy filter implementation sebagai test.
- [ ] **3. Implementasikan state orchestration** sequence/request generation + AbortController; actor/project/filter-aware identity; no duplicate same-page fetch; stale/aborted response ignored. Summary/list punya state failure masing-masing. Watch filters+revision, reload page1/summary pada mutation signal; reject409 refresh canonical tetapi pertahankan note. On disposed/logout buffers detail cleared and in-flight invalidated.
- [ ] **4. Wire page/modal** auth middleware sesuai read policy; resolve reactive slug→UUID via existing owner-readable detail; fetch owner summary/list. Replace card field access with canonical applicant_profile/project_role.skills/tools. Modal props ApplicationDetailResponse|null dan detail loading/error; owner permission controls accept/reject separately. Submit blocks double click, stale selected ID not mutated. Remove DEMO fixture/badge/delay/capacity increments; keep existing design tokens/components.
- [ ] **5. Verifikasi review UX** success uses server DTO, closes modal, notify actor revision once, refresh stats/list; no locally invented time/count. Failure retains note and does not toast success;409 refetch reflects full slot/final status. Desktop mode threshold1024px matches lg sidebar; mobile steps5. Confirm role badge label reflects requirements posisi.
- [ ] **6. Checkpoint** tests Langkah2 + existing project navigation/count tests → PASS; `git diff --check`. Usulan commit `feat: connect owner applicant management to persisted API`.

### Task 11: My Applications terbaru, summary dashboard, dan retirement helper legacy

**Files:** Create app/composables/useMyApplications.ts/tests/my-applications-state.test.mjs/tests/home-application-summary.test.mjs; update latest pages/projects/my-applications.vue and tests/my-applications-page.test.mjs; modify application counters only in pages/home.vue/composables/useProjects.ts/remaining ProjectService delegates; connect useMyProjectList.ts refresh revision. **Consumes:** Task9 APIs/notification. **Produces:** own server-filtered list/detail/withdraw and consistent counters without lost user layout work.

Interface: `useMyApplications(accountId: Ref<string|null>, gateway: MyApplicationGateway)` returns summary/items/meta/filters/read states/detail/submitting with refresh/loadPage/loadMore/openDetail/withdraw(id:string). Gateway uses Task9 own methods; default limit6. No loop fetch all application pages just for stats or filtering.

- [ ] **1. Tulis failing tests** own query sends origin enums not translated labels; summary zero remains zero; list empty shows actual empty and never dummy. Withdraw network/error409 never local withdrawn/success; canonical200 updates UI then notification/reload. Modal note and project summary preserved, pagination clamps; account/filter race drops old results.
- [ ] **2. Jalankan** `node --test tests/my-applications-state.test.mjs tests/my-applications-page.test.mjs tests/home-application-summary.test.mjs` → FAIL for current dummy/swallowed-error behavior. Refactor old test yang menyalin fungsi monogram/stats untuk mengimpor helper/composable nyata.
- [ ] **3. Integrasikan working tree terbaru** inspect diff sebelum edit dan gunakan existing template/layout/filter sheets/modal. Replace DUMMY_APPLICATIONS/default fallback/any creator-context helpers; use canonical project.origin/owner_profile/master names and own summary. Remove `.catch(()=>{})` withdrawal; read detail endpoint saat modal dibuka, preserve actual pending on failure. Gunakan auth middleware pada My Applications sesuai read policy r2, tanpa onboarding-guard yang mewajibkan verified untuk read; eligibility/auth error tetap memberi langkah next yang aman. Jangan ubah middleware global atau home policy. Keep six items/page, mode1024px, submitted data text from application not profile.
- [ ] **4. Sesuaikan home application counters secara terbatas** replace getMyApplications length/filter/fallback3/2/1 dengan ApplicationService.getMyApplicationSummary; loading/unavailable nullable display, valid0 tidak diganti dummy. Keep other home widgets unchanged. Listen revision untuk refresh own summary; useMyProjectList listens account/revision and uses existing refresh mechanism so pending applicant count eventually fresh, without adding all-applications loading.
- [ ] **5. Retire legacy helpers setelah consumer migration** `rg -n 'getMyApplications|getProjectApplicants|reviewApplication|withdrawApplication' app` harus menunjukkan consumer typed baru; hapus unused application wrappers/delegates useProjects/ProjectService yang return legacy arrays atau swallow errors. Jangan menghapus unrelated project methods. Pastikan no application-owned `Application` legacy type import pada touched pages/modal/services; type lama boleh tetap untuk consumer luar yang tidak dipakai scope ini sampai audit selesai.
- [ ] **6. Checkpoint** tests Langkah2 + existing my-project summary/list tests → PASS; compare user diff: latest UI preserved, changes fokus API state/types. `git diff --check`. Usulan commit `feat: persist personal application actions and dashboard counts`.

### Task 12: Detail project CTA, viewer cooldown, dan owner hasil claim

**Files:** Modify utils/project-presentation.ts/pages/projects/[slug]/index.vue/types/project-editor.ts; add/update tests/project-application-status-copy.test.mjs/tests/my-project-applicant-count.test.mjs/tests/project-applicant-owner-flow.test.mjs. **Consumes:** Task5 viewer DTO/owner read, Task9/10/11 notifications. **Produces:** consistent CTA/copy/manage link without reviving obsolete capabilities/claim simulation.

- [ ] **1. Tulis failing cases** organization creatorA/ownerB: B mendapat Kelola Pelamar link dan owned pending count, A does not. Viewer rejection sebelum72h sees retry time; at72h eligible CTA available; legacy time unknown safe unavailable/reload; withdrawn with active rejection cooldown remains blocked. Guest/unverified next action navigable, not permanent disabled submit.
- [ ] **2. Jalankan** `node --test tests/project-application-status-copy.test.mjs tests/my-project-applicant-count.test.mjs tests/project-applicant-owner-flow.test.mjs` → FAIL untuk retry/latest organization fixture rules yang belum ditangani.
- [ ] **3. Extend project adapter/copy** carry viewer reviewed_at/withdrawn_at/retry_after/reapply_block_reason while preserving ownership and master mappings. Derive owner from actual owner_id independently of definition editor/initiator. Eligible owner reads archived/private history; manage action navigates while acceptance restrictions explained inside management page.
- [ ] **4. Connect refresh** project lazy data key includes reactive slug and actor where viewer-specific; revision invalidates current project state and pending summary loading. Apply success refreshes persisted viewer; review/withdraw success from other active screen signals refetch on return/mount. No public count of other applicants, no generic can_* capabilities object, no claim fixture/automatic owner assignment.
- [ ] **5. Verifikasi** owner identity/master data preserved after mapper; already member/active app not reapply; capacity full role does not incorrectly close other role. 401/verification retains useful next step using routes existing. Claim dependency status documented, not hidden by UI success simulation.
- [ ] **6. Checkpoint** tests Langkah2 + existing editor/detail tests → PASS, `git diff --check`. Usulan commit `feat: align project application state with current ownership`.

### Task 13: Contract documentation, full verification, dan release readiness

**Files:** API handler annotations/docs/docs.go/swagger.json/swagger.yaml/docs/project-editor.md/README.md; spec and this plan execution checkboxes; existing test route/swagger coverage. **Consumes:** delivered Tasks1–12. **Produces:** synchronized docs/evidence and clearly scoped release status.

- [ ] **1. Tambah contract coverage tests** registered paths/strict DTO tags/private data fields/numeric meta; ensure exact routes section2 appear in Swagger. Protect project editor DTO without generic capabilities and unchanged create/update/publish fields. Run targeted route/swagger tests → FAIL until docs regenerated.
- [ ] **2. Update annotations/docs** source annotations pada application_handler.go dan existing Apply/detail handler where changed; project-editor doc links latest applicants spec/plan and distinguishes available routes from claim dependency. `task swagger` generate current artifacts; inspect diff, never copy deprecated Swagger/entities. Add current verifier README usage and legacy/rollback constraints.
- [ ] **3. API checks** API root: `gofmt` only touched Go files; `go test ./... -count=1`, `go vet ./...`, `go build ./...`, `git diff --check`. If integration tests skip because no DSN, explicitly list them pending; regular suite PASS alone does not prove PostgreSQL races/schema.
- [ ] **4. Client checks** client root: `pnpm test`, `pnpm exec nuxt typecheck`, `pnpm build`; use installed dependencies/runtime, don't install alternate framework. Node/Vue setup tests prove real page/composable behavior; build/typecheck catch canonical DTO mismatch with latest user-edited my-applications. Inspect existing failures and separate baseline from new regression without silently accepting errors.
- [ ] **5. Execute guarded PostgreSQL and UI journey checks** verifier Task8 plus browser/manual checklist below. Report actual commands/results, updated master query behavior, zero-data/error states, and organization claimed owner fixture. Mark claim→manage via UI dependency pending until endpoint claim available; never report mock claim as end-to-end success.
- [ ] **6. Checkpoint/handoff** compare spec13 acceptance matrix to completed tasks and fill evidence table below; mark implementation complete only for tested delivered scope. Prepare final review diff and concise commit/PR descriptions; publish/merge/deploy only when separately requested. This plan-writing request itself does not execute any of these steps.

## 6. Browser/manual acceptance checklist

- [ ] Owner personal loads project summary/global stats/5-item list, status+role multi-filter, q/sort, desktop pagination and mobile append.
- [ ] Persisted organization project creator A/owner B: B opens summary/list/detail and accepts/rejects; A/member/contributor cannot access applicants by manipulating URL/client state. Organization attribution still visible.
- [ ] Applicant has own 6-item pagination/context origin search and global stats; modal displays persisted submission/portfolio/reviewer note; own detail never exposes another applicant.
- [ ] Valid empty list remains empty; disconnected backend shows retry; append failure retains loaded items; no dummy or fabricated counts.
- [ ] Accept last slot refreshes all cards' role capacity, global pending/accepted stats and my-project count; decision persisted after refresh/re-login.
- [ ] Reject note appears on applicant own page; retry-before72h denied across roles; other project/actor unaffected; exact boundary eligible.
- [ ] Withdraw API failure gives no success toast/local status change; race409 refetch; success persisted and no new cooldown.
- [ ] Switch actor/project/filter with delayed request: old private data absent; modal context not wrong; stale decision not applied to new selected application.
- [ ] Role archived history and null profile render fallback without losing count; master rename reflected after refetch; badge says role requirements.
- [ ] Tablet768–1023px uses mobile list/filter behavior consistently;1024px+ uses desktop pagination/sidebar; no duplicate load on breakpoint change.
- [ ] Existing project editor draft/publish/role master selection round-trip still works; stale version after accept conflicts rather than overwriting capacity state.

## 7. Coverage, evidence, dan dependency ledger

| Requirement | Task owner | Evidence required |
| --- | --- | --- |
| Latest references supersede old/deprecated code | 1, 2, 9, 13 | Contracts/schema/code compared to r2, not old plan. |
| Claimant owner rights and relational identity | 1, 3, 4, 7, 10, 12 | Real persisted org-owner fixture; creator-negative tests; CTA/page access. |
| Master/reference consistency and history | 2, 3, 8, 9 | Master rename read/search; FK/scope; profile-null/role-archived fixtures. |
| Filter/search/sort/pagination and global counts | 2, 3, 6, 10, 11 | Query/handler assertions, no JOIN duplication, page vs stats boundary. |
| Atomic accept/reject/withdraw/idempotency | 4, 6, 8 | PostgreSQL final-slot/race/rollback plus repeat request invariants. |
| Apply membership/cooldown/portfolio URLs | 5, 8, 12 | Clock boundary tests, cross-pair scope, unsafe URL denial. |
| Latest My Applications/home consumer consistency | 9, 11 | Preserved local layout diff; no dummy/swallow; canonical summary zero/error. |
| Clean code/scalable query/readability | 2–9, 13 | Layer boundaries, pure mapping, batch query count, gofmt/vet/typecheck/reviewer diff. |
| Migration safety and editor regression | 1, 4, 8, 13 | Guarded version replay/down preflight, editor round-trip/version conflict. |
| API/privacy/envelope/documentation | 7, 9, 13 | Auth matrix, JSON tags, private cache policy, generated Swagger. |

Dependency claim nyata: setelah fitur claim tersendiri tersedia, jalankan real claim B → owner persisted/member/open → Kelola Pelamar→review journey. Sebelum itu, paket ini membuktikan compatibility dan authorization owner hasil claim lewat persisted fixture, bukan menyediakan action claim. Tidak ada task yang menganggap membership contributor saja atau creator_id sebagai jalan pintas.

Integration DB unavailable, runtime/dependency unavailable, atau docs context referenced-but-missing dicatat sebagai keterbatasan aktual pada evidence; tidak diganti dummy tests. Rujukan knowledge/context skill yang hilang pada workspace tetap limitation konteks; sebelum implementasi cek ulang apakah kini tersedia dan reconcile bila berisi keputusan yang lebih baru, tanpa mengalahkan arahan pengguna.

## 8. Self-review rencana

- [x] Scope dan terbaru-source hierarchy tercatat; legacy API/plan hanya konteks.
- [x] Requirement spec memiliki task di coverage matrix; owner organization constraint dependency tidak disembunyikan.
- [x] Interfaces/read-vs-write responsibilities/DTO canonical dan constructor wiring ditentukan.
- [x] Five Review Focus cases memiliki tests di task pemilik.
- [x] Snapshot changes my-applications dan consumer home masuk scope integrasi terbatas, tanpa reset perubahan pengguna.
- [x] Migration replay dan test skip dibedakan dari bukti runtime sukses.
- [x] Tidak ada implementasi, migration execution, commit, deployment, atau subagent dispatch dalam penyusunan rencana ini.

Langkah checkbox task di atas semuanya tetap belum dieksekusi. Spec r2 dan plan ini adalah artefak review untuk implementasi berikutnya.
