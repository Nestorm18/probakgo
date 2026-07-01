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
