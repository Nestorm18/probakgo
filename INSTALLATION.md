# Guía de instalación

Esta guía cubre el servidor Probakgo, los clientes Proxmox y Windows, el acceso HTTPS/VPN y las tareas automáticas que instala cada binario.

## 1. Servidor

### Requisitos

- Linux x86-64.
- Puerto `36748` disponible, o el indicado en `API_PORT`.
- Acceso `root` durante el primer arranque si quieres que el binario instale systemd y el cron de actualización.
- No necesita un servicio SQLite externo ni CGO.

### Descargar y arrancar

```bash
install -d -m 0755 /opt/probakgo
wget https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo_linux_amd64 \
  -O /opt/probakgo/probakgo
chmod +x /opt/probakgo/probakgo
cd /opt/probakgo
./probakgo
```

En el primer arranque:

1. Genera una `SESSION_KEY` aleatoria y la guarda con permisos `0600` en `.env`.
2. Genera `DATA_ENCRYPTION_KEY` y la guarda también en `.env`.
3. Aplica todas las migraciones embebidas a `probakgo_data.db`.
4. Cifra las API keys existentes, la contraseña SMTP y los secretos TOTP antes de continuar.
5. Crea el administrador `probakgo` con una contraseña aleatoria de 16 caracteres en `.initial-admin-password`, con permisos `0600`.
6. Si se ejecuta como `root` desde `/opt/probakgo`, crea el usuario de sistema `probakgo` e instala una unidad systemd endurecida.
7. Si se ejecuta como `root`, crea `/etc/cron.d/probakgo` con un minuto estable por máquina dentro de la hora de la 01:00.

La contraseña inicial no se escribe en journald. Recupérala una sola vez desde otra terminal; el comando elimina el archivo al leerlo:

```bash
/opt/probakgo/probakgo initial-password
```

Abre `http://<ip-servidor>:36748`, inicia sesión y cambia la contraseña inmediatamente.

### Servicio y logs

```bash
systemctl status probakgo
systemctl restart probakgo
journalctl -u probakgo -f
```

El servicio se crea en el primer arranque, pero el proceso lanzado manualmente sigue ocupando el puerto. Termínalo antes de iniciar la unidad si fuera necesario.

En la ruta estándar, la unidad usa `User=probakgo`, `NoNewPrivileges`, `PrivateTmp`, `ProtectSystem=strict`, restricciones de dispositivos/kernel y acceso de escritura limitado a `/opt/probakgo`. El binario y ese directorio pertenecen al usuario de servicio para permitir el auto-update sin ejecutar la aplicación como `root`.

### Diagnóstico

```bash
/opt/probakgo/probakgo doctor
```

`doctor` comprueba:

- versión y variables básicas;
- presencia y validez de `DATA_ENCRYPTION_KEY` frente a los secretos cifrados;
- dirección de escucha y `SESSION_SECURE`;
- apertura de SQLite y migraciones;
- presencia de administradores activos y estado 2FA;
- URL pública y requisito TOTP para acciones sensibles;
- unidad systemd y cron de actualización.

Solo devuelve error por comprobaciones `FAIL`; los puntos revisables aparecen como `WARN`.

### Variables del servidor

El binario busca `.env` en el directorio de trabajo y junto al ejecutable. Puedes partir de `.env.server.example`.

```env
API_HOST=0.0.0.0
API_PORT=36748
DATABASE_PATH=./probakgo_data.db
TIMEZONE=Europe/Madrid
SESSION_SECURE=false

# Se genera automáticamente si falta:
# SESSION_KEY=<secreto-de-al-menos-32-bytes>
# DATA_ENCRYPTION_KEY=<secreto-de-al-menos-32-bytes>

# Solo el proxy indicado podrá aportar X-Forwarded-*:
# TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128

# Orígenes completos adicionales, separados por comas:
# CSRF_TRUSTED_ORIGINS=https://monitor.example

# Barra de depuración:
# DEV=true

# Releases privadas:
# GITHUB_TOKEN=ghp_...
```

