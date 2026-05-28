# TODO - Probakgo

## Revision produccion - 2026-05-28

### P1 - Bloqueantes antes de produccion

- [x] Limitar cada API key `pbk-` a su servidor esperado.
  - Hoy una key valida puede listar todos los PVE/PBS, ver reportes y modificar backup configs de cualquier servidor.
  - Rutas afectadas: `internal/api/router.go`, `internal/api/handlers/server.go`, `internal/api/handlers/backup.go`, `internal/api/handlers/report.go`.

- [x] Hacer obligatorio el `machine_id` cuando la API key ya esta ligada.
  - Hoy el binding se salta si el cliente omite `X-Machine-ID`.
  - Rutas afectadas: `internal/service/auth.go`, `client/sender.go`.

- [x] Recalcular alertas stale de PVE/PBS en `/alerts` usando la misma logica que el email.
  - Hoy `evalPVEStale` depende de `rep.IsStale`, no usa `IsStaleForServer`, ignora servidores sin reportes y falta equivalente claro para PBS sin reporte.
  - Rutas afectadas: `internal/service/alertengine.go`, `internal/service/report.go`, `internal/service/email.go`.

### P2 - Importantes

- [x] Añadir rate limit especifico por API key/servidor para endpoints de clientes.
  - Hoy existe un limite global del API, pero conviene limitar por `pbk-`/servidor para evitar que un cliente mal configurado o comprometido sature reportes y escrituras.
  - Rutas afectadas: `internal/api/router.go`, `internal/api/middleware.go`, `internal/ratelimit`.

- [x] Aplicar supresiones tambien en el correo diario.
  - Hoy `/alerts` filtra `alert_suppressions`, pero el email incluye alertas de `RunAll` sin filtrar.
  - Rutas afectadas: `internal/service/email.go`, `internal/web/handlers/alerts.go`.

- [x] Validar `StatusCode` en self-update antes de escribir binarios o checksums.
  - Hoy una respuesta HTTP no 200 podria acabar escrita como binario si no hay checksum valido obligatorio.
  - Rutas afectadas: `internal/selfupdate/selfupdate.go`.

- [x] Tratar cualquier respuesta Proxmox no 2xx como error en el cliente.
  - Hoy PVE solo corta 401/403 y PBS solo 401; otras respuestas pueden parsearse y producir reportes incompletos.
  - Rutas afectadas: `client/pve.go`, `client/pbs.go`.

### P3 - Documentacion

- [x] Actualizar documentacion de acceso inicial.
  - README e INSTALLATION dicen `probakgo/admin123`, pero el codigo genera password aleatoria y la muestra en logs.
  - Rutas afectadas: `README.md`, `INSTALLATION.md`, `main.go`.
