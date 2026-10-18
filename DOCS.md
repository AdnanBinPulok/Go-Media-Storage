# Go Media Storage — Documentation

Technical reference for operators and developers. For installation and quick start, see [README.md](README.md).

---

## Table of contents

1. [Overview](#overview)
2. [Core concepts](#core-concepts)
3. [Architecture](#architecture)
4. [Data model](#data-model)
5. [Authentication](#authentication)
6. [File storage & delivery](#file-storage--delivery)
7. [API reference](#api-reference)
8. [Web UI routes](#web-ui-routes)
9. [Configuration](#configuration)
10. [Data files](#data-files)
11. [Security](#security)
12. [Development guide](#development-guide)

---

## Overview

Go Media Storage is a self-hosted media server. Users register, create **vaults** (isolated storage buckets with API keys), upload files, and serve them over HTTP. External apps integrate via vault API keys; the dashboard uses session cookies.

| Layer | Path prefix | Purpose |
|-------|-------------|---------|
| Pages | `/` | Server-rendered HTML (Fiber + Go templates) |
| API | `/api` | JSON REST endpoints |
| Static | `/assets`, `/styles` | Favicon, CSS |
| Files | `/file/view/…`, `/file/download/…` | Public file delivery |

Default listen address: `0.0.0.0:8432`. Max upload body: **1 GB** per request.

---

## Core concepts

### Users

- Registered via `POST /api/signup` (email must be on the whitelist).
- Passwords hashed with **bcrypt**; minimum length **8** characters.
- Default limits (database defaults): **1 vault**, **1 GB** total storage quota (`storage_bytes_limit`).
- Sessions last **30 days** (`session_token` HTTP-only cookie).

### Vaults

Each vault belongs to one user and has:

| Field | Meaning |
|-------|---------|
| `api_key` | Bearer token for programmatic upload/delete/compress |
| `vault_folder_name` | Random folder under the storage root |
| `storage_bytes_limit` | Per-vault cap in bytes (`0` = unlimited) |
| `is_active` | Inactive vaults reject API operations |
| `total_files` / `total_size_bytes` | Cached stats (updated after upload/delete) |

### Media files

Stored on disk under the vault folder; metadata in `media_files`:

- `stored_name` — unique filename on disk (used in URLs)
- `file_type` — grouped type (`image`, `video`, `audio`, `document`, `other`)
- `compressed` — whether image compression has run
- `deleted` — soft delete flag (file removed from disk, row kept)

### Storage limits (three layers)

1. **Global** — `FILE_STORAGE_LIMIT` (e.g. `10GB`); sums all files on disk under the storage root.
2. **User** — `users.storage_bytes_limit`; sum of vault `storage_bytes_limit` values cannot exceed this when creating/editing vaults.
3. **Vault** — `vaults.storage_bytes_limit`; sum of non-deleted `media_files.file_size_bytes` for that vault.

---

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│  Browser /  │────▶│  Fiber app   │────▶│  PostgreSQL     │
│  API client │     │  (main.go)   │     │  (pggo tables)  │
└─────────────┘     └──────┬───────┘     └─────────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │ Local disk   │
                    │ ./storage/   │
                    └──────────────┘
```

**Request flow (API upload):**

1. `Authorization: Bearer <api_key>` → `modules.FetchVaultData`
2. Multipart `file` field(s) parsed
3. Global + vault byte limits checked
4. `FileScanner` validates MIME, extension, magic bytes
5. `UploadManager` writes file + inserts `media_files` row
6. Response includes `view` / `download` URLs

**Request flow (dashboard upload):**

Same as above, but session cookie identifies the user; `vault_id` is sent as form field.

### Package layout

| Package | Role |
|---------|------|
| `settings/` | Env loading (`secrets/.env`) |
| `services/` | DB init, logging |
| `database_tables/` | pggo models + schema sync |
| `modules/` | Site data, auth helpers, MIME detection, status |
| `classes/` | FileManager, UploadManager, Compressor, SessionManager, FileScanner |
| `routes/` | `/api/*` handlers |
| `pages/` | SSR page handlers |
| `utils/` | Render helper, rate limiter, error handler |
| `static/` | HTML templates and CSS |

---

## Data model

### `users`

| Column | Notes |
|--------|-------|
| `email` | Unique, lowercased on signup |
| `vaults_limit` | Max vault count (default `1`) |
| `storage_bytes_limit` | User-wide quota in bytes (default 1 GiB) |
| `password_hash` | bcrypt |

### `sessions`

| Column | Notes |
|--------|-------|
| `session_token` | Random hex, stored in cookie |
| `expires_at` | 30 days from creation |
| `ip_address`, `user_agent` | Audit fields |

### `vaults`

See [Core concepts — Vaults](#vaults).

### `media_files`

| Column | Notes |
|--------|-------|
| `stored_name` | Unique; used in public URLs |
| `storage_path` | Relative path under storage root |
| `compression_metadata` | JSONB (algorithm, sizes, ratio, etc.) |
| `deleted` | Soft delete |

Tables are created/synced at startup via pggo `SetSchemaSync`.

---

## Authentication

### Session (dashboard & browser API)

| Item | Value |
|------|-------|
| Cookie name | `session_token` |
| Set by | `POST /api/login`, `POST /api/signup` |
| Validated by | `modules.FetchUserData` (DB lookup) |
| Flags | `HTTPOnly` |

Unauthenticated dashboard API calls return `401` JSON. Some vault list routes redirect to `/login` with `303`.

### Vault API key (programmatic)

```http
Authorization: Bearer <vault_api_key>
```

Used by:

- `POST /api/upload`
- `POST /api/delete`
- `POST /api/compressor/start`

The key maps to a vault row; `is_active` must be true. Keys are **not** cached — each request hits the database.

---

## File storage & delivery

### On-disk layout

```
{FILE_STORAGE_LOCAL_BASE_PATH}/
  {vault_folder_name}/
    {stored_name}
```

`FileManager` resolves paths and blocks directory traversal.

### Public URLs

Generated as:

| Purpose | Pattern |
|---------|---------|
| Inline view | `{BASE_URL}/file/view/{stored_name}` |
| Download | `{BASE_URL}/file/download/{stored_name}` |

Upload API responses include both under `url.view` and `url.download`.

### Response headers (file routes)

- Dangerous MIME types are served as `application/octet-stream` (XSS mitigation).
- `X-Content-Type-Options: nosniff`
- `Cache-Control: public, max-age={BROWSER_CACHE}, s-maxage={PROXY_CACHE}, immutable`

---

## API reference

Base path: **`/api`**. Unless noted, responses are JSON with `success` boolean.

### Health & status

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/ping` | None | `{"status":"ok"}` |
| `GET` | `/api/ip` | None | Client IP |
| `GET` | `/api/status` | None | Public status (rate limited: 20 req / 10s) |

`GET /api/status` returns a **public-safe** subset (no goroutine count, env mode, storage totals, or file-type breakdown). Internal cache TTL ~5 seconds.

Example shape:

```json
{
  "project_name": "Go Media Storage",
  "version": "1.0.0",
  "generated_at": "2026-06-04T12:00:00Z",
  "cached": false,
  "services": [
    { "name": "postgres", "state": "healthy", "scope": "database", "detail": "..." },
    { "name": "local-storage", "state": "healthy", "scope": "filesystem", "detail": "..." }
  ],
  "runtime": { "uptime": "1h 2m 3s", "go_version": "go1.25.x" },
  "endpoints": [ ... ]
}
```

---

### Auth

| Method | Path | Body (JSON) | Notes |
|--------|------|-------------|-------|
| `POST` | `/api/signup` | `full_name`, `email`, `password` | Whitelist required; auto-login |
| `POST` | `/api/login` | `email`, `password` | Sets `session_token` cookie |

Signup/login fail with `403` if email is not in `data/whitelist.json`.

---

### Vaults (session)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/vaults` | Paginated list |
| `GET` | `/api/vaults/info` | Lightweight `{id, name, is_active}[]` |
| `POST` | `/api/vaults/add` | Create vault |
| `POST` | `/api/vaults/edit` | Update vault |
| `POST` | `/api/vaults/delete` | Delete vault |

**`GET /api/vaults` query params:**

| Param | Default | Description |
|-------|---------|-------------|
| `page` | `1` | Page number |
| `limit` | `10` | Page size |
| `order_by` | `created_at` | Sort column |
| `order` | `desc` | `asc` or `desc` |
| `filter_id` | — | Filter by vault ID |
| `is_active` | — | `true` or `false` |

Response includes `data`, `stats` (`total`, `active`, `inactive`), and `meta` (`page`, `limit`, `total_items`, `total_pages`).

**`POST /api/vaults/add` body:**

```json
{
  "name": "My Vault",
  "storage_bytes_limit": 5368709120
}
```

`storage_bytes_limit` of `0` means no per-vault cap. Creation fails if user vault count ≥ `vaults_limit` or allocated storage would exceed user quota.

**`POST /api/vaults/edit` body:**

```json
{
  "id": 1,
  "name": "Renamed",
  "is_active": true,
  "storage_bytes_limit": 0
}
```

**`POST /api/vaults/delete` body:**

```json
{ "id": 1 }
```

---

### Files

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/upload` | Vault Bearer | Multipart upload |
| `POST` | `/api/frontend-upload` | Session | Multipart + `vault_id` form field |
| `GET` | `/api/uploads/recent` | Session | Last 18 uploads for user |
| `POST` | `/api/delete` | Vault Bearer | Soft-delete by URL |

**Upload (both upload endpoints):**

```
Content-Type: multipart/form-data

file: <binary>          # repeat field name for multiple files
vault_id: <id>            # frontend-upload only
```

**Success response (upload):**

```json
{
  "success": true,
  "message": "Files uploaded successfully",
  "data": [
    {
      "id": 1,
      "stored_name": "abc….jpg",
      "mime_type": "image/jpeg",
      "file_size": 204800,
      "url": {
        "view": "http://localhost:8432/file/view/abc….jpg",
        "download": "http://localhost:8432/file/download/abc….jpg"
      }
    }
  ]
}
```

**`POST /api/delete` body:**

```json
{ "url": "http://localhost:8432/file/view/abc….jpg" }
```

Accepts full URL or `stored_name`. Removes file from disk and sets `deleted = true`.

---

### Dashboard data (session)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/dashboard/stats` | Storage and vault allocation stats |
| `GET` | `/api/dashboard/file_types` | File type breakdown for user |

---

### Image compression

Runs in a **background goroutine**. Endpoint returns **`201 Created`** immediately.

| Method | Path | Auth |
|--------|------|------|
| `POST` | `/api/compressor/start` | Vault Bearer |
| `POST` | `/api/frontend/compressor/start` | Session |

**Request body:**

```json
{
  "file_id": "abc….jpg",
  "quality": 80,
  "max_width": 1920,
  "lossless": false,
  "convert_to_webp": false
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `file_id` | required | `stored_name` or full file URL |
| `quality` | `80` | JPEG quality 1–100 (ignored for PNG/GIF) |
| `max_width` | `0` | Resize width; `0` = no resize |
| `lossless` | `false` | Output lossless PNG (or WebP if built with CGo) |
| `convert_to_webp` | `false` | WebP output when CGo + libwebp available |

**Rules:**

- Only `file_type == "image"`
- Cannot compress twice (`compressed` must be false)
- Vault route scopes by `vault_id`; frontend route scopes by `owner_id`

**Supported input:** JPEG, PNG, GIF (including animated). WebP output requires a CGo build.

---

## Web UI routes

| Path | Auth | Description |
|------|------|-------------|
| `/` | Public | Landing |
| `/login`, `/signup` | Public | Auth forms |
| `/status` | Public | Status page (fetches `/api/status`) |
| `/theme` | Public | Theme preview |
| `/dashboard` | Session | Overview |
| `/dashboard/upload` | Session | Upload UI |
| `/dashboard/vaults` | Session | Vault management |
| `/dashboard/compress` | Session | Compression UI (`?file_url=…`) |
| `/dashboard/avatar` | Session | Avatar playground (random UUID + size) |

Dashboard pages use rate limiting: **50 requests / 5 seconds** per IP.

### Avatar delivery

| Path | Auth | Description |
|------|------|-------------|
| `/avatar/:uuid` | Public | Deterministic PNG avatar (`?size=8..4096`, default 256) |

Only valid UUIDs are accepted (canonical, uppercase, 32-hex, braced, or `urn:uuid:`); all spellings normalize to one lowercase form and therefore one cache entry. Responses are cached in memory (5 min, 10,000 items, 256 MiB) and served with `Cache-Control: public, max-age={AVATAR_CACHE_TIME_SECONDS}, s-maxage=…, immutable` plus `CDN-Cache-Control` / `Cloudflare-CDN-Cache-Control`, a stable `ETag`, and `X-Avatar-Cache: HIT|MISS`.

Cloudflare needs a Cache Rule for `^/avatar/` (no file extension means the default static cache ignores it). Keep cache level **Standard** so `?size=` stays part of the cache key.

Templates live under `static/pages/` and `static/components/`. Rendered via `utils.RenderPage`.

---

## Configuration

Environment is loaded from **`secrets/.env`** (optional; Docker can inject env directly).

### Application

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_HOST` | `0.0.0.0` | Bind host |
| `APP_PORT` | `8432` | Bind port |
| `BASE_URL` | `http://localhost:8432` | Public URL for file links |
| `LOG_LEVEL` | `info` | Log verbosity |

### PostgreSQL

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_HOST` | `127.0.0.1` | DB host |
| `POSTGRES_PORT` | `5432` | DB port |
| `POSTGRES_USER` | `postgres` | DB user |
| `POSTGRES_PASSWORD` | `postgres` | DB password |
| `POSTGRES_DB` | `go_media_storage` | Database name |
| `POSTGRES_SSL_MODE` | `disable` | SSL mode |
| `DATABASE_DSN` | *(built)* | Override full DSN |
| `DATABASE_MAX_OPEN_CONNS` | `25` | Connection pool |
| `DATABASE_MAX_IDLE_CONNS` | `10` | Idle connections |

### Storage & cache

| Variable | Default | Description |
|----------|---------|-------------|
| `FILE_STORAGE_LOCAL_BASE_PATH` | `./storage` | Upload root |
| `FILE_STORAGE_LIMIT` | `0` | Global cap (`0` = unlimited). Supports `500MB`, `2GB`, etc. |
| `BROWSER_CACHE_TIME_SECONDS` | `30` | `max-age` for served files |
| `PROXY_CACHE_TIME_SECONDS` | `60` | `s-maxage` for CDN/proxy |
| `AVATAR_CACHE_TIME_SECONDS` | `86400` | `max-age` / `s-maxage` / CDN TTL for `/avatar/:uuid` |
| `AVATAR_DEFAULT_SIZE` | `256` | Edge length used by `/avatar/:uuid` and the dashboard when `?size` is omitted (clamped to 8–4096) |

### Proxy trust

Fiber is configured with `ProxyHeader: CF-Connecting-IP` and trusted proxies `127.0.0.1`, `::1`, so the client IP is read from Cloudflare only when the request arrives through a local reverse proxy (nginx/Caddy on the same host).

If the app is exposed directly to Cloudflare with no local proxy, the peer address is a Cloudflare edge IP, not `127.0.0.1`, so `CF-Connecting-IP` is ignored and `c.IP()` becomes the edge IP. Every visitor then shares one rate-limit bucket. Add Cloudflare's published ranges to `TrustedProxies`, or terminate TLS in a local reverse proxy.

This matters for the public `/avatar/:uuid` limiter: with an untrusted header the 300 requests / 10s budget applies to the whole edge, not per visitor.

---

## Data files

### `data/site.json`

Copy from `data/site_example.json`. Drives branding on pages and status (`title`, `version`, `favicon`, etc.). Cached ~30 seconds in memory.

### `data/whitelist.json`

Copy from `data/whitelist_example.json`. **Signup and login only succeed for emails listed in this array.**

```json
[
  "user@example.com"
]
```

If the file is missing or empty, **no email can register or log in** (whitelist check returns false). Add every allowed address explicitly.

---

## Security

### Upload scanning

`FileScanner` rejects:

- Dangerous MIME types (HTML, JS, SVG, PHP, shell scripts, XML, etc.)
- Dangerous extensions (`.exe`, `.php`, `.sh`, `.py`, …)
- HTML/script signatures in the first 512 bytes

MIME detection uses `gabriel-vasile/mimetype` (magic bytes), not just the client extension.

### Path safety

All disk paths go through `FileManager.ResolvePath` to prevent traversal outside the storage root.

### Atomic writes

Uploads and compression use temp file + rename to avoid partial files.

### API vs public data

`/api/status` strips sensitive runtime and storage metrics. Full metrics exist only in internal `GetStatusData()` (used if you extend the app).

### Error handling

- `/api/*` errors → JSON
- Page errors → HTML (`static/errors/404.html`, `429.html`, `500.html`)

---

## Development guide

### Run locally

```bash
cp data/site_example.json data/site.json
cp data/whitelist_example.json data/whitelist.json
# Edit whitelist.json with your email
# Create secrets/.env with POSTGRES_* vars

go mod download
go run main.go
```

### Add an API route

1. Create `routes/my_feature.go` with `func MyFeatureRoutes(router fiber.Router)`.
2. Register in `main.go` → `RegisterRoutes` → `routesList`.
3. Document the path in this file and optionally in `modules/status.go` endpoint list.

### Add a page

1. Add handler in `pages/`.
2. Add HTML under `static/pages/` and components under `static/components/`.
3. Register in `RegisterPages` in `main.go`.

### Conventions

- Keep handlers small; put logic in `classes/` or `modules/`.
- Prefer new files over growing large route files (project style).
- JSON bodies use `c.BodyParser`; uploads use multipart `file` key.
- Use `GenerateFileURL(storedName)` for consistent view/download links.

### Production (Linux)

```bash
chmod +x start.sh update.sh pm2_update.sh
./start.sh    # build + pm2
./update.sh   # git pull + rebuild + restart
```

### Docker

```bash
docker compose up -d
docker compose logs -f app
```

See [README.md](README.md) for full Docker and native setup steps.

---

## Related documents

| Document | Contents |
|----------|----------|
| [README.md](README.md) | Features, quick start, high-level API summary |
| [LICENSE](LICENSE) | MIT license |