La aplicación rechaza puertos inválidos, zonas horarias inexistentes, claves de sesión o cifrado menores de 32 bytes, el secreto público de ejemplo y CIDR de proxy inválidos. Conserva `.env` junto con las copias de SQLite: sin `DATA_ENCRYPTION_KEY` no se pueden recuperar API keys, SMTP, Telegram ni TOTP.

### Ajustes desde la web

Las páginas administrativas están bajo **Configuración**:

- **Sistema**: URL usada en comandos de instalación, declaración de acceso exclusivo por VPN, política 2FA y checklist de producción.
- **Email**: SMTP, destinatarios, hora del informe diario y alertas críticas inmediatas.
- **Telegram**: bot global, usuarios vinculados y prueba de alertas críticas inmediatas.
- **Mantenimiento**: retención automática y descarga segura de una copia SQLite.
- **Alertas**: umbrales globales de disco PVE/PBS, disco Windows, backup fallido y heartbeat.
- **IPs baneadas**: desbloqueo de accesos.
- **Audit log**: historial de cambios administrativos.
- **Reiniciar BD**: reinicio de datos PVE/PBS/Windows y configuración operativa con doble confirmación; se preservan usuarios, audit log e historial de migraciones.

Los overrides y el modo mantenimiento de cada servidor se editan desde las listas de PVE, PBS y Windows. Los overrides por VM se configuran en PVE.

## 2. HTTPS o VPN

### Proxy inverso nginx

Ejemplo con TLS terminado en nginx:

