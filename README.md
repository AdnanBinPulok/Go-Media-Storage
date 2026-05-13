# Go Media Storage

A self-hosted media storage server built with Go. Upload, organise, and serve files through a vault-based access model with a built-in dashboard, REST API, and image compression.

---

## Features

- **Vault-based storage** — each vault has an isolated API key and folder; per-vault storage byte limits
- **File upload** — multipart upload via API (vault API key) or browser dashboard (session auth); up to 1 GB per request
- **MIME & magic-byte detection** — uses `gabriel-vasile/mimetype` to detect real file type regardless of extension; blocks dangerous types (HTML, JS, PHP, executables, SVG, etc.)
- **Image compression** — resize and re-encode JPEG, PNG, and GIF (animated) files; atomic file replacement; compression metadata stored in DB
- **User accounts** — registration with bcrypt passwords; optional email whitelist; 30-day session cookies
- **Dashboard** — browser UI for uploads, vault management, file browsing, and compression
- **Public status page** — live service health, runtime metrics, and endpoint listing via `GET /api/status` (cached, public-safe)
- **Rate limiting** — in-process per-IP rate limiter on public routes
- **Error pages** — custom 404, 429, and 500 HTML pages
- **pm2 deployment** — `start.sh` / `update.sh` scripts for Linux production deploys

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.25+ |
| HTTP framework | [Fiber v2](https://github.com/gofiber/fiber) |
| Database | PostgreSQL via [pggo](https://github.com/AdnanBinPulok/pggo) |
| Templates | Go HTML templates (server-side, Fiber template engine) |
| Styling | Tailwind CSS (CDN) |
| Image processing | `golang.org/x/image` (Catmull-Rom scaling, Floyd-Steinberg dithering) |
| Password hashing | bcrypt (`golang.org/x/crypto`) |
| MIME detection | `gabriel-vasile/mimetype` |
| In-process cache | `patrickmn/go-cache` |

---

## Project Structure

```
.
├── main.go                     # Entry point — wires Fiber, DB, routes, pages
├── go.mod / go.sum
├── start.sh                    # Build + pm2 start (Linux/macOS)
├── update.sh                   # git pull + rebuild + pm2 restart
│
├── settings/
│   └── config.go               # Env-based config (AppInfo, DatabaseConfig, FileStorageConfig)
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
│   ├── file.go                 # GET /file/:stored_name
│   └── theme.go                # GET /theme
│
├── utils/
│   ├── render.go               # RenderPage helper
│   ├── ratelimiter.go          # Per-IP in-process rate limiter middleware
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

# 2. Create your env file
cp .env.example .env
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

- Go 1.25+
- PostgreSQL 14+
- (Production) [pm2](https://pm2.keymetrics.io/) for process management

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
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=go_media_storage
DB_SSLMODE=disable
DB_MAX_OPEN_CONNS=10
DB_MAX_IDLE_CONNS=5

# File storage
FILE_STORAGE_LOCAL_BASE_PATH=./storage

# Cache
BROWSER_CACHE_TIME_SECONDS=30
PROXY_CACHE_TIME_SECONDS=60
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
chmod +x start.sh update.sh
./start.sh
```

This builds the binary and starts it under pm2 as `GO_MEDIA_STORAGE`. For future updates:

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

---

## Configuration Reference

| Variable | Default | Description |
|---|---|---|
| `APP_HOST` | `0.0.0.0` | Bind address |
| `APP_PORT` | `8432` | Bind port |
| `BASE_URL` | `http://localhost:8432` | Public base URL (used for generating file URLs) |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | — | PostgreSQL user |
| `DB_PASSWORD` | — | PostgreSQL password |
| `DB_NAME` | — | Database name |
| `DB_SSLMODE` | `disable` | `disable` / `require` / `verify-full` |
| `DB_MAX_OPEN_CONNS` | `10` | Max open DB connections |
| `DB_MAX_IDLE_CONNS` | `5` | Max idle DB connections |
| `FILE_STORAGE_LOCAL_BASE_PATH` | `./storage` | Where uploaded files are stored |
| `BROWSER_CACHE_TIME_SECONDS` | `30` | Cache-Control max-age for static assets |
| `PROXY_CACHE_TIME_SECONDS` | `60` | Proxy cache TTL |

---

## License

MIT — see [LICENSE](LICENSE) for details.
