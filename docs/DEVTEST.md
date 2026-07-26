# Guía de pruebas reales

Despliegue de laboratorio para validar el flujo completo: servidor, clientes, reportes, heartbeat, alertas, email y actualización.

## Entorno recomendado

- Una VM Linux x86-64 para el servidor.
- Un nodo PVE 7+.
- Opcional: un PBS 2+.
- Opcional: una máquina Windows.
- Acceso `root`/administrador y conectividad al puerto `36748`.
- Go 1.26.5 en la máquina de desarrollo.

No uses credenciales ni bases de datos de producción.

## 1. Comprobaciones locales

Desde la raíz del repositorio:

```powershell
go build ./...
go vet ./...
go test ./...
```

Comprueba también la versión única:

```powershell
Select-String -Path internal\version\version.go -Pattern 'var Version'
```

## 2. Compilar desde Windows

```powershell
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

go build -o probakgo .
go build -o probakgo-client ./client/

$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -o probakgo-windows-client.exe ./client-windows/

Remove-Item Env:GOOS
Remove-Item Env:GOARCH
Remove-Item Env:CGO_ENABLED
```

Sube los binarios:

```powershell
scp probakgo root@<ip-vm-servidor>:/tmp/probakgo
scp probakgo-client root@<ip-pve>:/tmp/probakgo-client
# Opcional:
scp probakgo-client root@<ip-pbs>:/tmp/probakgo-client
```

El cliente Windows se prueba directamente en Windows.

## 3. Servidor

En la VM:

```bash
install -d -m 0755 /opt/probakgo
mv /tmp/probakgo /opt/probakgo/probakgo
chmod +x /opt/probakgo/probakgo
cd /opt/probakgo
./probakgo
```

El primer arranque como `root`:

- guarda `SESSION_KEY` en `.env`;
- guarda `DATA_ENCRYPTION_KEY` en `.env`;
- crea/migra `probakgo_data.db`;
- crea el usuario `probakgo`;
- guarda la contraseña inicial en un archivo `0600`, sin escribirla en el log;
- instala `probakgo.service` con el usuario de sistema `probakgo` y el cron de update con jitter.

No existe una contraseña inicial fija. Recupérala una sola vez:

```bash
/opt/probakgo/probakgo initial-password
```

Inicia sesión en:

```text
http://<ip-vm-servidor>:36748
```

Cambia la contraseña, activa 2FA en **Perfil** y ejecuta:

```bash
/opt/probakgo/probakgo doctor
systemctl status probakgo
systemctl cat probakgo
```

La unidad debe incluir `User=probakgo`, `NoNewPrivileges=true`, `PrivateTmp=true` y `ProtectSystem=strict`.

Si el proceso manual sigue ejecutándose, termínalo antes de arrancar el servicio para evitar que ambos usen el mismo puerto.

## 4. Configuración inicial en la web

Como administrador:

1. Abre **Configuración → Sistema**.
2. Configura la URL pública/interna que deben usar los clientes.
3. Si el laboratorio solo entra por VPN, marca esa opción; no la uses para un puerto publicado.
4. Activa 2FA para acciones sensibles si quieres probar el flujo completo.
5. Revisa el checklist de producción.
6. En **Configuración → Alertas**, define umbrales fáciles de provocar.
7. Opcional: configura SMTP y envía una prueba.

## 5. Crear API keys

Crea una key distinta por equipo en **API Keys → Nueva API Key**.

- **Hostname del servidor** debe coincidir exactamente con el hostname que enviará el cliente.
- **Alias visible** es opcional.
- **URL Proxmox** es opcional.
- Copia la clave `pbk-`; no existen claves administrativas `adm-`.

La key se enlaza al Machine ID en el primer heartbeat/reporte. Para reutilizarla tras reinstalar un host, desvincúlala desde la web.

## 6. Instalar en PVE

```bash
ssh root@<ip-pve>
chmod +x /tmp/probakgo-client

/tmp/probakgo-client install \
  --api-url http://<ip-vm-servidor>:36748 \
  --api-key pbk-...
```

Comprueba:

```bash
probakgo-client doctor
systemctl status probakgo-client-heartbeat.timer --no-pager
grep probakgo /etc/vzdump.conf
tail -n 50 /var/log/probakgo/hook.log
```

