# CLAUDE.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

---

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
- Web UI: dashboard, PVE servers + detail + historical reports, PBS servers + detail (snapshots, estimated fill, mount status), API keys, users, email settings, profile
- Email: daily report scheduler (configurable time via DB, SMTP with STARTTLS)
- DB: SQLite with embedded migrations (001–006; 007 planned for alert config)
- Auth: bcrypt passwords, session cookies (gorilla/sessions)
- Roles: 3-tier RBAC - `reader` (read-only), `editor` (backup config), `admin` (full access)
- API key types: `pbk-` (client), `adm-` (admin)
- Alerts: global thresholds in `email_config` evaluated by `internal/store/alerts.go`; per-server/VM alert config planned (see TODO.md)

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
Registered in `makeFuncMap()`: `formatTime`, `formatBytes`, `pct`, `formatDuration`, `formatUnixTime`, `isPast`, `isAdmin`, `canEdit`, `keyPreview`. Add new helpers there, not inline in templates.

- `formatBytes`: uses SI base 1000 (not 1024), 2 decimal places. e.g. "1.05 GB".
- `isPast(ts int64) bool`: returns true if Unix timestamp is in the past (used for estimated full date).
- Template-to-Active-key mapping lives in `templateActive` map in the same file. Add new templates there.

### Import cycle note (2026-04)
Session code lives in `internal/session` (not `internal/web`) to avoid:
`internal/web` → `internal/web/handlers` → `internal/web` cycle.

### Settings pages (2026-04)
Email, Mantenimiento y Alertas son páginas separadas bajo `/settings/`:
- `/settings/email` — SMTP, destinatarios, hora de envío
- `/settings/maintenance` — retención de reportes (meses + toggle)
- `/settings/alerts` — umbrales globales: disco (%), backup fallido, PBS stale (h)
- `/settings/ip-bans` — gestión de IPs baneadas
Cada POST carga el config existente y sobreescribe solo sus campos.

`/settings/alerts` son los umbrales **globales** (fallback). La config per-servidor/VM
está planificada en TODO.md y vivirá en `/servers/pve/{id}/alerts` y `/servers/pbs/{id}/alerts`.

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

Embedded in `internal/db/migrations/`. Applied automatically on server startup via `schema_migrations` table.

| File | Contents |
|------|----------|
| `001_initial.up.sql` | All base schema: servers, reports, storages, email_config, users, API keys, roles |
| `002_settings.up.sql` | Settings columns in email_config (retention, alerts) |
| `003_ip_bans.up.sql` | `ip_bans` table |
| `004_backup_tasks.up.sql` | `pve_backup_tasks` table |
| `005_remove_mobile_key_type.up.sql` | Remove deprecated key type |
| `006_pbs_snapshots.up.sql` | `pbs_snapshots` table + `alert_pbs_stale_hours` column in email_config |
| `007_alert_config.up.sql` | (planned) `pve_alert_config`, `pve_vm_alert_config`, `pbs_alert_config` |

### Test fixtures (`testdata/`)

- `seed.sh` - envía un reporte PVE y uno PBS al servidor vía API (requiere clave `pbk-` activa)
- `seed_history.go` - inserta 6 días de reportes históricos directamente en la BD SQLite (`go run testdata/seed_history.go`); útil para probar el gráfico de duración y la vista de historial
- Ejecutar siempre `seed.sh` primero (crea los servidores), luego `seed_history.go`

### Backup tasks por VM (2026-05)

Contexto de uso: el usuario revisa las copias cada día a las 9h. Los backups corren de noche (ej. lunes 21h, acaban martes 02h). Algunos PVE con PBS hacen 2 jobs/día (mediodía + medianoche). Solo importa el **último job** — si el de medianoche falla, el backup está incompleto aunque el de mediodía funcionara.

**Tabla `pve_backup_tasks` (migration 004):**
Cada reporte PVE tiene N filas, una por tarea vzdump del último job. Campos: `report_id`, `vmid`, `vm_name`, `status` (texto completo de Proxmox, "OK" o mensaje de error), `starttime`, `endtime`, `duration`, `size` (bytes del fichero en storage), `filename` (volid).

**Cómo el cliente detecta el "último job" (`client/pve.go → backupJobTasks`):**
1. Consulta `nodes/{node}/tasks?typefilter=vzdump&limit=100`, ordena por `endtime DESC`.
2. Toma la tarea más reciente como ancla. Añade tareas consecutivas mientras el gap entre `prevTask.start` y `currentTask.end` sea < 2h. Al superar 2h de hueco, para — es un job distinto.
3. Deduplica por VMID (se queda con la más reciente dentro del job).
4. Enriquece cada tarea con nombre de VM (de `nodes/{node}/qemu` y `lxc`) y fichero+tamaño (cruzando con storage content por VMID y ventana `[task.start-60, task.end+60]`).

