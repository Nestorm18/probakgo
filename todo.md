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

## V2 - Servidores genericos no Proxmox

- Disenar primero el modelo de datos para que Windows no quede como caso cerrado y pueda servir a otros servidores en el futuro.
- Mantener el alcance pequeno: heartbeat, identidad del host, discos y alertas basicas.
- Evitar monitorizar CPU/RAM/procesos salvo que aparezca una necesidad real.

## QOL - Produccion

- Pendiente: pantalla "Checklist produccion" en `/about` o `/settings/system`.
- Debe mostrar en verde/rojo: HTTPS detectado, `SESSION_SECURE=true`, 2FA obligatorio para no readers, 2FA requerido en operaciones delicadas, URL publica configurada, version actual, ultimo envio de correo OK y estado basico de retencion.
- Debe servir como revision rapida antes de exponer la aplicacion o despues de actualizarla.


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