```nginx
server {
    listen 80;
    server_name monitor.example;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name monitor.example;

    ssl_certificate     /etc/letsencrypt/live/monitor.example/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/monitor.example/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:36748;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Configura el servidor así y reinícialo:

```env
API_HOST=127.0.0.1
SESSION_SECURE=true
TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128
```

Probakgo ignora `X-Forwarded-*` si la conexión no llega desde un CIDR autorizado. `CSRF_TRUSTED_ORIGINS` solo es necesario para orígenes adicionales; debe incluir esquema y host completos.

### Acceso privado por VPN

Si el panel solo es accesible mediante NetBird, WireGuard u otra VPN cifrada y no está publicado en el router:

1. configura una URL interna/VPN en **Configuración → Sistema**;
2. marca **Acceso exclusivo por VPN privada**;
3. mantén el firewall cerrado a redes no autorizadas.

La casilla informa al checklist; no crea reglas de firewall ni cifra por sí sola el tráfico. Con HTTP, `SESSION_SECURE=false` es necesario para que el navegador envíe la cookie. Con HTTPS debe ser `true`.

### PWA y notificaciones push

Probakgo se puede instalar como PWA desde Chrome, Edge y otros navegadores compatibles. Esto crea un acceso en el escritorio, el menú Inicio o la pantalla principal y permite abrir el panel en una ventana independiente. No instala otro binario ni duplica el servidor.

Para activar las notificaciones:

1. Publica Probakgo mediante HTTPS; los navegadores solo permiten service workers y Web Push en contextos seguros, con la excepción de `localhost`.
2. Inicia sesión y abre **Perfil → Notificaciones en el escritorio**.
3. Pulsa **Notificaciones del escritorio** y acepta el permiso del navegador.
4. Usa la opción **Instalar aplicación** del navegador si quieres un acceso independiente.

Cada suscripción queda vinculada al usuario y al navegador que la creó. Las claves VAPID se generan al activar la primera suscripción y la clave privada se cifra con `DATA_ENCRYPTION_KEY`. Al desactivar o eliminar un usuario se eliminan sus suscripciones.

Las notificaciones informan de alertas críticas y de su resolución; al pulsarlas se abre el servidor o la vista de alertas correspondiente. La PWA no ofrece funcionamiento offline ni cachea informes, porque Probakgo debe mostrar siempre el estado actual del servidor.

### Notificaciones de Telegram

Telegram funciona como un canal independiente de email y Web Push y utiliza una vinculación personal por cuenta:

1. Crea un bot dedicado con `@BotFather` y copia su token.
2. Como administrador, abre **Configuración → Telegram**, pega el token, guarda y activa el canal.
3. Cada usuario abre **Mi perfil → Mi Telegram**. Desde el móvil pulsa **Abrir bot**; desde un ordenador escanea el QR con el móvil. Después inicia la conversación con **Start**.
4. El mismo usuario vuelve al navegador que generó el enlace o QR y pulsa **Detectar vinculación**; después puede enviarse una prueba.
5. El administrador puede revisar o retirar vinculaciones desde **Configuración → Telegram** y desde la edición del usuario.

Cada usuario de Probakgo puede asociar un único chat privado y un mismo chat de Telegram no puede pertenecer a dos usuarios. Las cuentas inactivas no reciben avisos; al eliminar una cuenta se elimina automáticamente su vinculación. Cada usuario conserva su propio estado de entrega, por lo que un fallo no genera duplicados para los demás. No se admiten grupos ni canales en las vinculaciones personales.

El token se cifra con `DATA_ENCRYPTION_KEY` y nunca vuelve a mostrarse. Probakgo no expone un webhook: la vinculación consulta temporalmente las actualizaciones pendientes del bot, por lo que debe utilizarse un bot dedicado sin webhook. Telegram recibe el nombre del servidor y el detalle de la alerta; tenlo en cuenta al habilitar este canal.

## 3. Usuarios, 2FA y claves

### Roles

| Rol | Acceso |
|---|---|
| `reader` | Dashboard, alertas, servidores e históricos |
| `editor` | Lectura y edición de configuración de backups/alertas permitida por la UI |
| `admin` | Usuarios, API keys, configuración, auditoría y mantenimiento |

Cada usuario puede activar TOTP desde **Perfil**. El administrador puede:

- exigir 2FA a `editor` y `admin`, con 3 días de gracia desde el primer aviso;
- exigir una confirmación TOTP reciente para operaciones sensibles.

Una confirmación sensible es válida durante 10 minutos. Los cambios de contraseña, rol, estado o 2FA invalidan las sesiones existentes.

Si un usuario pierde el segundo factor y tienes acceso al servidor:

```bash
/opt/probakgo/probakgo unlock2fa <usuario>
```

### API keys

Todas las claves de clientes comienzan por `pbk-`. Al crear una:

- el **Hostname del servidor** es obligatorio y debe coincidir con el reportado por el cliente;
- el alias visible y la URL de Proxmox son opcionales;
- la clave se vincula al Machine ID del primer equipo que la usa;
- si reinstalas el host y cambia su identidad, usa **Desvincular** antes de reutilizar la clave.

La pantalla de creación muestra comandos preparados para Linux/Proxmox y Windows. Una clave existente se puede revelar como administrador tras validar la contraseña y, si la política lo exige, TOTP.

SQLite no guarda las claves `pbk-` en claro: conserva una copia cifrada para el revelado autorizado y un hash HMAC para autenticar peticiones sin búsquedas por texto plano.

## 4. Cliente Proxmox

### Requisitos

- Proxmox VE 7+ o Proxmox Backup Server 2+.
- `root`.
- Acceso de red al servidor Probakgo.
- `systemd` para el heartbeat automático.

### Instalar

```bash
wget https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo-client_linux_amd64 \
  -O /tmp/probakgo-client
chmod +x /tmp/probakgo-client

/tmp/probakgo-client install \
  --api-url http://<ip-servidor>:36748 \
  --api-key pbk-...
