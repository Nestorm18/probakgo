# TODO - probakgo

---

## Problemas conocidos

### Selfupdate roto en 4 puntos

El paquete `internal/selfupdate/selfupdate.go` está bien implementado internamente, pero el sistema completo no funciona:

**Problema 1 - Workflows sin activar**
`.github/workflows/ci.yml` y `release.yml` están escritos con triggers correctos (push/tags), pero no se han probado en producción (modo dev activo, no activar hasta revisión manual).

**Problema 2 - Subcomando `update` no implementado**
`client/main.go` tiene el stub (`update`) pero devuelve error. Falta cablear:
```go
selfupdate.Run("Nestorm18/probakgo", "probakgo-client", version)
```

**Problema 3 - `version` del cliente hardcoded**
`client/main.go` usa `const version = "dev"`. Debe inyectarse en el build de release:
```
go build -ldflags="-X main.version=v1.2.3" -o probakgo-client ./client/
```
Sin esto, `selfupdate` no puede comparar versiones.

**Problema 4 - Asset naming en selfupdate**
`selfupdate.go` busca assets con formato `{nombre}_{os}_{arch}` (ej: `probakgo-client_linux_amd64`).
El `release.yml` ya usa ese formato - verificar cuando se active CI.

---

## Pendiente

- [ ] Tests unitarios para el servidor (ver plan detallado más abajo)
- [ ] Cablear subcomando `update` en `client/main.go` → `selfupdate.Run("Nestorm18/probakgo", "probakgo-client", version)`
- [ ] Inyectar versión en cliente via `-ldflags "-X main.version=vX.Y.Z"` en release
- [ ] Activar y probar CI/CD (cuando salga de modo dev)
- [x] Limpieza automática de reportes con más de 3 meses en BD, configurable en settings
- [x] Alertas en caso de volumen casi lleno o errores, tanto en dashboad como en email (configurable)
- [x] Revisar consistencia de ventanas de añadir/editar alguna vez son dialog y otras páginas completas
- [x] Fix dialogos de alert por personalizados — `#confirmModal` genérico en `base.html`, eliminados todos los `confirm()` nativos de `api_keys.html`, `backup_config.html`, `users.html`
- [x] Si el backup se hizo en un pbs puede estar comprobado la integridad o pendiente de comprobación `"verification": {"state": "ok"}` — campo `verification` en `pve_storage_content`, extracción en cliente, badge en template `reports_pve.html`
- [x] Color sidebar izquierdo en tema claro - texto poco visible en modo claro (Configuración, Monitor, etc.)
- [x] Fixtures de prueba: `testdata/fixture_pve.json` (incluye storage tipo `pbs` con verificación ok/failed/–), `testdata/fixture_pbs.json`, `testdata/seed.sh`
- [x] Historial de fixtures: `testdata/seed_history.go` - inserta 6 días de reportes históricos con estados y duraciones variadas

---

## Tests servidor — plan (2026-04-25)

Estrategia: SQLite `:memory:` con migraciones reales para store/service.
API handlers con `httptest.NewRecorder`. Sin mocks — todo integrado.

### Fase 1 — Store (`internal/store/`)
Helper compartido: `testhelpers_test.go` → `openTestDB(t) *store.Store`

**`pve_test.go`** ✅
- [x] `TestUpsertPVEServer_CreateAndUpdate`
- [x] `TestInsertPVEReport_NilBackupStatus`
- [x] `TestGetLatestPVEReport_ReturnsNewest`
- [x] `TestGetLatestPVEReport_NoReports`
- [x] `TestDeleteOldPVEReports`

**`pbs_test.go`** ✅
- [x] `TestUpsertPBSServer_CreateAndUpdate`
- [x] `TestInsertPBSReport_And_GetLatest`
- [x] `TestDeleteOldPBSReports`

**`email_test.go`** ✅
- [x] `TestGetEmailConfig_Defaults`
- [x] `TestUpsertEmailConfig_RoundTrip`
- [x] `TestUpsertEmailConfig_UpdateInPlace`

**`alerts_test.go`** ✅ (6 tests, incluye `PBSDisk_UnderThreshold` extra)
- [x] `TestGetAlerts_DiskPctZero_NoCheck`
- [x] `TestGetAlerts_PBSDisk_OverThreshold`
- [x] `TestGetAlerts_PBSDisk_UnderThreshold`
- [x] `TestGetAlerts_PVEDisk_OverThreshold`
- [x] `TestGetAlerts_BackupError`
- [x] `TestGetAlerts_BackupOK_NoAlert`

### Fase 2 — Service (`internal/service/`)

**`report_test.go`**
- [ ] `TestIsStale_TodayNotStale` — reporte de hoy → false
- [ ] `TestIsStale_YesterdayStale` — reporte de ayer → true
- [ ] `TestSavePVEReport_FullRoundTrip` — guarda reporte con storages+content, consulta y verifica
- [ ] `TestSavePBSReport_FullRoundTrip` — guarda reporte PBS, consulta stores
- [ ] `TestBuildPVEServerResponse_NoReport` — servidor sin reportes → IsStale=true
- [ ] `TestBuildPVEServerResponse_StaleReport` — reporte de ayer → IsStale=true, StaleReason correcto

**`cleanup_test.go`**
- [ ] `TestRunCleanup_Disabled` — RetentionEnabled=false → sin borrados
- [ ] `TestRunCleanup_DeletesOld` — inserta viejo+nuevo, limpia, solo quedan nuevos

### Fase 3 — API handlers (`internal/api/handlers/`)

**`report_test.go`**
- [ ] `TestReportPVE_HappyPath` — POST JSON válido con API key pbk- → 200
- [ ] `TestReportPVE_MissingHostname` — sin hostname → 400
- [ ] `TestReportPVE_InvalidJSON` — body mal formado → 400
- [ ] `TestReportPVE_NoAuth` — sin Authorization → 401
- [ ] `TestReportPBS_HappyPath` — mismo para PBS → 200
- [ ] `TestReportPBS_NoAuth` — sin auth → 401

---

## Tests cliente (añadidos 2026-04-25)

- [x] `client/sysinfo_test.go` — `TestServerTypeFromContent`: detección pve/pbs/unknown desde contenido de `/etc/issue`
- [x] `client/pve_test.go` — verificación extraction (ok/failed/vacío), `lastBackupStatus` (vacío, ordenación, más reciente OK/ERROR), storage offline en error de contenido
- [x] `client/pbs_test.go` — auth header PBS, error 401, parsing de `status/datastore-usage` con gc-status
- [x] `client/sender_test.go` — envío desde fichero PVE/PBS, errores 401/500, fichero inexistente

---

## Web UI (auditado 2026-04-25)

- [x] 36 rutas implementadas, 17 templates completos
- [x] Gráfico de duración en `/servers/pve/{id}/reports` - Chart.js 4.4.0 via CDN
- [x] Todos los handlers con datos reales
- [x] Roles 3-tier, modales de confirmación personalizados, flash messages, tema claro/oscuro
- [x] Reveal de API key - modal con contraseña + AJAX
- [x] Botones de acción (ver / reportes / backup config) en tablas `/servers/pve` y `/servers/pbs`
- [x] Bug: API key visible en claro en `/api-keys/{id}/edit` - corregido con `keyPreview`
- [x] Bug: `stale_reason` NULL rompía el scan de reportes (todos mostraban "Sin reporte") - corregido con `sql.NullString`

---

## Entorno de despliegue

- **Servidor:** Debian - binario `probakgo` directo, sin Docker
- **Cliente:** Debian en nodo Proxmox (PVE o PBS) - `probakgo-client install`
