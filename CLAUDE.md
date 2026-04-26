# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project name: probakgo

The Go implementation is named **probakgo**. The Python predecessor was "probaky". Any remaining reference to "probaky" in source files is a leftover to be cleaned up.

### Repository layout
```
main.go          - server binary: API + web UI on one port (36748)
client/          - client binary: runs on Proxmox nodes
go.mod           - module probakgo, Go 1.26, modernc/sqlite (CGO-free)
internal/
  api/           - REST API (chi router), prefix /api/
  web/           - Web UI (chi router, gorilla/sessions)
  service/       - auth, report, email services
  store/         - SQLite queries
  db/            - embedded migrations (001_initial.up.sql)
  session/       - session helpers (extracted to avoid import cycles)
  domain/        - shared domain models + API schemas
  config/        - env-based config
web/
  templates/     - Go html/template files (embedded)
  static/        - CSS/JS (embedded)
```

### Building

```bash
# Server (API + web UI)
go build -o probakgo .
./probakgo          # reads .env, listens on port 36748

# Client (Proxmox monitoring agent)
go build -o probakgo-client ./client/
```

### What's implemented

**Server:**
- API: all endpoints (health, auth, PVE/PBS reports, backup config, admin API keys, download)
- Web UI: dashboard, PVE servers + detail + historical reports, PBS servers, API keys, users, email settings, profile
- Email: daily report scheduler (configurable time via DB, SMTP with STARTTLS)
- DB: SQLite with embedded migrations
- Auth: bcrypt passwords, session cookies (gorilla/sessions)
- Roles: 3-tier RBAC - `reader` (read-only), `editor` (backup config), `admin` (full access)
- API key types: `pbk-` (client), `app-` (mobile), `adm-` (admin)

**Client:**
- Detects server type (PVE/PBS) from `/etc/issue`
- PVE: queries storages, backup tasks, content; sends to `POST /report/pve`
- PBS: queries datastore usage; sends to `POST /report/pbs`
- Machine ID binding via `/etc/machine-id`
- TLS: configurable verify/skip/CA bundle via env vars
- Subcommands: `install`, `update`, `version` (report mode is default, `--vzdump-hook` flag)
- File mode: `--file path.json` for testing without a live Proxmox node

**Self-update (2026-04):**
- Server: `main.go` handles `update` subcommand via `selfupdate.Run("Nestorm18/probakgo", "probakgo", version)`. On first startup as root, writes `/etc/cron.d/probakgo` (daily at 01:00). After update calls `systemctl restart probakgo`.
- Client: `client/main.go` handles `update` subcommand via `selfupdate.Run("Nestorm18/probakgo", "probakgo-client", version)`. `install` subcommand writes `/etc/cron.d/probakgo-client` (daily at 01:00).
- `var version` (not `const`) required for `-ldflags "-X main.version=..."` injection at release build time.
- Note: GitHub API returns 404 for unauthenticated requests on private repos — selfupdate requires the repo to be public.

### SQLite nullable columns (2026-04)
All nullable TEXT columns in the DB (`stale_reason`, etc.) must be scanned into `sql.NullString`, not `string`. Scanning NULL into `string` silently returns an error in modernc/sqlite, which causes the query to return `nil` - breaking any downstream logic that expects a result. Pattern:
```go
var staleReason sql.NullString
row.Scan(..., &staleReason, ...)
r.StaleReason = staleReason.String
```

### Template functions (web/handlers/templates.go)
Registered in `makeFuncMap()`: `formatTime`, `formatBytes`, `pct`, `formatDuration`, `formatUnixTime`, `isAdmin`, `canEdit`, `keyPreview`. Add new helpers there, not inline in templates.

### Import cycle note (2026-04)
Session code lives in `internal/session` (not `internal/web`) to avoid:
`internal/web` → `internal/web/handlers` → `internal/web` cycle.

