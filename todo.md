# TODO - Probakgo

Trabajo pendiente real después de la revisión de la versión 0.0.191. Lo ya implementado se documenta en el código, los tests y las guías.

## Correcciones pendientes (revisión de la versión 0.0.269)

Confirmadas con tests temporales:

- Un backup nocturno perdido no se detecta si el job anterior terminó pasada la medianoche. `isStaleForConfigsAt` (`internal/service/report.go`) compara la hora de llegada del reporte con las 00:00 del día de backup, así que el reporte del job anterior tapa la ausencia; un fallo aislado no se detecta nunca. Valorar comparar con la hora de inicio del job (`backup_starttime`).
- Borrar una API key desvinculada deja servidores huérfanos. `DeleteAPIKeyPost` (`internal/web/handlers/apikeys.go`) solo borra los datos del servidor si la key tiene `server_name`, y «Desvincular» lo vacía. El servidor PVE/PBS queda con un `api_key_id` inexistente, sigue generando alertas y no se puede eliminar desde la interfaz. Llamar siempre a `HardDeleteServerDataForAPIKey`.
- Reactivar desde la web a un usuario desactivado por 2FA no sirve. `ToggleUser` (`internal/store/user.go`) no reinicia `totp_grace_started_at`, así que el siguiente login lo desactiva otra vez; solo funciona `probakgo enable-user`.

Detectadas leyendo el código, sin test:

- Informe diario por email: `StartEmailScheduler` calcula la hora una vez y duerme hasta entonces. Un cambio de `send_time` no se aplica hasta después del siguiente envío (puede salir a la hora antigua o dos veces el mismo día) y un envío perdido por reinicio no se recupera.
- Volumen Windows desaparecido: `evalWindowsMissingVolume` compara solo los dos últimos reportes, así que a los 5 minutos la alerta se resuelve sola y envía el aviso de resolución aunque el volumen siga sin aparecer.
- `evalPVEMissingVM` y `missingVMRows` calculan el día del job con la zona horaria del sistema en lugar de `TIMEZONE`. Si difieren, un job que empieza entre las 00:00 y las 02:00 se asigna al día anterior y salen falsas «VM sin backup».
- El borrado de un servidor (`hardDelete*ByID`) no limpia `alert_states`, así que después se envían avisos de «alerta resuelta» de un servidor que ya no existe.
- Un login correcto sin 2FA pone a cero los fallos de la IP (`ClearFailures` en `LoginPost`). Alternando con su propio login, alguien con cuenta puede probar contraseñas de otros sin llegar al bloqueo.
- Los POST de ajustes de `internal/web/handlers/email.go` ignoran el error de `GetEmailConfig`; si la lectura falla, guardan valores por defecto y desactivan umbrales y políticas de 2FA.
- El fixture de `maintenance_settings.html` en `templates_test.go` no rellena `LastAttempt`/`LastSuccess`, así que `formatTimestamp` no se ejecuta en ningún test.

## Seguridad y despliegue

- Configurar en GitHub los *required reviewers* del entorno `release`. El workflow ya usa ese entorno, pero la aprobación es una política externa al repositorio.
- Migrar gradualmente los 663 atributos HTML `style=` a clases CSS para poder retirar también `style-src-attr 'unsafe-inline'`. Los scripts ya usan nonce por petición y `script-src` no permite `unsafe-inline`.
- Valorar servir Bootstrap, Bootstrap Icons y Chart.js desde el binario si se quiere eliminar por completo la dependencia de red. Actualmente están versionados y protegidos con SRI.
- Probar la migración del servicio existente en una copia de producción: la instalación estándar `/opt/probakgo` pasa al usuario dedicado `probakgo`; despliegues en rutas personalizadas mantienen el servicio endurecido como `root`.

## Cobertura

- La cobertura global tiene un suelo CI del 35 % y `internal/ratelimit` supera el 85 %.
- Aumentar pruebas de integración de `internal/web` y del router, que siguen muy por debajo de `internal/db` y `internal/service`.
- Subir el umbral global por etapas cuando esos tests den margen estable.

## Evolución

- Diseñar un modelo común para futuros servidores no Proxmox cuando exista un caso operativo concreto. La identidad y heartbeat ya son comunes; los reportes siguen siendo específicos de PVE, PBS y Windows.
- Mantener el alcance inicial en identidad, heartbeat, discos y alertas básicas.
- Añadir capacidades nuevas solo cuando exista una necesidad operativa concreta.

## Comprobaciones manuales antes de producción

- HTTPS: login, 2FA, usuarios, revelado de API keys, acciones sensibles, CSP y cookie segura.
- VPN: firewall cerrado, URL interna correcta y puerto no publicado.
- PVE/PBS/Windows: instalación limpia, update desde una versión anterior, heartbeat y reporte automático.
- Servicio: migración a `User=probakgo`, permisos de `/opt/probakgo`, auto-update y reinicio tras sustituir el binario.
- Alertas: crítica, warning, supresión, mantenimiento, resolución, email y notificación del navegador.
- Datos: retención, histórico, reset operativo, exportaciones y copia/restauración de SQLite más `.env`.
- Release: entorno aprobado, tres binarios, `SHA256SUMS`, attestations, versión única y `doctor`.
- Recuperación: rollback de binario y compatibilidad de migraciones sobre una copia realista.
