# AGENTS.md

Guidance for Codex when working in this repository. Keep changes small, explicit, and easy to verify.

## Working Rules

- Prefer simple, surgical changes. Do not refactor unrelated code.
- If a requirement is ambiguous, ask before coding.
- Match existing style and naming. The project name is **probakgo**.
- Use Conventional Commits when committing: `feat:`, `fix:`, `refactor:`, `docs:`, `chore:`.
- Always verify meaningful code changes with tests or a targeted build.
- Always bump versions together after code changes:
  - `main.go`
  - `client/main.go`
  - `client-windows/main.go`

## Layout

```text
main.go          - server binary: API + web UI on port 36748
client/          - Proxmox PVE/PBS client
client-windows/  - Windows monitoring client
internal/api/    - REST API, prefix /api/
internal/web/    - Web UI
internal/service/- auth, reports, alerts, email
internal/store/  - SQLite queries
internal/db/     - embedded migrations
internal/domain/ - shared models and payloads
web/templates/   - Go html/template files
web/static/      - CSS/JS
```

## Builds

```bash
go build -o probakgo .
go build -o probakgo-client ./client/
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o probakgo-windows-client.exe ./client-windows/
go test ./...
```

Release assets must stay in sync with workflows and download handlers:

- `probakgo_linux_amd64`
- `probakgo-client_linux_amd64`
- `probakgo-windows-client_windows_amd64.exe`

## Server

- API endpoints include PVE, PBS and Windows reports, heartbeat, backup config, API keys and downloads.
- Web pages include dashboard, alerts, PVE, PBS, Windows, users, API keys, settings, profile and about.
- Auth uses bcrypt, sessions and RBAC: `reader`, `editor`, `admin`.
- API keys are `pbk-` server/client keys. Machine ID binding is enforced.
- Settings pages live under `/settings/*`; global alert thresholds live in `/settings/alerts`.

## Clients

### Proxmox Client

- Detects PVE/PBS from `/etc/issue`.
- PVE reports to `POST /api/report/pve`; PBS reports to `POST /api/report/pbs`.
- Heartbeat uses `POST /api/heartbeat`.
- Machine ID comes from `/etc/machine-id`.
- Subcommands: `install`, `uninstall`, `update`, `heartbeat`, `doctor`, `version`.
- `install` writes `/opt/probakgo/.env`, installs logrotate, update cron and the heartbeat systemd timer.
- PVE auto-config reads `/cluster/backup` to infer expected VM backup days.
- PBS reports include the latest completed remote sync and garbage collection tasks when its API exposes them.

### Windows Client

- Lives in `client-windows/` to keep PowerShell/WMI logic separate.
- Full report goes to `POST /api/report/windows`.
- Heartbeat uses `POST /api/heartbeat` with `server_type=windows`.
- Machine ID is Windows `MachineGuid`.
- `install --api-url ... --api-key ...` installs to `C:\ProgramData\Probakgo`, writes `.env`, and creates scheduled task `Probakgo Windows Report` every 5 minutes as SYSTEM.
- Subcommands: `install`, `update`, `heartbeat`, `doctor`, `version`.
- Logs are written to `C:\ProgramData\Probakgo\probakgo-windows-client.log`, rotate daily as `probakgo-windows-client-YYYY-MM-DD.log`, and keep the last 7 days only.
- Reports local/public IP, version, MachineGuid, fixed logical volumes and best-effort physical disk health.
- Alerts include Windows heartbeat, disk usage, disk health and missing logical volumes since the previous report.
- CPU/RAM monitoring is intentionally out of scope for now.

## Database

- Migrations are embedded in `internal/db/migrations/` and run automatically.
- Current latest migration: `034_pbs_maintenance_tasks.up.sql`.
- Nullable SQLite text fields must scan into `sql.NullString`, not `string`.
- Tests should use the real migration path via `openTestDB(t)` / `openTestStore(t)`.

## Web Templates

- Register template helpers in `makeFuncMap()` in `internal/web/handlers/templates.go`.
- Add every new template to `templateActive`.
- Template render fixtures in `templates_test.go` must cover every template.
- `formatBytes` uses SI base 1000.

## Alerts

- All alerts run through `internal/service/alertengine.go`.
- Add alert types by adding an evaluator to the `evaluators` slice.
- PVE/PBS can have per-server overrides; PVE can also have per-VM overrides.
- Windows currently uses global disk and heartbeat thresholds only.
- Suppressions live in `alert_suppressions`; maintenance windows live in `server_maintenance`; deleting API-key-bound server data must remove related suppressions, maintenance and heartbeats.

## Important Behavior

- PVE backup status is based on the last vzdump job, grouped by small task gaps and deduped by VMID.
- PVE staleness uses configured backup schedules and expected finish time, not just "today".
- PBS snapshots are informational; stale PBS snapshot alerts were intentionally removed/avoided for old retained backups.
- Swap detection exists for PVE/PBS reports and should remain visible in dashboard, PVE and PBS pages.