### Settings pages (2026-04)
Email, Mantenimiento y Alertas son páginas separadas bajo `/settings/`:
- `/settings/email` — SMTP, destinatarios, hora de envío
- `/settings/maintenance` — retención de reportes (meses + toggle)
- `/settings/alerts` — umbral de disco (%) + alerta de backup fallido
Cada POST carga el config existente y sobreescribe solo sus campos.

### Testing strategy (2026-04)
Tests de store usan `internal/store/testhelpers_test.go` → `openTestDB(t)`:
abre SQLite `:memory:` con `dbpkg.Open(":memory:")` (aplica migraciones reales).
Los tests son `package store` (whitebox) para acceder a `s.db` en backdates de timestamps.
Para tests deterministas de "devuelve el más reciente": insertar el report antiguo primero,
backdatearlo con `UPDATE … SET reported_at = ?`, luego insertar el report nuevo.

---

## Project Overview

**probakgo** monitors Proxmox Virtual Environment (PVE) and Proxmox Backup Server (PBS) nodes and provides a web dashboard with backup status, storage usage, and email reports.

### Component interaction

```
probakgo-client  ──POST /report/pve──▶  probakgo (server)
probakgo-client  ──POST /report/pbs──▶  probakgo (server)
                                            │
                                       SQLite DB
                                            │
                               Web UI (browser ← port 36748)
```

### Client installation on a Proxmox node

```bash
# 1. Build and copy binary to the node
go build -o probakgo-client ./client/
scp probakgo-client root@proxmox-node:/tmp/

# 2. On the node - the binary installs itself
ssh root@proxmox-node "/tmp/probakgo-client install --api-url http://your-server:36748 --api-key pbk-..."

# 3. Verify
ssh root@proxmox-node "/opt/probakgo/probakgo-client"
```

The `install` subcommand:
- Copies itself to `/opt/probakgo/`
- Auto-generates Proxmox API token via `pveum` (PVE) or `proxmox-backup-manager` (PBS)
- Writes `/opt/probakgo/.env`
- Generates and installs vzdump hook script in `/etc/vzdump.conf`
- Configures logrotate
- Installs `/etc/cron.d/probakgo-client` for daily self-update at 01:00

**Updates**: `probakgo-client update` or automatic via cron. No service restart needed — the client runs per-backup, not as a daemon.

### Client configuration (`/opt/probakgo/.env`)

```env
API_KEY=pbk-...                         # from probakgo web UI
API_URL=http://your-server:36748
PROXMOX_TOKEN=root@pam!probakgo-client   # auto-generated by install
PROXMOX_SECRET=xxxxxxxx-xxxx-...
PROXMOX_VERIFY_TLS=false                # false for self-signed certs (default on Proxmox)
# PROXMOX_CA_BUNDLE=/path/to/ca.pem    # optional: custom CA
```

### Server configuration (`.env`)

```env
API_HOST=0.0.0.0
API_PORT=36748
DATABASE_PATH=./probakgo_data.db
SESSION_KEY=32-byte-secret-key
TIMEZONE=Europe/Madrid
```

Email (SMTP, recipients, send time) is stored in the DB and configured via the web UI. No env vars needed for email.

### User roles

| Role | Access |
|------|--------|
| `reader` | Read-only: dashboard, servers, backups |
| `editor` | reader + backup config editing |
| `admin` | Full access including users, API keys, email settings |

Default login: `probakgo` / `admin123` - change immediately.

### DB migrations

Embedded in `internal/db/migrations/`. Applied automatically on server startup via `schema_migrations` table. Currently: `001_initial.up.sql` (all schema including roles).

### Test fixtures (`testdata/`)

- `seed.sh` - envía un reporte PVE y uno PBS al servidor vía API (requiere clave `pbk-` activa)
- `seed_history.go` - inserta 6 días de reportes históricos directamente en la BD SQLite (`go run testdata/seed_history.go`); útil para probar el gráfico de duración y la vista de historial
- Ejecutar siempre `seed.sh` primero (crea los servidores), luego `seed_history.go`
