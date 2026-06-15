# Guía de instalación

## Servidor

### Requisitos

- Linux x86-64
- Puerto 36748 disponible (configurable con `API_PORT`)
- Sin dependencias externas en runtime (SQLite CGO-free incluido)

### 1. Descargar y ejecutar

```bash
wget https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo_linux_amd64 -O /opt/probakgo/probakgo
chmod +x /opt/probakgo/probakgo
cd /opt/probakgo
./probakgo
```

En el **primer arranque como root**, el binario configura automáticamente:

- **SESSION_KEY** - genera una clave aleatoria y la persiste en `.env`
- **Servicio systemd** - instala y habilita `probakgo.service`
- **Cron de auto-actualización** - instala `/etc/cron.d/probakgo` (01:00 diario)

### 2. Acceso inicial

Abre `http://<ip-servidor>:36748` en el navegador.

- **Usuario:** `probakgo`
- **Contraseña:** se genera automáticamente en el primer arranque y aparece en los logs del servidor - **cambiar inmediatamente**

### 3. Arrancar como servicio

Tras el primer arranque el servicio ya está instalado y habilitado. Para gestionarlo:

```bash
systemctl status probakgo
systemctl restart probakgo
journalctl -u probakgo -f
```

### Configuración opcional (`.env`)

El binario crea `.env` automáticamente con la `SESSION_KEY`. Puedes añadir variables adicionales:

```env
API_PORT=36748           # puerto (default: 36748)
TIMEZONE=Europe/Madrid   # zona horaria del scheduler de email
SESSION_SECURE=true      # activar si hay un proxy HTTPS delante
DATABASE_PATH=./probakgo_data.db
# GITHUB_TOKEN=ghp_...  # token GitHub para auto-actualización (necesario si el repo es privado)
```

### Proxy inverso nginx con HTTPS (opcional)

```nginx
server {
    listen 80;
    server_name monitor.tudominio.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name monitor.tudominio.com;

    ssl_certificate     /etc/letsencrypt/live/monitor.tudominio.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/monitor.tudominio.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:36748;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Con nginx delante, añade `SESSION_SECURE=true` al `.env` y reinicia el servicio.

### Actualización manual

```bash
/opt/probakgo/probakgo update
# Descarga el binario más reciente y hace `systemctl restart probakgo`
```

---

## Cliente (nodos Proxmox)

El cliente se instala en cada nodo PVE o PBS que quieras monitorizar. Se ejecuta tras cada backup via hook de vzdump.

### Requisitos

- Nodo Proxmox VE 7+ o Proxmox Backup Server 2+
- Acceso root al nodo
- Conectividad de red hacia el servidor probakgo

### 1. Crear una API key en el servidor

En la interfaz web: **API Keys → Nueva API Key**

- **Nombre:** identifica el nodo (ej. `pve-01`)
- **Nombre de servidor** *(opcional)*: alias corto que aparece en el dashboard (ej. `pve-01`)
- **URL Proxmox** *(opcional)*: URL completa de la interfaz Proxmox (ej. `https://192.168.1.10:8006`). Permite abrir el panel Proxmox directamente desde el dashboard.

Tras crear la key, la interfaz muestra los **3 pasos de instalación con los datos precargados** — puedes copiarlos directamente. La key solo se muestra una vez.

### 2. Instalar en el nodo

```bash
# En el nodo Proxmox, como root:
wget https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo-client_linux_amd64 -O /tmp/probakgo-client
chmod +x /tmp/probakgo-client

/tmp/probakgo-client install \
  --api-url http://<ip-servidor>:36748 \
  --api-key pbk-<tu-clave> \
  --github-token ghp-<tu-token>   # opcional, necesario si el repo es privado
```

El subcomando `install` hace automáticamente:

1. Copia el binario a `/opt/probakgo/probakgo-client`
2. Detecta el tipo de nodo (PVE o PBS) desde `/etc/issue`
3. Genera un token API de Proxmox via `pveum` (PVE) o `proxmox-backup-manager` (PBS)
4. Escribe la configuración en `/opt/probakgo/.env`
5. Registra el hook vzdump en `/etc/vzdump.conf`
6. Configura logrotate en `/etc/logrotate.d/probakgo-client`
7. Instala cron de auto-actualización en `/etc/cron.d/probakgo-client` (01:00 diario)
8. Instala `probakgo-client-heartbeat.timer` para enviar heartbeat cada 5 minutos
9. En PVE, lee `/cluster/backup` para autoconfigurar las VMs esperadas segun los jobs activos

