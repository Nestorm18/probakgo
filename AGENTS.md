# AGENTS.md

Guidance for Codex when working in this repository. Keep changes small, explicit, and easy to verify.

## Working Rules

- Prefer simple, surgical changes. Do not refactor unrelated code.
- If a requirement is ambiguous, ask before coding.
- Match existing style and naming. The project name is **probakgo**.
- Use Conventional Commits when committing: `feat:`, `fix:`, `refactor:`, `docs:`, `chore:`.
- Always verify meaningful code changes with tests or a targeted build.
- After code changes, bump the single release version in `internal/version/version.go`.

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

The module and CI currently use Go 1.26.5.

```bash
go build -o probakgo .
go build -o probakgo-client ./client/
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o probakgo-windows-client.exe ./client-windows/
go vet ./...
go test ./...
```

Release assets must stay in sync with workflows and download handlers:

- `probakgo_linux_amd64`
- `probakgo-client_linux_amd64`
- `probakgo-windows-client_windows_amd64.exe`

## Server

- API endpoints include PVE, PBS and Windows reports, heartbeat, backup config, API keys and downloads.
- Web pages include dashboard, alerts, PVE, PBS, Windows, users, API keys, profile, about and the settings hub.
- Auth uses bcrypt, sessions and RBAC: `reader`, `editor`, `admin`.
- TOTP 2FA can be enforced for editors/admins and for sensitive actions. User security changes revoke existing sessions.
- API keys are `pbk-` client keys, bind to the first reporting Machine ID and can be revealed only after credential checks.
- API keys, SMTP passwords and TOTP secrets are encrypted at rest with `DATA_ENCRYPTION_KEY`; API-key authentication uses a keyed lookup hash.
- Settings under `/settings/*` cover system/security, email, retention/database backup, alerts, IP bans, audit log and operational reset.
- The UI exposes CSV/JSON exports for alerts and PVE/PBS server/report data.
- The standard `/opt/probakgo` systemd unit runs as the dedicated `probakgo` user with filesystem and process hardening.
- The initial admin password is stored in a `0600` one-time file and retrieved with `probakgo initial-password`; it must never be logged.

## Clients

### Proxmox Client

- Detects PVE/PBS from `/etc/issue`.
- PVE reports to `POST /api/report/pve`; PBS reports to `POST /api/report/pbs`.
- Heartbeat uses `POST /api/heartbeat`.
- Machine ID comes from `/etc/machine-id`.
- Subcommands: `install`, `uninstall`, `update`, `heartbeat`, `doctor`, `version`.
- `install` writes `/opt/probakgo/.env`, creates `/usr/local/bin/probakgo-client`, installs logrotate, a host-jittered update cron during the 01:00 hour and the heartbeat systemd timer.
- PVE reports run from the vzdump hook; PBS gets an additional daily report cron at 06:00.
- PVE auto-config reads `/cluster/backup` to infer expected VM backup days.
- PBS reports include the latest completed remote sync and garbage collection tasks when its API exposes them.

### Windows Client

- Lives in `client-windows/` to keep PowerShell/WMI logic separate.
- Full report goes to `POST /api/report/windows`.
- Heartbeat uses `POST /api/heartbeat` with `server_type=windows`.
- Machine ID is Windows `MachineGuid`.
- `install --api-url ... --api-key ...` installs to `C:\ProgramData\Probakgo`, writes `.env`, and creates `Probakgo Windows Report` every 5 minutes plus `Probakgo Windows Update` daily as SYSTEM.
- Subcommands: `install`, `update`, `heartbeat`, `doctor`, `version`.
- Logs are written to `C:\ProgramData\Probakgo\probakgo-windows-client.log`, rotate daily as `probakgo-windows-client-YYYY-MM-DD.log`, and keep the last 7 days only.
- Reports local/public IP, version, MachineGuid, fixed logical volumes and best-effort physical disk health.
- Alerts include Windows heartbeat, disk usage, disk health and missing logical volumes since the previous report.
- Windows supports a per-server disk threshold plus per-server maintenance mode.
- CPU/RAM monitoring is intentionally out of scope for now.

## Database

- Migrations are embedded in `internal/db/migrations/` and run automatically.
- Current latest migration: `041_alert_resolution_push_pending.up.sql`.
- Nullable SQLite text fields must scan into `sql.NullString`, not `string`.
- Tests should use the real migration path via `openTestDB(t)` / `openTestStore(t)`.

## Web Templates

- Register template helpers in `makeFuncMap()` in `internal/web/handlers/templates.go`.
- Add every new template to `templateActive`.
- Template render fixtures in `templates_test.go` must cover every template.
- `formatBytes` uses SI base 1000.
- Inline scripts require the per-request CSP nonce. Pinned CDN assets require matching SRI and `crossorigin="anonymous"`.

## PWA / Web Push

- The web UI is installable as a PWA: `web/static/manifest.webmanifest` declares the icons, scope and theme colour; `web/static/sw.js` is the service worker that displays the toast on `push` events and focuses the relevant page on click.
- `/sw.js` is served with `Service-Worker-Allowed: /` so the worker can claim the entire origin from a sub-FS embed. The manifest is served at `/manifest.webmanifest` with `application/manifest+json`.
- VAPID keys live in the single-row `push_config` table; the private key is encrypted with the same `secretbox` as SMTP/TOTP/API keys. Keys are generated lazily on the first subscription via `service.PushSender.EnsureVAPIDKeys`.
- Subscriptions are scoped to the logged-in user (`push_subscriptions.user_id`) and cascade-delete with the `users` row. The unsubscribe endpoint refuses to remove a subscription that does not belong to the requesting user.
- `SendImmediateCriticalAlerts` fans out critical alerts (and resolutions) to every active push subscription in a background goroutine. Email and push keep separate delivery state, so one channel cannot suppress or duplicate the other; push is marked sent after at least one device accepts it.
- The push sender is a process-wide singleton wired up in `main.go` via `service.SetPushSender`; tests that do not care about push leave it as `nil`.
- The PWA icons are generated by `tools/genpwaicons` (`go run ./tools/genpwaicons`); re-run when the brand colour changes.

## Alerts

- All alerts run through `internal/service/alertengine.go`.
- Add alert types by adding an evaluator to the `evaluators` slice.
- PVE/PBS can have per-server overrides; PVE can also have per-VM overrides.
- Windows inherits the global Windows disk threshold and can override it per server; heartbeat uses the global PVE heartbeat interval.
- Suppressions live in `alert_suppressions`; maintenance windows live in `server_maintenance`; deleting API-key-bound server data must remove related suppressions, maintenance and heartbeats.
- Alert state/history drives immediate critical and resolution emails, plus PWA push notifications. PBS snapshot age remains informational and is not an active evaluator.

## Important Behavior

- PVE backup status is based on the last vzdump job, grouped by small task gaps and deduped by VMID.
- PVE staleness uses configured backup schedules and expected finish time, not just "today".
- PBS snapshots are informational; stale PBS snapshot alerts were intentionally removed/avoided for old retained backups.
- Swap detection exists for PVE/PBS reports and should remain visible in dashboard, PVE and PBS pages.
- Operational reset removes PVE/PBS/Windows reports and configuration but preserves users, audit logs and `schema_migrations`.
