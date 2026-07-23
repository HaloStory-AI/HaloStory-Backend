# Admin API — HaloStory Backend

**Base URL:** `https://api-hsaiapp.my-bini.my.id/api/admin`

**Auth:** Semua endpoint membutuhkan:
1. JWT token (`Authorization: Bearer <token>`) — login via `POST /api/login`
2. Role `"admin"` — dicek oleh `CheckAdmin()` middleware (return 403 jika bukan admin)
3. Tidak dalam status banned

---

## Daftar Endpoint

| # | Method | Path | Deskripsi |
|---|--------|------|-----------|
| 1 | GET | `/api/admin/users` | List semua user |
| 2 | PUT | `/api/admin/users/:id/role` | Ubah role user |
| 3 | POST | `/api/admin/users/:id/make-vip` | Grant/extend VIP |
| 4 | POST | `/api/admin/users/:id/ban` | Ban user |
| 5 | POST | `/api/admin/users/:id/unban` | Unban user |
| 6 | DELETE | `/api/admin/users/:id` | Hapus user |
| 7 | GET | `/api/admin/bots` | List semua bot |
| 8 | POST | `/api/admin/bots` | Tambah bot baru |
| 9 | POST | `/api/admin/bots/import` | Import CSV |
| 10 | PUT | `/api/admin/bots/:id` | Update bot |
| 11 | DELETE | `/api/admin/bots/:id` | Hapus bot |
| 12 | POST | `/api/admin/bots/bulk-assign-category` | Pindahkan bot ke kategori |
| 13 | POST | `/api/admin/bots/bulk-store` | Bulk add banyak bot |
| 14 | GET | `/api/admin/categories` | List kategori |
| 15 | POST | `/api/admin/categories` | Tambah kategori |
| 16 | PUT | `/api/admin/categories/:id` | Update kategori |
| 17 | DELETE | `/api/admin/categories/:id` | Hapus kategori |
| 18 | GET | `/api/admin/settings` | Lihat pengaturan |
| 19 | PUT | `/api/admin/settings` | Simpan pengaturan |
| 20 | GET | `/api/admin/stats` | Statistik ringkas |
| 21 | GET | `/api/admin/stats/charts` | Data grafik aktivitas |
| 22 | GET | `/api/admin/student-requests` | List request murid |
| 23 | PUT | `/api/admin/student-requests/:id/status` | Update status request |

---

## 1. GET /api/admin/users

List semua user. **Tidak ada pagination.**

### Response 200
```json
[
  {
    "id": 1,
    "name": "string",
    "email": "string",
    "role": "string",
    "is_banned": false,
    "ban_reason": "string | null",
    "vip_expires_at": "2026-01-01T00:00:00Z | null",
    "created_at": "2026-01-01T00:00:00Z"
  }
]
```

---

## 2. PUT /api/admin/users/:id/role

### Request
```json
{
  "role": "admin" | "Sensei"
}
```

### Response 200
```json
{
  "message": "Role updated successfully"
}
```

### Error
- `422` — role harus `"admin"` atau `"Sensei"`
- `400` — cannot demote yourself
- `404` — user not found

---

## 3. POST /api/admin/users/:id/make-vip

### Request
```json
{
  "duration": 30
}
```
`duration`: 1–3650 hari.

### Response 200
```json
{
  "message": "VIP granted successfully",
  "vip_expires_at": "2026-08-22T00:00:00Z"
}
```

### Logic
- Jika user sudah VIP dan belum expired → diperpanjang dari expiry lama
- Jika user bukan VIP atau sudah expired → mulai dari sekarang

### Error
- `422` — duration wajib 1-3650 hari
- `404` — user tidak ditemukan

---

## 4. POST /api/admin/users/:id/ban

### Request
```json
{
  "reason": "Melanggar aturan",
  "duration": "permanent" | "1" | "7" | "30"
}
```
- `"permanent"` atau `""` → banned_until = null (permanent)
- `"1"` → 1 hari
- `"7"` → 7 hari
- `"30"` → 30 hari

### Response 200
```json
{
  "message": "User banned successfully"
}
```

### Error
- `422` — reason wajib diisi, atau duration tidak valid
- `404` — user tidak ditemukan

---

## 5. POST /api/admin/users/:id/unban

### Request
Body kosong.

### Response 200
```json
{
  "message": "User unbanned successfully"
}
```

### Error
- `404` — user tidak ditemukan

---

## 6. DELETE /api/admin/users/:id

### Request
Body kosong.

### Response 200
```json
{
  "message": "User deleted successfully"
}
```

### Error
- `400` — cannot delete yourself
- `404` — user not found

---

## 7. GET /api/admin/bots

List semua bot. **Tidak ada pagination.**

