# Guía de pruebas reales (devtest)

Cómo desplegar **probakgo** en tu entorno local de VMs + Proxmox para validar el flujo end-to-end real (servidor + cliente + backup real → dashboard).

---

## Setup necesario

- 1 VM Linux x86-64 para el servidor probakgo (Debian/Ubuntu mínimo, 1GB RAM, 2GB disco)
- 1 nodo Proxmox VE 7+ accesible por red desde la VM del servidor (puede ser virtualizado o físico)
- Acceso root SSH al nodo Proxmox
- Conectividad de red entre la VM servidor y el Proxmox en el puerto 36748

---

## 1. Compilar desde Windows (cross-compile para Linux AMD64)

Si tu máquina de desarrollo es Windows, compila los binarios para Linux antes de subirlos:

```powershell
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

# Servidor
go build -o probakgo .

# Cliente
go build -o probakgo-client ./client/

# Restaurar entorno tras compilar
Remove-Item Env:GOOS
Remove-Item Env:GOARCH
Remove-Item Env:CGO_ENABLED
```

Subir los binarios por SCP (disponible en Windows 10/11 por defecto):

```powershell
# Servidor → VM Debian
scp probakgo root@192.168.10.222:/tmp/probakgo

# Cliente → nodo Proxmox
scp probakgo-client root@192.168.10.230:/tmp/probakgo-client
```

En la VM Debian, crear el directorio, mover y dar permisos:

```bash
mkdir -p /opt/probakgo
mv /tmp/probakgo /opt/probakgo/probakgo
chmod +x /opt/probakgo/probakgo
```

---

## 2. Servidor

### Arrancar

Con el binario ya movido a `/opt/probakgo/` (sección 1), arráncalo:

```bash
cd /opt/probakgo
./probakgo
```

En el primer arranque (como root) auto-instala:
- `SESSION_KEY` aleatoria persistida en `.env`
- Servicio systemd `probakgo.service`
- Cron de auto-update en `/etc/cron.d/probakgo` (01:00 diario)

### Acceder a la web UI

```
http://<ip-vm-servidor>:36748
```

- Usuario: `probakgo`
- Contraseña: `admin123`
- **Cambia la contraseña inmediatamente** desde Usuarios → editar contraseña

En los logs del primer arranque aparece también una API key `adm-...` (preview). La clave completa la consigues entrando a la web → API Keys.

### Arrancar como servicio

```bash
systemctl status probakgo
systemctl restart probakgo
journalctl -u probakgo -f         # tail de logs en vivo
```

---

## 3. Crear API key para el cliente

En la web UI:

1. Ir a **API Keys → Nueva API Key**
2. **Tipo:** `server (pbk-)` - cliente Proxmox
3. **Nombre:** identifica el nodo, ej. `pve-lab`
4. Copiar la clave generada - **sólo se muestra una vez**

---

## 4. Cliente en el nodo Proxmox

### Subir el cliente al nodo

El binario `probakgo-client` ya está en `/tmp/` del nodo Proxmox (subido en la sección 1).

### Instalar en el nodo

```bash
ssh root@<ip-proxmox>

/tmp/probakgo-client install \
  --api-url http://<ip-vm-servidor>:36748 \
  --api-key pbk-<clave-creada-arriba>
```

El subcomando `install` hace automáticamente:

1. Copia el binario a `/opt/probakgo/probakgo-client`
2. Detecta el tipo de nodo (PVE o PBS) desde `/etc/issue`
3. Genera un token API de Proxmox con `pveum` (PVE) o `proxmox-backup-manager` (PBS)
4. Escribe `/opt/probakgo/.env`
5. Registra el hook vzdump en `/etc/vzdump.conf`
6. Configura logrotate
7. Instala cron de auto-update en `/etc/cron.d/probakgo-client`

---

## 5. Verificar que llegan datos

### Envío manual (sin esperar a un backup real)

```bash
# En el nodo Proxmox
/opt/probakgo/probakgo-client --vzdump-hook
```

Debería aparecer el nodo en el dashboard del servidor en segundos.

### Backup real para ver el flujo completo

Lanza un backup desde la web de Proxmox o por consola:

```bash
# Backup de la VM 100, por ejemplo
vzdump 100 --storage <tu-storage-backup> --mode snapshot
```

Cuando termine, el hook vzdump invoca automáticamente al cliente y envía el reporte. Verás en el dashboard:
- Status del último backup (OK/error)
- Duración
- Storages disponibles + %used
- Listado de VMs/CTs con sus backups

---

## 6. Inspeccionar qué datos manda el cliente

### Modo debug (recomendado para la primera prueba)

```bash
/opt/probakgo/probakgo-client --debug --debug-api-calls --vzdump-hook
```

- `--debug` - log verbose en stdout
- `--debug-api-calls` - guarda respuestas crudas de la API de Proxmox en `debug/`

### Logs persistentes del cliente

```bash
tail -f /var/log/probakgo-client.log
```

### Estructura de los datos (referencia)

| Campo PVE | Descripción |
|---|---|
| `hostname` | nombre del nodo |
| `ip_address` / `public_ip` | IP local y pública |
| `machine_id` | binding de seguridad (de `/etc/machine-id`) |
| `client_version` | versión de probakgo-client |
| `last_backup_status` | status, starttime, endtime, duration del último vzdump |
| `storages[]` | tipo, path, content, capacidad, %used, prune_backups |
| `storages[].content_data[]` | VMs/CTs con vmid, formato, size, ctime, notas |

| Campo PBS | Descripción |
|---|---|
| `hostname` / `ip_address` / `machine_id` | igual que PVE |
| `datastores[]` | name, total, used, avail, %used |

---

## 7. Inspeccionar la BD del servidor

```bash
# En la VM servidor
sqlite3 probakgo_data.db ".tables"

sqlite3 probakgo_data.db "SELECT hostname, last_backup_status, reported_at FROM pve_reports ORDER BY reported_at DESC LIMIT 5;"

sqlite3 probakgo_data.db "SELECT * FROM api_keys;"
```

---

## Resolución de problemas

### El cliente no conecta con el servidor

```bash
# Comprobar que el servidor responde
curl http://<ip-vm-servidor>:36748/api/health

# Ejecutar cliente con debug
/opt/probakgo/probakgo-client --debug --vzdump-hook
```

### El nodo no aparece en el dashboard

```bash
# Verificar API key activa
sqlite3 probakgo_data.db "SELECT name, key_type, is_active, last_used FROM api_keys WHERE key_type='server';"

# Ver logs del servidor
journalctl -u probakgo -n 50
```

### El hook vzdump no se ejecuta

```bash
# Comprobar que está registrado
grep probakgo /etc/vzdump.conf

# Comprobar permisos
ls -la /opt/probakgo/probakgo-client
```

### Limpieza para repetir la prueba

```bash
# En el nodo Proxmox - desinstalar cliente
rm -rf /opt/probakgo
sed -i '/probakgo/d' /etc/vzdump.conf
rm -f /etc/cron.d/probakgo-client /etc/logrotate.d/probakgo-client

# En la VM servidor - reset completo
systemctl stop probakgo
rm probakgo_data.db .env
./probakgo                # vuelve a generar SESSION_KEY y crear el admin
```

---

## Notas de seguridad

- No uses credenciales de producción en estas VMs de prueba
- La API key `adm-...` del primer arranque permite acceso completo vía API - trátala como secreto
- Cambia `admin123` antes de cualquier prueba realista
- Si la VM servidor está expuesta fuera del lab, configura `SESSION_SECURE=true` y un proxy HTTPS delante (ver `INSTALLATION.md`)
