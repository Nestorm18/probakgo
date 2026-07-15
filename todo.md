# TODO - Probakgo

## V2 - Windows Server

- Hecho: agente Windows separado en `client-windows/` con instalador simple, `.env`, API key, heartbeat/report cada 5 minutos y logs locales.
- Hecho: monitorizar estado de discos fisicos/logicos, salud SMART si esta disponible y espacio libre.
- Hecho: monitorizar hostname, IP publica/local, version del agente y ultimo heartbeat.
- Hecho: vista web propia para Windows con columnas centradas en discos.
- Hecho: alertas basicas de heartbeat, disco, SMART y volumen no detectado desde el ultimo reporte.
- Hecho: comando `doctor` con conectividad real, API key, MachineGuid, discos y tareas programadas.
- Hecho: logs rotados a diario en `C:\ProgramData\Probakgo` conservando 7 dias.
- Hecho: auto-update especifico del cliente Windows con asset `.exe` de GitHub Releases.
- Decision actual: mantener tareas programadas para reporte y actualizacion; no hace falta un servicio residente.
- RAM y CPU quedan fuera del alcance inicial; Proxmox ya cubre ese diagnostico cuando hay algo raro.

## V2 - PBS

- Hecho: detectar tareas PBS de sincronizacion remota y garbage collection, guardar el ultimo estado por datastore/remoto y mostrarlo en dashboard, `/servers/pbs`, detalle PBS, alertas y correo.
- Hecho: generar alerta critica si falla una sincronizacion remota o un garbage collection; usa las mismas supresiones y modo mantenimiento del resto de alertas PBS.

## V2 - Servidores genericos no Proxmox

- Disenar primero el modelo de datos para que Windows no quede como caso cerrado y pueda servir a otros servidores en el futuro.
- Mantener el alcance pequeno: heartbeat, identidad del host, discos y alertas basicas.
- Evitar monitorizar CPU/RAM/procesos salvo que aparezca una necesidad real.

## QOL - Produccion

- Hecho: pantalla "Checklist produccion" en `/settings/system`.
- Hecho: muestra HTTPS detectado, cookie segura, 2FA, URL publica, version, correo y retencion.
- Hecho: permite declarar acceso exclusivo por VPN privada para aceptar HTTP dentro de NetBird/WireGuard sin ocultar las limitaciones del navegador.

## Seguridad y endurecimiento

### Alta prioridad

- Hecho: Go 1.26.5, `chi` 5.2.2 y proteccion de origen de `net/http`.
- Hecho: cabeceras de proxy aceptadas solo desde `TRUSTED_PROXY_CIDRS`.
- Hecho: acciones GitHub fijadas por SHA, CI sin permiso de escritura y actualizador obligado a verificar `SHA256SUMS`.
- Pendiente: valorar aprobacion de entorno para publicar y firmas verificables de los binarios.
- Pendiente: ejecutar el servidor con un usuario dedicado y endurecer systemd (`NoNewPrivileges`, `ProtectSystem`, `PrivateTmp` y permisos minimos).
- Hecho: rechazar valores conocidos de ejemplo para `SESSION_KEY`.
- Pendiente: proteger API keys con hash o cifrado, y cifrar SMTP/TOTP con una clave fuera de la base de datos.
- Hecho: permisos `0600` para SQLite/WAL/SHM y ACL explicita para `C:\ProgramData\Probakgo`.

### Media prioridad

- Hecho: limitar cuerpos JSON de API a 8 MiB y encolar el recalculo de alertas.
- Pendiente: fijar SRI o servir localmente Bootstrap e iconos; eliminar `unsafe-inline` de la CSP mediante nonces o hashes.
- Hecho: validar estrictamente Host y URL publica al generar comandos de instalacion; un host publico requiere una URL publica configurada.
- Hecho: limitar metadatos, checksums y binarios descargados durante actualizaciones y descargas web.

### Corregido

- Hecho: operaciones sensibles por POST con proteccion de origen/2FA y sin codigos TOTP en URLs.
- Hecho: evitar inyeccion de formulas en exportaciones CSV.
- Hecho: unificar politica de contrasenas y revocar sesiones tras cambios de contrasena, 2FA, rol o estado.
- Hecho: responder con `Cache-Control: no-store` en contenido autenticado y sensible.

## Comprobaciones manuales antes de produccion

- Probar en HTTPS: login, 2FA, suprimir/reactivar alerta, crear API key, actualizar cliente, heartbeat y correo.
- Confirmar `SESSION_SECURE=true`, firewall y acceso solo por HTTPS o NetBird segun el despliegue.
- Confirmar que el backup de la VM incluye `probakgo_data.db` y `.env`.
- Vigilar que la retencion, la paginacion del historial y los correos no generen ruido repetido.
- Probar instalaciones limpias y actualizaciones desde una version anterior en PVE, PBS y Windows.
- Confirmar que cada release contiene los tres binarios y `SHA256SUMS`.