La instalación debe:

- generar el token Proxmox;
- crear `/opt/probakgo/.env`;
- registrar el hook;
- instalar el heartbeat cada 5 minutos;
- instalar update en un minuto estable por host dentro de la hora de la 01:00;
- importar VMs/días esperados desde los jobs activos;
- enviar un primer reporte.

### Reporte manual

```bash
probakgo-client --vzdump-hook
```

### Backup real

```bash
vzdump 100 --storage <storage-backup> --mode snapshot
```

Al terminar el job, valida en la web:

- estado y duración;
- tareas del último job por VM;
- fichero y tamaño cuando Proxmox permite emparejarlos;
- storages, swap y heartbeat;
- historial;
- VMs esperadas, ausentes y desconocidas.

## 7. Instalar en PBS

```bash
ssh root@<ip-pbs>
chmod +x /tmp/probakgo-client

/tmp/probakgo-client install \
  --api-url http://<ip-vm-servidor>:36748 \
  --api-key pbk-...
```

Comprueba:

```bash
probakgo-client doctor
probakgo-client
cat /etc/cron.d/probakgo-client
```

El cron debe contener update a `1:<minuto-jitter>` y reporte a las 06:00. En la web revisa:

- datastores y tendencia;
- grupos/snapshots y verificación;
- montaje y estimación de llenado;
- último sync remoto y garbage collection si la API los expone;
- heartbeat y swap.

Un snapshot antiguo retenido no debe generar por sí solo una alerta. Sí deben hacerlo, según configuración, un reporte PBS atrasado, verificación fallida, disco/llenado o fallo de sync/GC.

## 8. Instalar en Windows

PowerShell como administrador:

```powershell
.\probakgo-windows-client.exe install `
  --api-url http://<ip-vm-servidor>:36748 `
  --api-key pbk-...
```

Verifica:

```powershell
C:\ProgramData\Probakgo\probakgo-windows-client.exe doctor
schtasks /Query /TN "Probakgo Windows Report"
schtasks /Query /TN "Probakgo Windows Update"
Get-Content C:\ProgramData\Probakgo\probakgo-windows-client.log -Tail 80
```

Fuerza heartbeat y reporte:

```powershell
C:\ProgramData\Probakgo\probakgo-windows-client.exe heartbeat
C:\ProgramData\Probakgo\probakgo-windows-client.exe
```

Valida:

- tarea de reporte cada 5 minutos;
- tarea de update diaria a las 04:17;
- MachineGuid, IPs y versión;
- volúmenes fijos y salud best-effort;
- alertas de disco, heartbeat, salud y volumen desaparecido;
- override de disco y modo mantenimiento del servidor.

CPU y RAM no deben aparecer: están fuera del alcance actual.

## 9. Fixtures sin Proxmox

Los fixtures actuales se cargan con dos claves y sus cabeceras Machine ID:

```bash
curl -fS -X POST http://localhost:36748/api/report/pve \
  -H "Authorization: Bearer pbk-CLAVE-PVE" \
  -H "X-Machine-ID: 11223344-5566-7788-99aa-bbccddeeff00" \
  -H "Content-Type: application/json" \
  --data-binary @testdata/fixture_pve.json

curl -fS -X POST http://localhost:36748/api/report/pbs \
  -H "Authorization: Bearer pbk-CLAVE-PBS" \
  -H "X-Machine-ID: aabbccdd-eeff-0011-2233-445566778899" \
  -H "Content-Type: application/json" \
  --data-binary @testdata/fixture_pbs.json
