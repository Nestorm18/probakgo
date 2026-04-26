# TODO - probakgo

---

## Problemas conocidos

### Selfupdate — estado actual ✅

El sistema completo está operativo:

- **Workflows**: `ci.yml` corre en cada push (build + vet + tests). `release.yml` crea release en GitHub al hacer push de un tag `v*`.
- **Subcomando `update`**: cableado en `client/main.go` → `selfupdate.Run("Nestorm18/probakgo", "probakgo-client", version)`.
- **Versión inyectable**: cambiado `const version` a `var version` en `client/main.go`; el release.yml inyecta `-X main.version=${{ github.ref_name }}`.
- **Asset naming**: `release.yml` genera `probakgo-client_linux_amd64/arm64` que coincide exactamente con lo que busca `selfupdate.go`.

**Para crear un release**: `git tag v1.0.0 && git push origin v1.0.0` → GitHub Actions construye los 4 binarios, genera SHA256SUMS y publica el release automáticamente.

---

## Pendiente

- [x] Tests unitarios para el servidor (fases 1-7 completadas, 79 tests en total)
- [x] Cablear subcomando `update` en `client/main.go` → `selfupdate.Run("Nestorm18/probakgo", "probakgo-client", version)`
- [x] Inyectar versión en cliente via `-ldflags "-X main.version=vX.Y.Z"` en release (cambiado `const` → `var version`)
- [x] CI/CD listo — `git tag v1.0.0 && git push origin v1.0.0` crea el release completo
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

### Fase 2 — Service (`internal/service/`) ✅

**`report_test.go`**
- [x] `TestIsStale_TodayNotStale` — reporte de hoy → false
- [x] `TestIsStale_YesterdayStale` — reporte de ayer → true
- [x] `TestSavePVEReport_FullRoundTrip` — guarda reporte con storages+content, consulta y verifica
- [x] `TestSavePBSReport_FullRoundTrip` — guarda reporte PBS, consulta stores
- [x] `TestBuildPVEServerResponse_NoReport` — servidor sin reportes → IsStale=true
- [x] `TestBuildPVEServerResponse_StaleReport` — reporte de ayer → IsStale=true, StaleReason correcto

**`cleanup_test.go`**
- [x] `TestRunCleanup_Disabled` — RetentionEnabled=false → sin borrados
- [x] `TestRunCleanup_DeletesOld` — inserta viejo+nuevo, limpia, solo quedan nuevos

### Fase 3 — API handlers (`internal/api/handlers/`) ✅

**`report_test.go`**
- [x] `TestReportPVE_HappyPath` — POST JSON válido con API key pbk- → 200
- [x] `TestReportPVE_MissingHostname` — sin hostname → 400
- [x] `TestReportPVE_InvalidJSON` — body mal formado → 400
- [x] `TestReportPVE_NoAuth` — sin Authorization → 401
- [x] `TestReportPBS_HappyPath` — mismo para PBS → 200
- [x] `TestReportPBS_NoAuth` — sin auth → 401

### Fase 4 — Store adicional (`internal/store/`) ✅

**`user_test.go`**
- [x] `TestCreateUser_And_GetByUsername`
- [x] `TestToggleUser`
- [x] `TestUpdateUserPassword`
- [x] `TestUpdateUserRole`

**`backup_test.go`**
- [x] `TestCreateAndListVMBackupConfig`
- [x] `TestUpdateVMBackupConfig`
- [x] `TestDeleteVMBackupConfig_SoftDelete`
- [x] `TestToggleVMExclude`

### Fase 5 — Service auth (`internal/service/`) ✅

**`auth_test.go`**
- [x] `TestExtractBearer_WithPrefix`
- [x] `TestExtractBearer_WithoutPrefix`
- [x] `TestValidateServerKey_HappyPath`
- [x] `TestValidateServerKey_WrongType`
- [x] `TestValidateServerKey_MachineBinding_First`
- [x] `TestValidateServerKey_MachineBinding_Mismatch`
- [x] `TestValidateAdminKey_HappyPath`
- [x] `TestValidateAdminKey_WrongType`

### Fase 6 — API handlers adicionales (`internal/api/handlers/`) ✅

**`admin_test.go`**
- [x] `TestCreateAPIKey_HappyPath`
- [x] `TestCreateAPIKey_MissingFields`
- [x] `TestCreateAPIKey_RequiresAdminKey`
- [x] `TestListAPIKeys`
- [x] `TestDeleteAPIKey`

**`server_test.go`**
- [x] `TestListPVEServers_Empty`
- [x] `TestListPVEServers_WithServer`
- [x] `TestListPVEReports_NotFound`
- [x] `TestListPVEReports_HappyPath`

**`backup_test.go`**
- [x] `TestGetBackupConfig_Empty`
- [x] `TestCreateVMConfig_HappyPath`
- [x] `TestCreateVMConfig_MissingVMID`

### Fase 7 — Service email (`internal/service/`) ✅

**`email_test.go`**
- [x] `TestParseRecipients_Multiple`
- [x] `TestParseRecipients_Empty`
- [x] `TestNextRunTime_Future`
- [x] `TestNextRunTime_Past`
- [x] `TestBuildEmailData_AllOK`
- [x] `TestBuildEmailData_WithStale`

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
