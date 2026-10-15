## Konteks sebelum implementasi fitur Kolaboria

Sebelum implementasi, perubahan perilaku, atau review implementasi, baca [skill kolaboria-feature-context](../.agents/skills/kolaboria-feature-context/SKILL.md), lalu [Implementation Quick Reference](../kolaboria-knowledge/04-development/implementation-quick-reference.md). Ikuti rujukan sesuai task dan ringkas kesepakatan fitur, scope, acceptance, serta verifikasinya sebelum edit kode. Jika otorisasi implementasi sudah tersedia, lanjutkan tanpa meminta approval yang sama kembali. Pengarah ini berlaku juga ketika chat dimulai langsung dari akar repo ini; skill dapat dibaca melalui path meskipun tidak muncul di selector repo anak.

## Standarisasi Output Response API (Wajib snake_case)

Seluruh output response HTTP dari backend Kolaboria WAJIB mematuhi format snake_case tanpa terkecuali:
- Seluruh JSON key pada struct DTO response dan metadata WAJIB menggunakan `snake_case` (misal: `access_token`, `refresh_token`, `created_at`, `updated_at`, `is_active`, `total_pages`, `village_id`, `postal_code`, `trace_id`).
- Dilarang keras menggunakan `camelCase` pada field DTO response.
- Response Sukses (200/2xx) wajib menggunakan format:
  ```json
  {
    "status": 200,
    "message": "[Success Message]",
    "data": any | omitempty
  }
  ```
  (Gunakan `response.Success(c, status, message, data)` atau `response.SuccessWithMeta(...)` untuk list dengan pagination).
- Response Error (4xx/5xx) wajib menggunakan format:
  ```json
  {
    "status": 400,
    "message": "[Error Message]",
    "data": {
      "err_message": "[Exception or error details]",
      "trace_id": "[Trace ID UUID]"
    }
  }
  ```
  (Gunakan `response.Error(c, status, message)`).
- Tag struct DTO response wajib didefinisikan secara eksplisit: `json:"field_name"` dan jika opsional nil: `json:"field_name,omitempty"`.
- Rujuk skill [kolaboria-response-convention](.agents/skills/kolaboria-response-convention/SKILL.md) dan [kolaboria-api-dev](.agents/skills/kolaboria-api-dev/SKILL.md) saat membuat endpoint baru.

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

When the user types `/graphify`, use the installed graphify skill or instructions before doing anything else.

Rules:
- For codebase questions, first run `graphify query "<question>"` when graphify-out/graph.json exists. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- Dirty graphify-out/ files are expected after hooks or incremental updates; dirty graph files are not a reason to skip graphify. Only skip graphify if the task is about stale or incorrect graph output, or the user explicitly says not to use it.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
