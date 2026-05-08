# todo

## Pendiente

- [ ] **Overrides de alerta por VM** — `pve_vm_alert_config` tiene store + engine, falta UI en el detalle PVE: tabla de VMs del último job con selector de "backup fallido" y campo "tamaño mínimo MB" por VM. Rutas previstas: `POST /servers/pve/{id}/alerts/vm`.

- [x] **Tiempo hasta llenado en lista PBS** — en `/servers/pbs`, la columna Datastores debe mostrar los días que faltan para que se llene cada datastore (`estimated_full_date` de `pbs_stores`). Si faltan menos de 7 días, mostrar en rojo como indicador de urgencia. El umbral de alerta de `evalPBSFill` ya marca `critical` cuando `daysLeft < 7`, pero eso requiere config por servidor (`DaysUntilFull`); este indicador visual debe mostrarse siempre que `estimated_full_date` exista, independientemente de si hay config de alerta.

- [ ] **Badge de alertas en el nav en todas las páginas** — actualmente el badge solo aparece en dashboard y `/alerts`. Para mostrarlo siempre se necesita middleware que calcule el conteo (con caché de ~60s) e inyecte `AlertCritical`/`AlertWarning` en cada respuesta.