### 3. Verificar

```bash
/opt/probakgo/probakgo-client doctor
/opt/probakgo/probakgo-client --vzdump-hook
```

El nodo deberia aparecer en el dashboard del servidor en pocos segundos. `doctor` comprueba `.env`, API key, conexion con Probakgo, conexion con Proxmox, hook de vzdump y timer de heartbeat.

### Desinstalar el cliente

```bash
/opt/probakgo/probakgo-client uninstall
```

El subcomando `uninstall` (requiere root) deshace la instalación completamente:

1. Elimina la línea del hook en `/etc/vzdump.conf`
2. Revoca el token API de Proxmox (`pveum` en PVE, `proxmox-backup-manager` en PBS)
3. Elimina `/etc/cron.d/probakgo-client` y `/etc/logrotate.d/probakgo`
4. Deshabilita y elimina `probakgo-client-heartbeat.timer` y `.service`
5. Elimina los directorios `/opt/probakgo/` y `/var/log/probakgo/`

> Si el binario ya no está en `/opt/probakgo/`, copia una versión reciente al nodo y ejecútala con `uninstall` antes de eliminarla.

### Múltiples nodos

Repite los pasos 1-2 para cada nodo. Se recomienda crear una API key por nodo para poder revocarlas individualmente.

### Configuración manual (`/opt/probakgo/.env`)

El instalador genera este archivo automáticamente. Si necesitas ajustarlo:

```env
API_URL=http://tu-servidor:36748
API_KEY=pbk-...
PROXMOX_TOKEN=root@pam!probakgo-client
PROXMOX_SECRET=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
PROXMOX_VERIFY_TLS=false          # false para certificados auto-firmados (habitual en Proxmox)
# PROXMOX_CA_BUNDLE=/ruta/a/ca.pem
# GITHUB_TOKEN=ghp_...            # token GitHub para auto-actualización (necesario si el repo es privado)
```

### Forzar tipo de servidor

Si la detección automática falla:

```bash
/opt/probakgo/probakgo-client --server-type pve --vzdump-hook
# o --server-type pbs
```

### Reiniciar la base de datos

En **Configuración → Reiniciar BD** puedes borrar todos los datos monitorizados (reportes, servidores, alertas, API keys) manteniendo los usuarios. Requiere confirmación con contraseña dos veces. Útil para empezar desde cero sin reinstalar.

---

## Resolución de problemas

### Diagnostico rapido del cliente

```bash
/opt/probakgo/probakgo-client doctor
```

Si el heartbeat manual funciona pero la web lo marca offline, revisa especificamente:

```bash
systemctl status probakgo-client-heartbeat.timer --no-pager
systemctl list-timers --all | grep probakgo
journalctl -u probakgo-client-heartbeat.service -n 100 --no-pager
```

Si el timer aparece activo pero sin proxima ejecucion (`Trigger: n/a`), rearmalo:

```bash
systemctl stop probakgo-client-heartbeat.timer
systemctl reset-failed probakgo-client-heartbeat.timer probakgo-client-heartbeat.service
systemctl daemon-reload
systemctl start probakgo-client-heartbeat.service
systemctl start probakgo-client-heartbeat.timer
```

### El cliente no conecta con el servidor

```bash
curl http://<ip-servidor>:36748/api/health
/opt/probakgo/probakgo-client --debug --vzdump-hook
```

### Formularios del dashboard devuelven 403

Las sesiones tienen protección CSRF. Asegúrate de acceder siempre por el mismo dominio/IP.

### El servidor no retiene las sesiones tras reiniciar

La `SESSION_KEY` debería haberse guardado en `.env` automáticamente en el primer arranque. Comprueba:

```bash
grep SESSION_KEY /opt/probakgo/.env
```

Si no está, añádela manualmente:

```bash
echo "SESSION_KEY=$(openssl rand -hex 32)" >> /opt/probakgo/.env
systemctl restart probakgo
```

### Logs del cliente

```bash
cat /var/log/probakgo-client.log
/opt/probakgo/probakgo-client --debug --debug-api-calls --vzdump-hook
```