### Response 200
```json
[
  {
    "id": "jessica",
    "name": "Jessica",
    "category_slug": "cewe",
    "description": "... | null",
    "avatar": "Bot/jessica/avatar.jpg | null",
    "background": "... | null",
    "personality": "... | null",
    "chat_examples": "... | null",
    "birthday": "2006-01-02 | null",
    "sbr": "S | null",
    "color": "#FF69B4 | null",
    "child": false,
    "created_at": "2026-01-01T00:00:00Z",
    "updated_at": "2026-01-01T00:00:00Z"
  }
]
```

> **Catatan:** Field `background` di JSON = field `jailbreak` di database (frontend mapping).

---

## 8. POST /api/admin/bots

### Request
```json
{
  "bot_id": "jessica",
  "name": "Jessica",
  "category_slug": "cewe",
  "description": "Seorang siswi SMA",
  "avatar": "Bot/jessica/avatar.jpg",
  "background": "Kamu adalah Jessica...",
  "personality": "Ceria, ramah",
  "chat_examples": "S: Halo | A: Hai... || ...",
  "birthday": "2006-01-02",
  "sbr": "S",
  "color": "#FF69B4",
  "child": false
}
```

### Response 200
```json
{
  "message": "Bot created successfully",
  "bot": {
    "id": "jessica",
    "...": "..."
  }
}
```

### Error
- `422` — bot_id dan name wajib diisi
- `409` — bot_id sudah ada

---

## 9. POST /api/admin/bots/import

Import CSV. **Multipart form-data.**

### Request
```
Content-Type: multipart/form-data
Field: csv_file (file .csv)
```

### CSV Format
```csv
bot_id,name,category_slug,description,avatar,background,personality,chat_examples,birthday,sbr,color,child
jessica,Jessica,cewe,Seorang siswi SMA,https://...,Kamu adalah...,Ceria,...,2006-01-02,S,#FF69B4,false
```

Header case-insensitive. `bot_id` dan `name` wajib ada. Upsert: jika bot_id sudah ada, field diupdate.

### Response 200
```json
{
  "message": "10 bots imported successfully",
  "imported": 10
}
```

### Error
- `422` — file csv_file wajib diupload dan harus .csv
- `400` — header CSV tidak valid, kolom bot_id/name tidak ditemukan

---

## 10. PUT /api/admin/bots/:id

Update bot. `:id` adalah `bot_id` (string). **Partial update** — hanya field yang dikirim yang diupdate.

### Request (semua field optional)
```json
{
  "bot_id": "jessica-baru",
  "name": "Jessica Baru",
  "category_slug": "cewe",
  "description": "...",
  "avatar": "...",
  "background": "...",
  "personality": "...",
  "chat_examples": "...",
  "birthday": "2006-01-02",
  "sbr": "S",
  "color": "#FF69B4",
  "child": true
}
```

### Response 200
```json
{
  "message": "Bot updated successfully"
}
```

### Error
- `404` — bot tidak ditemukan
- `409` — bot_id sudah dipakai bot lain
- `422` — nama kosong, tidak ada field yang diupdate, format tidak valid

---

## 11. DELETE /api/admin/bots/:id

### Request
Body kosong.

### Response 200
```json
{
  "message": "Bot deleted successfully"
}
```

### Error
- `404` — bot tidak ditemukan

---

## 12. POST /api/admin/bots/bulk-assign-category

Pindahkan banyak bot ke satu kategori.

### Request
```json
{
  "bot_ids": ["jessica", "akeyla", "luna"],
  "category_slug": "cewe"
}
```

### Response 200
```json
{
  "message": "3 bots assigned to category",
  "updated": 3
}
```

### Error
- `422` — bot_ids wajib diisi, category_slug wajib diisi
- `404` — kategori tidak ditemukan

---

## 13. POST /api/admin/bots/bulk-store

Tambah banyak bot sekaligus.

### Request
```json
{
  "entries": [
    {
      "bot_id": "jessica",
      "name": "Jessica",
      "category_slug": "cewe",
      "description": "...",
      "avatar": "...",
      "background": "...",
      "personality": "...",
      "chat_examples": "...",
      "birthday": "2006-01-02",
      "sbr": "S",
      "color": "#FF69B4",
      "child": false
    }
  ]
}
```

Semua field opsional kecuali `bot_id` dan `name`.

### Response 200
```json
{
  "message": "5 bots imported successfully",
  "imported": 5,
  "skipped": 1
}
```

### Error
- `422` — entries wajib diisi

---

## 14. GET /api/admin/categories

List semua kategori dengan jumlah bot per kategori.

### Response 200
```json
[
  {
    "id": 1,
    "name": "Cewe",
    "slug": "cewe",
    "logo_url": "Category/cewe/logo.png | null",
    "pinned": false,
    "featured": true,
    "bot_count": 15,
    "created_at": "2026-01-01T00:00:00Z",
    "updated_at": "2026-01-01T00:00:00Z"
  }
]
```

---

## 15. POST /api/admin/categories

### Request
```json
{
  "name": "Cewe",
  "slug": "cewe",
  "logo_url": "Category/cewe/logo.png",
  "pinned": false,
  "featured": true
}
```
Semua field opsional. `slug` auto-generated dari `name` jika tidak diisi.

