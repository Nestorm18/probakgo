# TODO - Probakgo

Trabajo pendiente real después de la revisión de la versión 0.0.191. Lo ya implementado se documenta en el código, los tests y las guías.

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