```

La forma abreviada equivalente usa el script actualizado:

```bash
bash testdata/seed.sh http://localhost:36748 pbk-CLAVE-PVE pbk-CLAVE-PBS
```

Después puedes ejecutar `go run testdata/seed_history.go`. Consulta [testdata/README.md](../testdata/README.md).

## 10. Pruebas de alertas

Prueba al menos:

1. Baja temporalmente un umbral de disco.
2. Detén un heartbeat durante más tiempo que el umbral.
3. Genera o carga un backup fallido.
4. Marca una VM como esperada y omítela del último job.
5. Activa swap en un host de laboratorio.
6. Fuerza un estado Windows no saludable o simula un volumen ausente.
7. En PBS, usa un fixture/task fallido para sync o GC.

Para cada caso:

- aparece severidad y detalle correctos;
- CSV/JSON contiene la alerta;
- supresión oculta el aviso durante el plazo;
- modo mantenimiento oculta todas las alertas de ese servidor;
- al resolverla, queda el evento histórico;
- si SMTP crítico está activo, se envía una vez al aparecer y otra al resolverse.

La página usa polling para badges/notificaciones; prueba también sonido y permisos del navegador si forman parte del despliegue.

## 11. Seguridad y roles

Valida con un usuario de cada rol:

| Acción | reader | editor | admin |
|---|---:|---:|---:|
| Ver dashboard/servidores/alertas | Sí | Sí | Sí |
| Editar backup config | No | Sí | Sí |
| Editar alertas por servidor | No | Sí | Sí |
| Gestionar usuarios/API keys/settings | No | No | Sí |

Además:

- login con y sin TOTP;
- plazo de 3 días cuando se fuerza 2FA a no-readers;
- confirmación TOTP para acciones sensibles;
- revocación de sesión tras cambios de seguridad;
- revelado de API key con contraseña y TOTP;
- bloqueo progresivo de IP y desbloqueo administrativo;
- entradas del audit log sin secretos.
- ausencia de errores CSP en la consola: cada script debe llevar nonce y los recursos CDN deben validar SRI.

## 12. Base de datos

En la VM:

```bash
cd /opt/probakgo
sqlite3 probakgo_data.db ".tables"
sqlite3 probakgo_data.db \
  "SELECT name, applied_at FROM schema_migrations ORDER BY name;"
sqlite3 probakgo_data.db \
  "SELECT name, key_type, is_active, machine_id, last_used FROM api_keys;"
sqlite3 probakgo_data.db \
  "SELECT server_type, server_id, last_seen_at FROM server_heartbeats;"
```

La última migración actual es `037_secret_storage.up.sql`. Hay números repetidos (`012` y `013`), por lo que el identificador real es el nombre completo del archivo.

Comprueba que los secretos no están en claro:

```bash
sqlite3 probakgo_data.db \
  "SELECT key LIKE 'enc:v1:%', length(key_hash) FROM api_keys;"
sqlite3 probakgo_data.db \
  "SELECT smtp_password LIKE 'enc:v1:%' FROM email_config WHERE smtp_password <> '';"
sqlite3 probakgo_data.db \
  "SELECT totp_secret LIKE 'enc:v1:%' FROM users WHERE totp_secret <> '';"
```

Antes de probar una actualización:

```bash
systemctl stop probakgo
cp probakgo_data.db probakgo_data.before-test.db
cp .env probakgo.before-test.env
systemctl start probakgo
```

## 13. Actualización

Con una release de laboratorio válida:

```bash
/opt/probakgo/probakgo update
probakgo-client update
```

Windows:

```powershell
C:\ProgramData\Probakgo\probakgo-windows-client.exe update
```

Verifica versión, `SHA256SUMS`, attestation (`gh attestation verify`), reinicio del servidor y permanencia de `.env`, timers y tareas.

## 14. Limpieza segura

PVE/PBS:

```bash
probakgo-client uninstall
```

Windows no tiene subcomando `uninstall`; elimina primero las tareas desde PowerShell como administrador y conserva una copia de logs si la necesitas:

```powershell
schtasks /Delete /TN "Probakgo Windows Report" /F
schtasks /Delete /TN "Probakgo Windows Update" /F
```

Después retira `C:\ProgramData\Probakgo` mediante el procedimiento habitual del laboratorio.

En el servidor, usa **Configuración → Mantenimiento** para descargar una copia y **Configuración → Reiniciar BD** para reiniciar datos operativos conservando usuarios, audit log y migraciones. No borres a mano una base mientras el servicio está activo.

## Criterio de aceptación

La prueba se considera completa cuando:

- los tres tipos de servidor aparecen con identidad y heartbeat correctos;
- los reportes se actualizan por sus mecanismos automáticos;
- alertas, supresión, mantenimiento, historial y email funcionan;
- roles, 2FA, claves e IP bans respetan la política;
- exports y copia SQLite se descargan;
- `doctor`, tests y actualización no muestran fallos críticos.
