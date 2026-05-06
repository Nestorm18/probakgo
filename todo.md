# todo

## Alta prioridad — bugs reales

## Media — funcionalidad

- [ ] **Cliente PBS recoge solo `status/datastore-usage`**.
  No hay info de snapshots / último backup recibido / fallos. PVE consulta tasks, content, VMs. Para PBS faltaría algo como `admin/datastore/{store}/snapshots` para ver "último snapshot recibido por VM" y poder detectar VMs sin backup desde N días.

- [x] **Email PBS sin detalle por datastore**.
  La sección PVE del correo lista VMs con OK/ERROR/SIN BACKUP. La sección PBS solo dice "OK / sin reporte". Añadir tabla por datastore con uso, último GC, último snapshot.

- [ ] **Dashboard alerts no incluye PBS**.
  `dashboard.go` itera `pveServers` para alertas de tasks/missing. Para PBS solo hay alerta de disco vía `GetAlerts`. Faltan alertas tipo "datastore X sin snapshot reciente".

## Baja — calidad

## Notas