```

Opciones útiles:

```text
--proxmox-token <token>     usar un token existente
--proxmox-secret <secret>   usar su secreto
--github-token <token>      releases privadas
--replace-env               reemplazar .env en una reinstalación
```

En una reinstalación, `.env` se conserva por defecto y solo se actualizan los valores no vacíos indicados. Usa `--replace-env` para generarlo de nuevo.

### Qué instala

- `/opt/probakgo/probakgo-client`
- `/usr/local/bin/probakgo-client` como enlace simbólico
- `/opt/probakgo/.env` con permisos `0600`
- `/opt/probakgo/vzdump_client.sh`
- `/var/log/probakgo/`
- `/etc/logrotate.d/probakgo`
- `/etc/cron.d/probakgo-client`
- `probakgo-client-heartbeat.service` y `.timer`

El token Proxmox se genera como `root@pam!probakgo-client` cuando no se proporciona:

- PVE: token sin separación de privilegios para consultar tareas, storages y backups.
- PBS: token con rol `Audit` en `/`.

En PVE:

- registra `script: /opt/probakgo/vzdump_client.sh` en `/etc/vzdump.conf`;
- el hook envía un reporte al terminar el job;
- intenta importar VMs y días esperados desde `/cluster/backup`;
- envía un reporte inicial.

En PBS:

- no instala hook `vzdump`;
- añade un reporte diario a las 06:00.

En ambos:

- actualiza a diario en un minuto estable por máquina durante la hora de la 01:00;
- envía heartbeat cada 5 minutos;
- rota los logs diariamente, comprime y conserva 7 rotaciones.

### Verificar y usar

```bash
probakgo-client doctor
probakgo-client heartbeat
probakgo-client version
```

Forzar reporte PVE:

```bash
probakgo-client --vzdump-hook
```

Forzar reporte PBS:

```bash
probakgo-client
```

Otros flags:

```bash
probakgo-client --server-type pve --vzdump-hook
probakgo-client --file reporte.json --server-type pve
probakgo-client --debug --debug-api-calls --vzdump-hook
```

`--debug-api-calls` guarda respuestas crudas de Proxmox en `debug/`.

### Configuración del cliente

```env
API_URL=http://probakgo.example:36748
API_KEY=pbk-...
PROXMOX_TOKEN=root@pam!probakgo-client
PROXMOX_SECRET=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
PROXMOX_VERIFY_TLS=false

# PROXMOX_CA_BUNDLE=/ruta/ca.pem
# SERVER_TYPE=pve
# DEBUG_MODE=true
# DEBUG_API_CALLS=true
# GITHUB_TOKEN=ghp_...
```

`PROXMOX_VERIFY_TLS=false` afecta a la conexión del cliente con la API de Proxmox, no a la conexión con Probakgo.

### Actualizar o desinstalar

```bash
probakgo-client update
probakgo-client uninstall
```

`uninstall` requiere `root` y:

- retira el hook de `/etc/vzdump.conf`;
- revoca el token Proxmox;
- desactiva/elimina el timer;
- elimina cron, logrotate, enlace, `/opt/probakgo` y `/var/log/probakgo`.

## 5. Cliente Windows

### Requisitos e instalación

Abre PowerShell como administrador:

```powershell
Invoke-WebRequest `
  -Uri "https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo-windows-client_windows_amd64.exe" `
  -OutFile "$env:TEMP\probakgo-windows-client.exe"

& "$env:TEMP\probakgo-windows-client.exe" install `
  --api-url http://<ip-servidor>:36748 `
  --api-key pbk-...
