# Master data API

Ketiga katalog hanya menyediakan endpoint baca publik:

| Katalog | Endpoint | Nilai awal |
| --- | --- | ---: |
| Peran kontribusi | `GET /api/v1/master-data/contribution-roles` | 30 |
| Skills | `GET /api/v1/master-data/skills` | 50 |
| Tools | `GET /api/v1/master-data/tools` | 80 |

Query `page` default `1`; `limit` default `50` dan maksimum `100`; `q` adalah pencarian substring literal, case-insensitive, setelah trim, maksimum 100 karakter Unicode. Parameter yang dikenal tidak boleh muncul lebih dari sekali. Hasil kosong tetap mengembalikan array kosong.

Respons sukses memakai envelope `status`, `message`, `data`, dan `meta`. Setiap item hanya berisi `id`, `name`, dan `slug`; metadata memuat `page`, `limit`, `total`, dan `total_pages`.

Schema dan 160 nilai awal berada pada migration SQL `000001` dan `000002`. Lihat [petunjuk migration](../migrations/README.md) dan [implementation plan](superpowers/plans/2026-10-03-master-data-current-boilerplate.md) untuk constraint, manifest lengkap, dan prosedur verifikasi.