**PBS pull-mode task IDs:** en PVE configurado con PBS como destino, las tareas vzdump pueden tener IDs en formato `"vm/101"` o `"ct/101"` en lugar del numérico `"101"`. `parseVMID()` en `client/pve.go` maneja ambos formatos.

**Ventana de matching de ficheros:** se usa `f.ctime >= task.start-60 && f.ctime <= task.end+60` (no `±300s desde starttime`). El motivo: `±300s` causaba cross-matching entre tareas consecutivas de jobs distintos.

**Por qué 2h de gap:** backups consecutivos dentro de un job son secuenciales (VM A acaba → VM B empieza, gap de segundos/minutos). Entre jobs distintos (mediodía vs medianoche) el gap es de horas. 2h es suficientemente grande para no partir un job largo y suficientemente pequeño para separar dos jobs del mismo día.

**Staleness con schedule (`service/report.go → IsStaleForServer`):**
- Lee `VMBackupConfig` del servidor para saber qué días de la semana tiene backups.
- Busca hacia atrás el último día programado que ya "completó": `dayStart + 28h < now` (28h de gracia para backups de madrugada que pertenecen al día anterior).
- Stale si no se recibió ningún reporte desde el inicio de ese día.
- Sin config → fallback a `IsStale` (reporte recibido hoy).
- Retorna `(bool, string)` — el string es la razón ("no report received today" o "no report received on last backup day").

**Ejemplo (backup solo Lun-Vie, corre a las 21h, acaba ~02h):**
- Sábado 10h: último día programado = Viernes. `Vie 00h + 28h = Sab 04h < Sab 10h` → completado. Reporte del Sáb 02h ≥ Vie 00h → **no stale** ✓
- Domingo: mismo razonamiento → **no stale** ✓
- Martes 05h (si lunes falló): `Lun 00h + 28h = Mar 04h < Mar 05h` → completado. Sin reporte desde Lun 00h → **stale** ✓

**UI en `web/templates/server_pve_detail.html` (PVE):**
- Card "Último job de backup": tabla VMID/Nombre/Estado/Duración/Tamaño/Fichero del job más reciente. El tooltip del badge ERROR muestra el mensaje completo de Proxmox.
- Accordion "Historial de jobs (últimos 5 días)": hasta 4 jobs anteriores, colapsables, con badge OK/ERROR en la cabecera. Solo muestra entradas que tengan tasks en DB (los reportes anteriores al despliegue de esta feature no tienen tasks).
- Ambas secciones usan datos de `pve_backup_tasks` ligados al `report_id` del reporte correspondiente.

### PBS snapshots (2026-05)

**Migration 006** añade tabla `pbs_snapshots` y columna `alert_pbs_stale_hours` en `email_config`.

**Cliente (`client/pbs.go`):**
1. Para cada datastore, consulta `admin/datastore/{store}/groups` (lista de grupos de backup).
2. Consulta `admin/datastore/{store}/snapshots` para obtener `size` y `verification.state` del snapshot más reciente de cada grupo (el endpoint de groups omite ambos campos).
3. Inyecta `size` y `verification-state` en cada grupo antes de enviar el payload.

**Campos en `pbs_stores`:** `estimated_full_date` (Unix timestamp, 0 si no disponible), `mount_status` (texto, "available" o error).

**Campos en `pbs_snapshots`:** `backup_type`, `backup_id`, `last_backup`, `backup_count`, `owner`, `comment`, `verification_state`, `size`.

**Relación:** `pbs_snapshots.store_id` → `pbs_stores.id` → `pbs_reports.id`. Los snapshots se muestran del último reporte: `GetPBSStoresForReport(latestReport.ID)` → por cada store, `GetPBSSnapshotsForStore(store.ID)`.

**UI en `web/templates/server_pbs_detail.html`:**
- Tarjeta de datastore: muestra `EstimatedFullDate` (con `isPast` para detectar si ya pasó sin riesgo real), `MountStatus`.
- `<details>` colapsable "Grupos de backup": tabla con tipo/ID/último backup/copias/tamaño/verificación/propietario.

### Alert system — estado actual y roadmap (2026-05)

**Estado actual:** umbrales globales en `email_config`. `internal/store/alerts.go` contiene `GetAlerts(diskPct, backupErr)` (PVE disk + PBS disk + PVE backup errors) y `GetPBSStaleAlerts(hours)`. El dashboard llama ambos y añade task-level alerts manualmente.

**Roadmap completo:** ver `TODO.md → PLAN: Sistema de Alertas por Servidor/VM`.

Resumen de la arquitectura planificada:
- Migration 007: `pve_alert_config`, `pve_vm_alert_config`, `pbs_alert_config`
- `internal/service/alertengine.go`: slice de `AlertEvaluator` functions — añadir tipo = añadir función
- Página `/alerts` (entre Dashboard y Proxmox VE en el nav): vista tipo Grafana/Zabbix
- Config por servidor en `/servers/pve/{id}/alerts` y `/servers/pbs/{id}/alerts`
