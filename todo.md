# TODO - Probakgo

## V2 - Windows Server / sistemas no Proxmox

- Agente para Windows Server o similar usando la misma filosofia que el cliente actual: instalador simple, `.env`, API key, heartbeat y auto-update.
- Objetivo principal: monitorizar estado de discos fisicos/logicos, salud SMART si esta disponible y espacio libre.
- Monitorizar hostname, IP publica/local, version del agente y ultimo heartbeat.
- RAM y CPU quedan fuera del alcance inicial; Proxmox ya cubre ese diagnostico cuando hay algo raro.
- Enviar alertas si no hay heartbeat, si un disco falla, si SMART avisa o si un volumen se queda sin espacio.
- Vista web separada o unificada por tipo de cliente para distinguir PVE, PBS y Windows/servidor generico.
- Instalacion como servicio de Windows, con logs locales y comando `doctor` equivalente para diagnostico.
- Disenar primero el modelo de datos para que tambien sirva a otros servidores no Proxmox en el futuro.
