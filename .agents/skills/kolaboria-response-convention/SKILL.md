---
name: kolaboria-response-convention
description: Pedoman standarisasi format response HTTP JSON (wajib snake_case, standard envelope sukses & error, trace_id) untuk backend Kolaboria API. Wajib dirujuk saat membuat modul baru, endpoint baru, atau DTO response baru.
---

# Panduan Standarisasi Response API Kolaboria (Wajib snake_case)

Dokumen ini adalah acuan baku bagi seluruh developer dan AI coding agent saat membuat, memodifikasi, atau mereview endpoint HTTP di **`kolaboria-api`**.

---

## 1. Aturan Mutlak Penamaan Key Response (`snake_case`)

1. **Seluruh field pada output response JSON WAJIB menggunakan `snake_case`**.
   - Contoh BENAR: `access_token`, `refresh_token`, `email_verified_at`, `is_active`, `created_at`, `total_pages`, `village_id`, `postal_code`, `trace_id`.
   - Contoh SALAH (DILARANG): `accessToken`, `refreshToken`, `emailVerifiedAt`, `isActive`, `createdAt`, `totalPages`, `villageId`, `postalCode`.
2. **Definisikan JSON tag eksplisit pada setiap field DTO response**: Setiap struct yang menjadi payload response harus memiliki tag `json:"nama_field"`. Jangan biarkan Go menggunakan nama field struct default (PascalCase).
3. **Pemisahan Request vs Response**:
   - Tag JSON pada **Request DTO** dapat menyesuaikan konsumen yang sudah ada (misalnya camelCase pada auth payload).
   - Namun tag JSON pada **Response DTO** SELALU dan MUTLAK `snake_case`.

---

## 2. Standar Envelope Response

Setiap response HTTP yang dikirimkan ke client wajib menggunakan envelope yang telah distandarisasi di package `internal/http/response`:

### A. Response Sukses (200, 201, dll.)

```json
{
  "status": 200,
  "message": "Operasi berhasil",
  "data": {
    "id": "uuid-string",
    "name": "Contoh",
    "created_at": "2026-10-03T12:00:00Z"
  }
}
```

- Jika `data` bernilai `nil`, field `data` otomatis di-omit (`omitempty`):

```json
{
  "status": 200,
  "message": "Data berhasil dihapus"
}
```

- Khusus list berpagination, ditambahkan field `meta` (juga snake_case):

```json
{
  "status": 200,
  "message": "Success",
  "data": [...],
  "meta": {
    "page": 1,
    "limit": 50,
    "total": 100,
    "total_pages": 2
  }
}
```

### B. Response Error (400, 401, 403, 404, 409, 422, 500, 503, dll.)

```json
{
  "status": 400,
  "message": "Invalid query parameters",
  "data": {
    "err_message": "invalid query parameters: unexpected field",
    "trace_id": "c043419c-3c26-4a6d-a6a9-8c406efc21f1"
  }
}
```

- `status`: integer sama dengan HTTP status code.
- `message`: string pesan error yang aman bagi client (hindari membocorkan detail raw database/SQL).
- `data`: objek yang berisi `err_message` (exception/error yang tersedia, atau pesan error sebagai fallback) dan `trace_id` untuk menghubungkan error dengan server log. Pesan top-level tetap aman bagi client; jangan menaruh stacktrace penuh atau rahasia konfigurasi pada payload production.

---

## 3. Cara Penggunaan di Handler Go

Gunakan package helper `github.com/syahrulfahmi/kolaboria-api/internal/http/response`:

```go
import "github.com/syahrulfahmi/kolaboria-api/internal/http/response"

// 1. Response sukses biasa
response.Success(c, http.StatusOK, "Login successful", authResponse)
// atau untuk 200 OK:
response.OK(c, "Login successful", authResponse)

// 2. Response sukses 201 Created
response.Success(c, http.StatusCreated, "User registered successfully", regResponse)

// 3. Response list dengan pagination
response.SuccessWithMeta(c, http.StatusOK, "Success", items, meta)

// 4. Response error
response.Error(c, http.StatusBadRequest, "Invalid query parameters")
response.Error(c, http.StatusNotFound, "Resource not found")
response.Error(c, http.StatusInternalServerError, "Internal server error")

// 5. Response validation error
response.ValidationError(c, err.Error())
```

> **PENTING:** Jangan pernah merender JSON error secara manual seperti `c.JSON(400, gin.H{"error": ...})`. Selalu gunakan `response.Error` atau `response.Success`.

---

## 4. Contoh Definisi Struct DTO Response

Di dalam berkas `dto.go` setiap modul:

```go
package example

import (
	"time"
	"github.com/google/uuid"
)

// CONTOH RESPONSE DTO YANG BENAR (Semua snake_case)
type ExampleResponse struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	IsActive    bool       `json:"is_active"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	VerifiedAt  *time.Time `json:"verified_at,omitempty"`
}

type ExampleDetailResponse struct {
	ExampleResponse
	ParentCategory string `json:"parent_category"`
	PostalCode     string `json:"postal_code"`
}
```

---

## 5. Checklist Verifikasi Saat Menambah Fitur Baru

Sebelum menyelesaikan task atau submit code:

1. [ ] Periksa seluruh struct berakhiran `Response` di `dto.go` modul Anda.
2. [ ] Pastikan tidak ada satupun tag `json:"camelCase"`. Semua harus `json:"snake_case"`.
3. [ ] Pastikan endpoint mengembalikan `response.Success` / `response.Error`.
4. [ ] Jalankan unit test `go test ./...`.
5. [ ] Jalankan regenerasi Swagger `task swagger`.