### Response 200
```json
{
  "message": "Category created successfully",
  "category": {
    "id": 1,
    "name": "Cewe",
    "slug": "cewe",
    "...": "..."
  }
}
```

### Error
- `422` — nama wajib diisi, pinned dan featured tidak bisa bersamaan
- `409` — slug sudah ada

---

## 16. PUT /api/admin/categories/:id

**Partial update.** `:id` adalah numeric ID kategori.

### Request (semua field optional)
```json
{
  "name": "Cewe Updated",
  "slug": "cewe-updated",
  "logo_url": "Category/cewe/logo.png",
  "pinned": true,
  "featured": false
}
```

### Response 200
```json
{
  "message": "Category updated successfully"
}
```

### Error
- `404` — kategori tidak ditemukan
- `422` — pinned dan featured tidak bisa bersamaan
- `409` — slug sudah dipakai kategori lain

---

## 17. DELETE /api/admin/categories/:id

Hapus kategori. Semua bot dalam kategori ini akan di-set `category_slug = null`.

### Response 200
```json
{
  "message": "Category deleted successfully"
}
```

### Error
- `404` — kategori tidak ditemukan

---

## 18. GET /api/admin/settings

### Response 200
```json
{
  "site_name": "Halo Story",
  "maintenance_mode": "false",
  "app_version": "1.0.0"
}
```
Flat object: key → value (string).

---

## 19. PUT /api/admin/settings

**Upsert.** Simpan key-value pairs. Nilai bool dikonversi ke string `"true"`/`"false"`.

### Request
```json
{
  "site_name": "Halo Story",
  "maintenance_mode": false,
  "app_version": "1.0.1"
}
```

### Response 200
```json
{
  "message": "Settings updated successfully"
}
```

---

## 20. GET /api/admin/stats

### Response 200
```json
{
  "hourly_users": 42,
  "daily_users": 356,
  "total_users": 15234,
  "total_bots": 89
}
```

- `hourly_users`: distinct user_id di user_activities dalam 1 jam terakhir
- `daily_users`: distinct user_id dalam 24 jam terakhir
- `total_users`: total user
- `total_bots`: total bot

---

## 21. GET /api/admin/stats/charts

Data grafik untuk dashboard. Mengelompokkan `user_activities` per jam/hari/minggu/bulan.

### Response 200
```json
{
  "hourly": [
    { "label": "2026-07-21 14:00", "value": 5 },
    { "label": "2026-07-21 15:00", "value": 12 }
  ],
  "daily": [
    { "label": "2026-07-15", "value": 45 },
    { "label": "2026-07-16", "value": 52 }
  ],
  "weekly": [
    { "label": "2026-07-14", "value": 120 },
    { "label": "2026-07-21", "value": 98 }
  ],
  "monthly": [
    { "label": "2026-01", "value": 450 },
    { "label": "2026-02", "value": 520 }
  ]
}
```

- `hourly`: 24 titik data terakhir (per jam)
- `daily`: 7 hari terakhir
- `weekly`: 4 minggu terakhir (grouped by iso week)
- `monthly`: 12 bulan terakhir

---

## 22. GET /api/admin/student-requests

List request murid. **Pending first**, lalu diurutkan dari terbaru.

### Response 200
```json
[
  {
    "id": 1,
    "student_name": "Budi",
    "status": "pending",
    "user_name": "andi",
    "user_id": 1,
    "created_at": "2026-01-01 15:04"
  }
]
```

---

## 23. PUT /api/admin/student-requests/:id/status

### Request
```json
{
  "status": "pending" | "approved" | "rejected"
}
```

Ketika status berubah dari `"pending"` ke `"approved"` atau `"rejected"`, mengirim notifikasi Discord.

### Response 200
```json
{
  "message": "Status updated successfully"
}
```

### Error
- `422` — status harus `"pending"`, `"approved"`, atau `"rejected"`
- `404` — request tidak ditemukan

---

## Auth Flow

1. **Login:** `POST /api/login` dengan `Authorization: Bearer <API_STATIC_TOKEN>`
   ```json
   {
     "email": "admin@example.com",
     "password": "password"
   }
   ```
   Response: `{"token": "jwt_token_here", "user": {...}}`

2. **Gunakan token** di setiap request:
   ```
   Authorization: Bearer <jwt_token_here>
   ```

3. **Cek admin:** Setelah login, user harus memiliki `role: "admin"`. Jika bukan admin, semua `/api/admin/*` return 403.

---

## Error Response Format

Semua endpoint mengembalikan error dalam format:
```json
{
  "message": "Deskripsi error"
}
```

Kode error umum:
| Status | Arti |
|--------|------|
| 400 | Bad request (self-deletion, CSV error) |
| 401 | Token tidak valid / expired |
| 403 | Bukan admin / kena banned |
| 404 | Resource tidak ditemukan |
| 409 | Duplicate / konflik |
| 422 | Validasi gagal |
| 500 | Internal server error |
