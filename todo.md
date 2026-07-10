# TODO - Probakgo

## V2 - Windows Server

- Hecho: agente Windows separado en `client-windows/` con instalador simple, `.env`, API key, heartbeat/report cada 5 minutos y logs locales.
- Hecho: monitorizar estado de discos fisicos/logicos, salud SMART si esta disponible y espacio libre.
- Hecho: monitorizar hostname, IP publica/local, version del agente y ultimo heartbeat.
- Hecho: vista web propia para Windows con columnas centradas en discos.
- Hecho: alertas basicas de heartbeat, disco y SMART.
- Hecho: comando `doctor` y logs rotados a diario en `C:\ProgramData\Probakgo` conservando 7 dias.
- Hecho: auto-update especifico del cliente Windows con asset `.exe` de GitHub Releases.
- Pendiente: comprobar si conviene instalarlo como servicio ademas de tarea programada.
- RAM y CPU quedan fuera del alcance inicial; Proxmox ya cubre ese diagnostico cuando hay algo raro.
- Hecho: alerta de volumen no detectado desde el ultimo reporte.
- Hecho: `doctor` comprueba conectividad real con Probakgo/API key, MachineGuid, discos via WMI/PowerShell y tarea programada.

## V2 - PBS

- Hecho: detectar tareas PBS de sincronizacion remota y garbage collection, guardar el ultimo estado por datastore/remoto y mostrarlo en dashboard, `/servers/pbs`, detalle PBS, alertas y correo.
- Hecho: generar alerta critica si falla una sincronizacion remota o un garbage collection; usa las mismas supresiones y modo mantenimiento del resto de alertas PBS.
- Ejemplos a reconocer en logs/tareas PBS: `Sync remote 'carnicas-pbs-casa' datastore 'synology' successful`, `Sync remote 'anteva' datastore 'bembibre' failed`, `Garbage Collect Datastore 'synology' successful`.

## V2 - Servidores genericos no Proxmox

- Disenar primero el modelo de datos para que Windows no quede como caso cerrado y pueda servir a otros servidores en el futuro.
- Mantener el alcance pequeno: heartbeat, identidad del host, discos y alertas basicas.
- Evitar monitorizar CPU/RAM/procesos salvo que aparezca una necesidad real.

## QOL - Produccion

- Hecho: pantalla "Checklist produccion" en `/settings/system`.
- Debe mostrar en verde/rojo: HTTPS detectado, `SESSION_SECURE=true`, 2FA obligatorio para no readers, 2FA requerido en operaciones delicadas, URL publica configurada, version actual, ultimo envio de correo OK y estado basico de retencion.
- Debe servir como revision rapida antes de exponer la aplicacion o despues de actualizarla.

## Seguridad y endurecimiento

### Alta prioridad

- Hecho: actualizar la cadena de compilacion a Go 1.26.5, actualizar `github.com/go-chi/chi/v5` a 5.2.2 y sustituir `gorilla/csrf` por `net/http.CrossOriginProtection`.
- Hecho: aceptar `X-Forwarded-For`, `X-Real-IP` y cabeceras de proxy solo desde `TRUSTED_PROXY_CIDRS`; un cliente directo ya no puede falsear su IP.
- Pendiente: terminar de endurecer releases: fijar acciones GitHub por SHA, aprobacion de entorno para publicar y firmas verificables de los binarios. Hecho: CI sin permiso de escritura y actualizador obligado a verificar `SHA256SUMS`.
- Pendiente: ejecutar el servidor con un usuario dedicado sin privilegios y endurecer systemd (`NoNewPrivileges`, `ProtectSystem`, `PrivateTmp` y permisos minimos).
- Hecho: rechazar valores conocidos de ejemplo para `SESSION_KEY`; el ejemplo ya no sugiere una clave publica reutilizable.
- Pendiente: proteger secretos persistidos con API keys solo con hash o cifrado, y cifrado de SMTP/TOTP con una clave fuera de la base de datos. Hecho: permisos `0600` para SQLite/WAL/SHM y ACL explicita para `C:\ProgramData\Probakgo`.

### Media prioridad

- Hecho: limitar cuerpos JSON de API a 8 MiB y encolar el recalculo de alertas para que un agente no pueda crear trabajo ilimitado.
- Pendiente: fijar SRI o servir localmente Bootstrap e iconos; eliminar `unsafe-inline` de la CSP mediante nonces o hashes.
- Pendiente: validar estrictamente Host y URL publica al generar comandos de instalacion, sin usar cabeceras Host no confiables.
- Pendiente: revisar limites de timeout y tamano de descarga en actualizaciones y proxy.

### Corregido en esta tanda

- Hecho: convertir operaciones sensibles por GET a POST con proteccion de origen/2FA y evitar codigos TOTP en URLs.
- Hecho: evitar inyeccion de formulas en exportaciones CSV.
- Hecho: unificar politica de contrasenas y revocar sesiones tras cambios de contrasena, 2FA, rol o estado.
- Hecho: responder con `Cache-Control: no-store` en contenido autenticado y sensible.


Ahora mismo, para lo que querías inicialmente, está bastante completo: alertas de backups, heartbeat, PBS/PVE/Windows, 2FA, auditoría, histórico, supresiones, doctor, updates, vistas móviles, exports y detalle de alerta.
Lo que aún miraría antes de producción:
Probar flujo real completo en HTTPS:
Login, 2FA, suprimir/reactivar alerta, crear API key, actualizar cliente, heartbeat y correo.

Revisar permisos del servidor:
SESSION_SECURE=true, cookies seguras, puerto solo por HTTPS/NetBird si aplica, firewall correcto.

Confirmar backups de la propia app:
Ya haces backup de la VM, suficiente. Solo comprobar que incluye probakgo_data.db y .env.

Retención y ruido:
Ver si el historial de alertas crece bien con paginación y si los correos no repiten alertas suprimidas.

Instaladores/clientes:
Probar Linux PVE, Linux PBS y Windows desde cero con una API key nueva, y también update desde una versión anterior.

Release:
Crear tag nuevo único, confirmar que GitHub Actions genera los 3 binarios:
probakgo_linux_amd64, probakgo-client_linux_amd64, probakgo-windows-client_windows_amd64.exe.
