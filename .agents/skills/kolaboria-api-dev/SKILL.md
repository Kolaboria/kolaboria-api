---
name: kolaboria-api-dev
description: Triggered when modifying or adding features to the Kolaboria Go API backend application.
---

# Kolaboria Backend Development Guide & Agent Customization

Pedoman instruksi bagi agen kecerdasan buatan (AI agent) saat memodifikasi, memelihara, atau menambahkan fitur baru di dalam workspace backend **`kolaboria-api`**.

---

## 1. Indeks Dokumen Standarisasi Backend

Sebelum mulai menulis kode backend, selalu rujuk dokumen standarisasi detail yang ada di `workspace/docs/` untuk pemahaman arsitektur penuh:

- ⚙️ **Standar API Go:** [GO_API_STANDARDIZATION.md](/development/workspace/docs/backend/GO_API_STANDARDIZATION.md)
- 📌 **Lembar Contekan Backend:** [QUICK_REFERENCE.md](/development/workspace/docs/backend/QUICK_REFERENCE.md)
- 🌿 **Git Commit & Templat PR:** [GIT_WORKFLOW.md](/development/workspace/docs/common/GIT_WORKFLOW.md)

---

## 2. Arsitektur Proyek (Modular Clean Architecture)

Backend Kolaboria ditulis menggunakan **Go** dengan menerapkan pola Modular Clean Architecture di bawah direktori `internal/modules/<nama_modul>/`. Setiap modul harus memisahkan tanggung jawab kode ke dalam berkas-berkas standar berikut:

- **`routes.go`:** Pendeklarasian route endpoint HTTP menggunakan router group dari framework **Gin** (`github.com/gin-gonic/gin`).
- **`handler.go`:** Penerimaan request HTTP, parsing payload/query parameter ke dalam struktur Data Transfer Object (DTO), validasi input, pemanggilan layer Service, dan pengiriman response HTTP JSON.
- **`dto.go`:** Struktur data (_struct_) request & response payload khusus untuk API input-output.
- **`service.go`:** Implementasi logika bisnis utama (_business logic_), orkestrasi transaksi database, penegakan aturan hak akses, dan pemanggilan layer Repository.
- **`repository.go`:** Berisi interface dan implementasi query database menggunakan ORM **GORM** (`gorm.io/gorm`).
- **`entity.go`:** Definisi struktur tabel database (_GORM models_) yang mencerminkan skema database fisik.
- **`mapper.go`:** Fungsi utilitas murni untuk konversi timbal-balik antara database `entity` dan output `DTO`.

---

## 3. Aliran Eksekusi Kode & Aturan Main (Code Flow Rules)

Setiap request HTTP wajib mengikuti alur eksekusi searah berikut:

```
[Client Request] ➜ routes ➜ handler ➜ service ➜ repository ➜ database
```

- **Handler** dilarang memanggil repository secara langsung (harus melalui service).
- **Service** dilarang mengembalikan objek database model (`entity`) langsung ke handler (harus dikonversi menjadi `DTO` terlebih dahulu menggunakan `mapper` sebelum dikirim balik).
- **Repository** hanya berfokus pada eksekusi query SQL (menggunakan GORM `db.Where()`, `db.Create()`, dsb.) dan mengembalikan objek `entity` atau list `entity`.

---

## 4. Validasi Input, Standarisasi Response, & Penanganan Error

- **Validasi Struct:** Gunakan tag validator Gin pada _struct_ DTO di dalam `dto.go` untuk memvalidasi parameter secara deklaratif:
  ```go
  type CreateProjectRequest struct {
      Title       string `json:"title" binding:"required,min=5,max=100"`
      Description string `json:"description" binding:"required"`
  }
  ```
- **Handler Validation & Error:** Di dalam handler, parse dan validasi DTO menggunakan `c.ShouldBindJSON()`. Gunakan helper standar `response.Error` atau `response.ValidationError`:
  ```go
  var req CreateProjectRequest
  if err := c.ShouldBindJSON(&req); err != nil {
      response.Error(c, http.StatusBadRequest, err.Error())
      return
  }
  ```
- **Standarisasi Response (WAJIB snake_case):**
  - Seluruh field pada struct DTO response WAJIB menggunakan tag `snake_case`:
    ```go
    type ProjectResponse struct {
        ID          uuid.UUID `json:"id"`
        Title       string    `json:"title"`
        CreatedBy   string    `json:"created_by"`
        CreatedAt   time.Time `json:"created_at"`
        TotalMembers int      `json:"total_members"`
    }
    ```
  - Dilarang keras menggunakan `camelCase` pada response struct (misalnya `createdBy`, `totalMembers`).
  - Response sukses: gunakan `response.Success(c, statusCode, message, data)` atau `response.SuccessWithMeta(c, statusCode, message, data, meta)` untuk list berpagination.
  - Response error: gunakan `response.Error(c, statusCode, message)` (otomatis menyertakan `data: { "trace_id": "..." }`).
- **Penanganan Error:** Service harus mengembalikan error yang spesifik jika terjadi kegagalan logika bisnis, dan handler bertugas menerjemahkannya ke dalam kode status HTTP yang tepat menggunakan `response.Error(c, statusCode, message)`. Rujuk skill `kolaboria-response-convention` untuk panduan lengkap.

---

## 5. Migrasi Database (Database Migrations)

- Perubahan skema database PostgreSQL dikelola secara terpisah melalui file SQL migrasi di dalam direktori `migrations/`.
- Setiap migrasi wajib menyediakan sepasang file:
  - `XXXXXX_nama_perubahan.up.sql` : Skrip SQL untuk menerapkan perubahan (contoh: `CREATE TABLE`, `ALTER TABLE`).
  - `XXXXXX_nama_perubahan.down.sql` : Skrip SQL untuk membatalkan perubahan (contoh: `DROP TABLE`, `DROP COLUMN`).
- Dilarang keras memodifikasi skema database secara langsung atau mengandalkan fitur auto-migrate GORM di fase produksi.