```

El instalador:

1. crea `C:\ProgramData\Probakgo`;
2. restringe las ACL a `SYSTEM` y administradores;
3. instala `probakgo-windows-client.exe`;
4. escribe `.env` con `API_URL` y `API_KEY`;
5. crea `Probakgo Windows Report` cada 5 minutos;
6. crea `Probakgo Windows Update` a diario a las 04:17.

Las tareas se ejecutan como `SYSTEM`. Una reinstalación detiene ambas tareas y reintenta hasta 30 segundos si Windows mantiene ocupado el ejecutable.

### Verificar y operar

```powershell
C:\ProgramData\Probakgo\probakgo-windows-client.exe doctor
C:\ProgramData\Probakgo\probakgo-windows-client.exe heartbeat
C:\ProgramData\Probakgo\probakgo-windows-client.exe version
C:\ProgramData\Probakgo\probakgo-windows-client.exe update
```

Ejecutar el binario sin subcomando envía un reporte completo:

```powershell
C:\ProgramData\Probakgo\probakgo-windows-client.exe
```

El cliente reporta hostname, IP local/pública, versión, MachineGuid, volúmenes fijos y salud física best-effort mediante PowerShell/WMI.

### Logs y tareas

```powershell
Get-Content C:\ProgramData\Probakgo\probakgo-windows-client.log -Tail 100
schtasks /Query /TN "Probakgo Windows Report"
schtasks /Query /TN "Probakgo Windows Update"
```

Al cambiar el día, el log activo se archiva como `probakgo-windows-client-YYYY-MM-DD.log`; solo se conservan los últimos 7 días.

## 6. Actualizaciones privadas

Para un repositorio privado, crea un token GitHub de corta duración con permiso **Contents: Read-only** sobre `Nestorm18/probakgo`.

- Servidor: guarda `GITHUB_TOKEN` en `/opt/probakgo/.env`.
- Proxmox: instala con `--github-token`; queda guardado en `/opt/probakgo/.env`.
- Windows: añade `GITHUB_TOKEN=...` manualmente a `C:\ProgramData\Probakgo\.env` si necesita auto-update privado.
- La pantalla de API key acepta temporalmente el token para generar comandos de descarga; no lo guarda en SQLite.

El actualizador descarga la release correspondiente y valida el binario con `SHA256SUMS`. El workflow de publicación genera además una attestation firmada de procedencia para cada binario.

## 7. Resolución de problemas

### API y conectividad

```bash
curl http://<ip-servidor>:36748/api/health
probakgo-client doctor
probakgo-client --debug --vzdump-hook
```

`/api/health` devuelve `200` solo cuando SQLite y el esquema migrado se pueden leer; devuelve `503` si el servicio está vivo pero la base de datos no está disponible.

Comprueba que:

- `API_URL` no termina en una ruta extra;
- `API_KEY` empieza por `pbk-` y está activa;
- el hostname de la key coincide con el host;
- la key no está vinculada a otro Machine ID.

### Heartbeat Proxmox

```bash
systemctl status probakgo-client-heartbeat.timer --no-pager
systemctl list-timers --all --no-pager probakgo-client-heartbeat.timer
journalctl -u probakgo-client-heartbeat.service -n 100 --no-pager
```

Si el timer está activo pero sin próxima ejecución:

```bash
systemctl stop probakgo-client-heartbeat.timer
systemctl reset-failed probakgo-client-heartbeat.timer probakgo-client-heartbeat.service
systemctl daemon-reload
systemctl start probakgo-client-heartbeat.service
systemctl start probakgo-client-heartbeat.timer
```

### Hook PVE

```bash
grep probakgo /etc/vzdump.conf
tail -f /var/log/probakgo/hook.log
probakgo-client --vzdump-hook
```

### Formularios 403

La web usa protección de origen. Accede siempre por un origen estable. Si hay un proxy:

- configura `TRUSTED_PROXY_CIDRS`;
- usa un `Host` coherente;
- añade a `CSRF_TRUSTED_ORIGINS` solo orígenes adicionales completos.

### Sesiones perdidas

```bash
grep '^SESSION_KEY=' /opt/probakgo/.env
```

Si falta, detén el servicio, inicia una vez el binario desde `/opt/probakgo` para generarla y vuelve a arrancar systemd.

### Backup antes de cambios

Descarga una copia desde **Configuración → Mantenimiento** o copia con el servicio detenido:

```bash
systemctl stop probakgo
cp /opt/probakgo/probakgo_data.db /ruta/segura/probakgo_data.db
cp /opt/probakgo/.env /ruta/segura/probakgo.env
systemctl start probakgo
```

La copia de `.env` es imprescindible para conservar sesiones y descifrar API keys, SMTP, Telegram y TOTP; también conserva el acceso a releases privadas y otros secretos de despliegue.
