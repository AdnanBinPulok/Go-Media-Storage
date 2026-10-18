# Go Media Storage

A self-hosted media storage server built with Go. Upload, organise, and serve files through a vault-based access model with a built-in dashboard, REST API, image compression, and deterministic avatars.

---

## Features

- **Vault-based storage** — each vault has an isolated API key and folder; per-vault storage byte limits
- **File upload** — multipart upload via API (vault API key) or browser dashboard (session auth); up to 1 GB per request
- **MIME & magic-byte detection** — uses `gabriel-vasile/mimetype` to detect real file type regardless of extension; blocks dangerous types (HTML, JS, PHP, executables, SVG, etc.)
- **Image compression** — resize and re-encode JPEG, PNG, and GIF (animated) files; atomic file replacement; compression metadata stored in DB
- **Generated avatars** — deterministic PNG avatars derived from any UUID (8–4096 px), with long-lived browser/CDN caching and an in-process render cache
- **User accounts** — registration with bcrypt passwords; optional email whitelist; 30-day session cookies
- **Dashboard** — browser UI for uploads, vault management, file browsing, and compression
- **Public status page** — live service health, runtime metrics, and endpoint listing via `GET /api/status` (cached, public-safe)
- **Rate limiting** — in-process per-IP rate limiter on public routes
- **Error pages** — custom 404, 429, and 500 HTML pages
- **pm2 deployment** — `pm2_start.sh` / `update.sh` scripts for Linux production deploys

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.25.1+ |
| HTTP framework | [Fiber v2](https://github.com/gofiber/fiber) |
| Database | PostgreSQL via [pggo](https://github.com/AdnanBinPulok/pggo) |
| Templates | Go HTML templates (server-side, Fiber template engine) |
| Styling | Tailwind CSS (CDN) |
| Image processing | `golang.org/x/image` (Catmull-Rom scaling, Floyd-Steinberg dithering) |
| Avatar rendering | In-house deterministic generator (`avatar_generator/`), `klauspost/compress` for PNG deflate |
| Password hashing | bcrypt (`golang.org/x/crypto`) |
| MIME detection | `gabriel-vasile/mimetype` |
| In-process cache | `patrickmn/go-cache` |

---

## Project Structure

```
.
├── main.go                     # Entry point — wires Fiber, DB, routes, pages
├── go.mod / go.sum
├── pm2_start.sh                # Build + pm2 start (Linux/macOS; Go 1.25.1+)
├── update.sh                   # git pull + rebuild + pm2 restart
│
├── settings/
│   └── config.go               # Env-based config (AppInfo, DatabaseConfig, FileStorageConfig, CachingConfig, AvatarConfig)
│
├── services/
│   ├── database.go             # PostgreSQL init, table creation, connection singleton
│   └── logging.go              # Log level / prefix setup
│
├── database_tables/
│   ├── users.go                # UserModel + pggo table definition
│   ├── sessions.go             # SessionModel
│   ├── vaults.go               # VaultModel (api_key, storage limits, stats)
│   └── media_files.go          # MediaFileModel (compression columns, JSONB metadata)
│
├── modules/
│   ├── data.go                 # site.json + whitelist.json read/write with cache
│   ├── detector.go             # MIME detection + FileTypeFromMimeType grouping
│   ├── encryptions.go          # bcrypt hash / verify / strength check
│   ├── users.go                # FetchUserData (session cookie → user map)
│   ├── vault.go                # FetchVaultData (Bearer token → vault map)
│   └── status.go               # GetStatusData / GetPublicStatusData (live metrics, 30s cache)
│
├── classes/
│   ├── FileManager.go          # Path-safe storage root operations (resolve, write, delete, stat)
│   ├── FileScanner.go          # Upload safety scanner (MIME blocklist, extension blocklist, magic bytes)
│   ├── SessionManager.go       # Session creation, cookie writing, token generation
│   ├── UploadManager.go        # File upload orchestration + UpdateVaultMediaStats
│   └── Compressor.go           # Image compression (JPEG, PNG, GIF animated), atomic write, DB update
│
├── avatar_generator/           # Deterministic avatar renderer (no external assets or fonts)
│   ├── spec.go                 # Seeded spec: shapes, faces, eyes, palette, clamps
│   ├── shape.go                # Shape math, LUT, analytic antialiasing, blending
│   ├── eyes.go                 # 6 eye styles
│   ├── color.go                # HSL conversion + contrast pick
│   ├── generator.go            # GeneratePNG, canvas pool, draw pipeline, ETag
│   ├── pngfast.go              # Fixed-filter PNG writer
│   ├── parallel.go             # Row-band fan-out for draw + encode
│   └── cache.go                # TTL + LRU cache with per-key singleflight
│
├── routes/                     # API route handlers (all mounted under /api)
│   ├── ping.go                 # GET  /api/ping
│   ├── ip.go                   # GET  /api/ip
│   ├── status.go               # GET  /api/status  (public, cached)
│   ├── signup.go               # POST /api/signup
│   ├── login.go                # POST /api/login
│   ├── upload.go               # POST /api/upload  (vault API key)
│   ├── frontend_upload.go      # POST /api/frontend/upload  (session auth)
│   ├── uploads.go              # GET  /api/uploads/recent  (session auth)
│   ├── delete.go               # DELETE /api/delete
│   ├── vaults.go               # GET  /api/vaults
│   ├── vaults_add.go           # POST /api/vaults/add
│   ├── vaults_edit.go          # POST /api/vaults/edit
│   ├── vaults_delete.go        # POST /api/vaults/delete
│   ├── dashboard.go            # Dashboard data endpoints
│   └── compress.go             # POST /api/compressor/start  (vault API key)
│                               # POST /api/frontend/compressor/start  (session auth)
│
├── pages/                      # SSR page routes (mounted under /)
│   ├── home.go                 # GET /
│   ├── login.go                # GET /login
│   ├── signup.go               # GET /signup
│   ├── status.go               # GET /status
│   ├── dashboard.go            # GET /dashboard, /dashboard/upload, /dashboard/vaults, /dashboard/compress
│   ├── avatar.go               # GET /avatar/:uuid  (generated PNG, public)
│   ├── avatar_store.go         # Shared avatar cache + size helpers
│   ├── dashboard_avatar.go     # GET /dashboard/avatar  (avatar playground)
│   ├── file.go                 # GET /file/:stored_name
│   └── theme.go                # GET /theme
│
├── utils/
│   ├── render.go               # RenderPage helper
│   ├── ratelimiter.go          # Per-IP in-process rate limiter middleware
│   ├── uuid.go                 # NormalizeUUID (canonical, 32-hex, braced, urn forms)
│   └── error_handler.go        # Global Fiber error handler (JSON for /api, HTML otherwise)
│
├── static/
│   ├── pages/                  # Top-level HTML page shells (rendered by Fiber template engine)
│   ├── components/             # Reusable Go template partials
│   │   ├── common/             # Header, footer, nav
│   │   ├── dashboard/          # Dashboard sidebar + header
│   │   ├── pages/              # Per-page component trees
│   │   └── utils/              # Tailwind config, notification, popups
│   ├── styles/                 # main.css, dashboard_main.css
│   └── errors/                 # 404.html, 429.html, 500.html
│
├── assets/
│   └── images/                 # Static assets (favicon, etc.)
│
├── data/
│   ├── site_example.json       # Copy to site.json and customise
│   └── whitelist_example.json  # Copy to whitelist.json and add allowed emails
│
└── secrets/                    # Place your .env file here (git-ignored)
```

---

## Quick Start

Three ways to run the project — pick the one that fits your setup.

---

### Option A — Docker Compose (recommended)

Requires: [Docker](https://docs.docker.com/get-docker/) + [Docker Compose](https://docs.docker.com/compose/)

```bash
# 1. Clone
git clone https://github.com/your-org/go-media-storage.git
cd go-media-storage

# 2. Create your env file (docker compose reads ./.env for variable interpolation)
cp secrets/.env_example .env
# Edit .env — at minimum set POSTGRES_PASSWORD and BASE_URL

# 3. Prepare data files
cp data/site_example.json data/site.json
cp data/whitelist_example.json data/whitelist.json

# 4. Start (builds image + starts postgres)
docker compose up -d

# 5. Tail logs
docker compose logs -f app
```

The app is now available at `http://localhost:8432`.

**Useful commands:**
```bash
docker compose down          # stop & remove containers
docker compose down -v       # also remove volumes (wipes DB + storage)
docker compose build --no-cache app   # force image rebuild
docker compose restart app   # restart app after config change
```

---

### Option B — Docker only (bring your own PostgreSQL)

```bash
docker build -t go-media-storage .

docker run -d \
  --name go-media-storage \
  -p 8432:8432 \
  -e POSTGRES_HOST=host.docker.internal \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=changeme \
  -e POSTGRES_DB=go_media_storage \
  -e BASE_URL=http://localhost:8432 \
  -v "$(pwd)/storage:/app/storage" \
  -v "$(pwd)/data:/app/data" \
  go-media-storage
```

---

### Option C — Native Go (local development)

#### Prerequisites

- **Go 1.25.1 or newer** (install from https://go.dev/dl/ — do **not** use `apt install golang-go`; Ubuntu’s package is often 1.18 and cannot build this project)
- PostgreSQL 14+
- (Production) [pm2](https://pm2.keymetrics.io/) for process management

**VPS / Ubuntu install (official Go):**

```bash
# Remove old apt Go from PATH if needed, then:
curl -fsSL -o /tmp/go.tgz https://go.dev/dl/go1.25.1.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf /tmp/go.tgz
echo 'export PATH=/usr/local/go/bin:$PATH' >> ~/.bashrc
source ~/.bashrc
go version   # should print go1.25.1
```

(Use `go1.25.1.linux-arm64.tar.gz` on ARM servers.)


### 1. Clone

```bash
git clone https://github.com/AdnanBinPulok/Go-Media-Storage.git
cd Go-Media-Storage
```

### 2. Configuration

Create `secrets/.env`:

```env
# Application
APP_HOST=0.0.0.0
APP_PORT=8432
BASE_URL=http://localhost:8432
LOG_LEVEL=info

# Database
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_password
POSTGRES_DB=go_media_storage
POSTGRES_SSL_MODE=disable
DATABASE_MAX_OPEN_CONNS=25
DATABASE_MAX_IDLE_CONNS=25

# File storage
FILE_STORAGE_LOCAL_BASE_PATH=./storage
FILE_STORAGE_LIMIT=100GB

# Cache
BROWSER_CACHE_TIME_SECONDS=30
PROXY_CACHE_TIME_SECONDS=60
AVATAR_CACHE_TIME_SECONDS=86400
AVATAR_DEFAULT_SIZE=256
AVATAR_MEMORY_TTL_SECONDS=3600
```

Create site metadata:

```bash
cp data/site_example.json data/site.json
# Edit data/site.json with your project name, favicon path, version, etc.
```

Optionally restrict signups to specific emails:

```bash
cp data/whitelist_example.json data/whitelist.json
# Add allowed email addresses to the array, or leave it empty to allow all
```

### 3. Run (development)

```bash
go mod download
go run main.go
```

### 4. Build & run (production — Linux)

```bash
chmod +x pm2_start.sh update.sh
./pm2_start.sh
```

`pm2_start.sh` builds the binary and starts it under pm2 as `GO_MEDIA_STORAGE`. Your VPS must already have Go 1.25.1+ on `PATH`. For future updates:

```bash
./update.sh   # git pull + rebuild + pm2 restart
```

---

## API Reference

All API routes are prefixed with `/api`.

### Authentication

| Type | How |
|---|---|
| **Vault API key** | `Authorization: Bearer <api_key>` header |
| **Session** | `session_token` HTTP-only cookie (set on login) |

---

### Health

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/ping` | None | Liveness probe. Returns `{"status":"ok"}` |
| `GET` | `/api/ip` | None | Returns client IP address |
| `GET` | `/api/status` | None | Live system status (30s cached, public-safe) |

**`GET /api/status` response:**
```json
{
  "project_name": "Go Media Storage",
  "version": "2.0.0",
  "generated_at": "2026-05-13T10:51:45Z",
  "cached": false,
  "services": [
    { "name": "postgres", "state": "healthy", "scope": "database", "last_checked": "...", "detail": "..." },
    { "name": "local-storage", "state": "healthy", "scope": "filesystem", "last_checked": "...", "detail": "..." }
  ],
  "runtime": { "uptime": "2h 14m 09s", "go_version": "go1.25.1" },
  "endpoints": [ ... ]
}
```

---

### Auth

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/signup` | None | Register a new user |
| `POST` | `/api/login` | None | Log in, sets `session_token` cookie |

---

### Vaults

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/vaults` | Session | List your vaults |
| `POST` | `/api/vaults/add` | Session | Create a vault |
| `POST` | `/api/vaults/edit` | Session | Edit a vault |
| `POST` | `/api/vaults/delete` | Session | Delete a vault |

---

### Files

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/upload` | Vault API key | Upload one or more files (multipart) |
| `POST` | `/api/frontend/upload` | Session | Upload files via browser dashboard |
| `GET` | `/api/uploads/recent` | Session | List 18 most recent uploads |
| `DELETE` | `/api/delete` | Session | Delete a file |

**`POST /api/upload` request:**
```
Content-Type: multipart/form-data
Authorization: Bearer <vault_api_key>

file: <binary>   (repeat for multiple files)
```

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "stored_name": "abc123.jpg",
      "url": "https://your-domain.com/file/abc123.jpg",
      "mime_type": "image/jpeg",
      "file_size_bytes": 204800
    }
  ]
}
```

---

### Image Compression

Compression runs asynchronously. The endpoint returns `201 Created` immediately and processes in the background.

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/compressor/start` | Vault API key | Start compression job |
| `POST` | `/api/frontend/compressor/start` | Session | Start compression from dashboard |

**Request body:**
```json
{
  "file_id": "abc123.jpg",
  "quality": 80,
  "max_width": 1920,
  "lossless": false
}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `file_id` | string | required | `stored_name` or full file URL |
| `quality` | int | `80` | JPEG quality (1–100). No effect on PNG or GIF |
| `max_width` | int | `0` | Resize to this width preserving aspect ratio. `0` = no resize |
| `lossless` | bool | `false` | Convert JPEG/PNG to lossless PNG. GIF is always lossless |

**Supported formats:** JPEG, PNG, GIF (including animated)

---

## Avatars

`GET /avatar/:uuid` returns a deterministic PNG avatar. The same UUID and size always produce the exact same image, so responses are safe to cache forever and to embed anywhere. No upload, account, or API key is required.

Live instance: `https://vault.gliders.dev`

```text
https://vault.gliders.dev/avatar/550e8400-e29b-41d4-a716-446655440000
https://vault.gliders.dev/avatar/550e8400-e29b-41d4-a716-446655440000?size=512
```

| Parameter | Default | Description |
|---|---|---|
| `uuid` (path) | required | Any UUID spelling: canonical, uppercase, 32-hex, braced (`{...}`), or `urn:uuid:`. All forms normalize to one lowercase canonical UUID, so every spelling maps to the same avatar and the same cache entry |
| `size` (query) | `AVATAR_DEFAULT_SIZE` (`256`) | Square edge length in px, clamped to `8`–`4096`. Omit it to get the configured default |

### Embedding

```html
<!-- Default size -->
<img src="https://vault.gliders.dev/avatar/550e8400-e29b-41d4-a716-446655440000" alt="avatar" width="64" height="64">

<!-- Explicit size -->
<img src="https://vault.gliders.dev/avatar/550e8400-e29b-41d4-a716-446655440000?size=128" alt="avatar"
     width="128" height="128" loading="lazy">
```

```markdown
![avatar](https://vault.gliders.dev/avatar/550e8400-e29b-41d4-a716-446655440000?size=128)
```

```css
.avatar { background-image: url("https://vault.gliders.dev/avatar/550e8400-e29b-41d4-a716-446655440000?size=96"); }
```

Any stable identifier can be turned into a UUID — hash a username or email (for example, a UUIDv5 of the address) and use the result as the path. Do not put a raw email or username in the URL unless you are comfortable with it appearing in logs.

Download the PNG directly (the response is `image/png` with a transparent background):

```bash
curl -o avatar.png "https://vault.gliders.dev/avatar/550e8400-e29b-41d4-a716-446655440000?size=512"
```

### Responses and caching

| Detail | Value |
|---|---|
| Content type | `image/png`, transparent background, 5% padding |
| `Cache-Control` | `public, max-age={AVATAR_CACHE_TIME_SECONDS}, s-maxage=…, immutable` (1 day by default) |
| CDN headers | `CDN-Cache-Control`, `Cloudflare-CDN-Cache-Control` — Cloudflare honours these directly |
| `ETag` | Strong validator derived from `(uuid, size)`, stable across restarts |
| Conditional requests | `If-None-Match` returns `304 Not Modified` |
| `X-Avatar-Cache` | `HIT` when served from the in-process cache, `MISS` after a fresh render |
| `X-Avatar-Generation-Time` | Origin render time in microseconds |

Status codes: `200` on success, `400` for an invalid UUID or non-numeric `size`, `429` when the per-IP rate limit is exceeded, `503` when the large-render gate is saturated (sizes above 1024 px).

### Cloudflare

Avatars are extensionless URLs, so Cloudflare's default static cache ignores them. Add a Cache Rule for `URI Path starts with /avatar/` with **Eligible for cache** and an edge TTL of 1 day. Keep cache level **Standard** so `?size=` remains part of the cache key, and make sure avatar requests are not sent with an `Authorization` header (Cloudflare bypasses the edge cache for those).

### Playground

`/dashboard/avatar` (login required) generates a random UUID, previews the result at any size from 8 to 4096, copies the URL or downloads the PNG, and shows a gallery of sample variants.

---

## Dashboard Pages

| Path | Description |
|---|---|
| `/` | Landing page |
| `/login` | Login |
| `/signup` | Register |
| `/status` | Live status (CSR — fetches `/api/status`) |
| `/dashboard` | Overview — stats, recent uploads |
| `/dashboard/upload` | Upload files, view recent uploads |
| `/dashboard/vaults` | Manage vaults |
| `/dashboard/avatar` | Avatar playground — random UUID, size 8–4096, preview, copy URL, download |
| `/dashboard/compress?file_url=...` | Compress an image |

---

## Security Notes

- Uploaded files are scanned by magic bytes before storage; dangerous MIME types and extensions (HTML, JS, PHP, EXE, etc.) are rejected
- Session tokens are HTTP-only cookies; sessions expire after 30 days
- Vault API keys are validated on every request; no caching of auth state
- File paths are validated against the storage root to prevent path traversal
- Passwords are hashed with bcrypt (default cost)
- The public `/api/status` endpoint omits: environment name, goroutine count, total storage size, and per-type file counts
- Rate limiting is applied on public routes (configurable `Requests`/`Window`)
- Atomic file writes (temp file + rename) prevent partial/corrupt writes
- `/avatar/:uuid` renders from a hash of the UUID only — no database read, no filesystem access, and nothing about a user is exposed beyond the identifier you put in the URL

---

## Configuration Reference

| Variable | Default | Description |
|---|---|---|
| `APP_HOST` | `0.0.0.0` | Bind address |
| `APP_PORT` | `8432` | Bind port |
| `BASE_URL` | `http://localhost:8432` | Public base URL (used for generating file URLs) |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `POSTGRES_HOST` | `127.0.0.1` | PostgreSQL host |
| `POSTGRES_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_USER` | `postgres` | PostgreSQL user |
| `POSTGRES_PASSWORD` | `postgres` | PostgreSQL password |
| `POSTGRES_DB` | `go_media_storage` | Database name |
| `POSTGRES_SSL_MODE` | `disable` | `disable` / `require` / `verify-full` |
| `DATABASE_DSN` | — | Full DSN; overrides the individual `POSTGRES_*` values |
| `DATABASE_MAX_OPEN_CONNS` | `25` | Max open DB connections |
| `DATABASE_MAX_IDLE_CONNS` | `10` | Max idle DB connections |
| `FILE_STORAGE_LOCAL_BASE_PATH` | `./storage` | Where uploaded files are stored |
| `FILE_STORAGE_LIMIT` | `0` | Global storage cap (`0` = unlimited). Accepts `500MB`, `2GB`, etc. |
| `BROWSER_CACHE_TIME_SECONDS` | `30` | Cache-Control max-age for static assets |
| `PROXY_CACHE_TIME_SECONDS` | `60` | Proxy cache TTL |
| `AVATAR_CACHE_TIME_SECONDS` | `86400` | `max-age` / `s-maxage` / CDN TTL for `/avatar/:uuid` |
| `AVATAR_DEFAULT_SIZE` | `256` | Edge length used when `?size` is omitted (clamped to 8–4096) |
| `AVATAR_MEMORY_TTL_SECONDS` | `3600` | In-process avatar render cache TTL (also bounded by 10 000 items / 256 MiB) |

---

## License

MIT — see [LICENSE](LICENSE) for details.
