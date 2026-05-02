# todo

## Alta prioridad — bugs reales

- [x] **Email report usa `IsStale` en vez de `IsStaleForServer`** (`internal/service/email.go` ~líneas 158-163 PVE, 178-180 PBS).
  Impacto: el correo diario marca como "stale" servidores que solo hacen backup L-V cuando lo abres en sábado/domingo, aunque el viernes funcionara. Mismo bug que ya arreglamos en la web.
  Fix: usar `r.IsStaleForServer(reportedAt, sv.Name)` en `buildEmailData` para PVE.

- [ ] **Staleness PBS en la web usa check simple** (`internal/web/handlers/servers.go` `PBSServers`).
  PVE usa `IsStaleForServer`, PBS solo `rep.IsStale`. Aunque ahora mismo PBS no tiene `vm_backup_configs`, conviene extraer una versión de schedule para PBS o al menos respetar la "ventana de gracia" de 28h. Pero el pbs no hace copias, este se gestiones desde pve. No es necesario mas logica en el pbs.

## Media — funcionalidad

- [ ] **Cliente PBS recoge solo `status/datastore-usage`**.
  No hay info de snapshots / último backup recibido / fallos. PVE consulta tasks, content, VMs. Para PBS faltaría algo como `admin/datastore/{store}/snapshots` para ver "último snapshot recibido por VM" y poder detectar VMs sin backup desde N días.

- [x] **Email PBS sin detalle por datastore**.
  La sección PVE del correo lista VMs con OK/ERROR/SIN BACKUP. La sección PBS solo dice "OK / sin reporte". Añadir tabla por datastore con uso, último GC, último snapshot.

- [ ] **Dashboard alerts no incluye PBS**.
  `dashboard.go` itera `pveServers` para alertas de tasks/missing. Para PBS solo hay alerta de disco vía `GetAlerts`. Faltan alertas tipo "datastore X sin snapshot reciente".

## Baja — calidad

- [x] **Sin tests para `IsStaleForServer`**.
  Solo hay tests de `IsStale` simple. Falta cubrir: schedule L-V con consulta en sábado, ventana de gracia 28h, servidor sin config (fallback), config con `IsExcluded`.

- [x] **Sin tests para `pve_backup_tasks`** (`InsertPVEBackupTask`, `GetPVEBackupTasksForReport`).

- [x] **Sin tests para detección de MISSING VMs** (`vmScheduledForDay`, lógica del handler `PVEServerDetail`).

## Notas

- No hay endpoints API huérfanos ni referencias a "probaky" sin migrar.
- Sin SQL injection visible (queries parametrizadas).
- Plantillas y handlers están sincronizados.
