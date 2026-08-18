# probakgo

Monitor de copias de seguridad e infraestructura para Proxmox VE, Proxmox Backup Server y Windows.

[![Go 1.26.5](https://img.shields.io/badge/Go-1.26.5-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: PolyForm NC](https://img.shields.io/badge/License-PolyForm_NC_1.0-blue)](LICENSE)

El servidor se distribuye como un único binario con la web y las migraciones embebidas, usa SQLite sin CGO y escucha por defecto en el puerto `36748`. Los clientes envían reportes autenticados con claves `pbk-` vinculadas al Machine ID del primer equipo que las usa.

## Funcionalidad actual

### Monitorización

- Dashboard unificado para PVE, PBS y Windows, con estado, último reporte, heartbeat, IP local/pública y versión del cliente.
- PVE: storages, swap, último job `vzdump`, detalle por VM/CT, tamaño/duración/fichero e historial de jobs.
- Configuración de backups esperados por VM y día; el instalador PVE intenta importarla desde `/cluster/backup`.
- PBS: datastores, uso, tendencia y fecha estimada de llenado, montaje, grupos/snapshots, verificación, garbage collection y sincronizaciones remotas.
- Windows: volúmenes lógicos, espacio usado/libre, salud física best-effort, heartbeat e historial.
- Exportaciones CSV/JSON de alertas, servidores PVE/PBS e históricos.

### Alertas y avisos

- Disco, backup fallido o pequeño, VM esperada ausente, VM desconocida, reporte/heartbeat atrasado y swap activa.
- PBS: llenado estimado, verificación fallida y fallos de sync/garbage collection. La antigüedad de snapshots retenidos es informativa y no genera alerta.
- Windows: disco, heartbeat, salud de disco y volumen desaparecido respecto al reporte anterior.
- Umbrales globales y overrides por servidor; PVE añade overrides por VM.
- Supresión temporal por alerta y modo mantenimiento por servidor.
- Estado e historial de alertas, badges en vivo, sonido/notificaciones opcionales del navegador.
- Informe diario por SMTP y avisos inmediatos opcionales por email, Web Push y Telegram al aparecer o resolverse alertas críticas; Telegram vincula un chat privado por usuario activo.

### Aplicación web instalable (PWA)

Probakgo se puede instalar desde un navegador compatible para abrirlo desde el escritorio, el menú Inicio o la pantalla principal del móvil con una ventana independiente. No es un cliente nativo separado: sigue utilizando la misma web y el mismo servidor.

La PWA se utiliza principalmente para:

- abrir Probakgo como una aplicación instalada, sin la interfaz habitual del navegador;
- recibir notificaciones del sistema cuando aparece o se resuelve una alerta crítica, aunque la pestaña esté cerrada;
- entrar directamente en la alerta o el servidor correspondiente al pulsar la notificación.

Las notificaciones se activan por usuario y navegador desde **Perfil → Notificaciones en el escritorio**. Requieren HTTPS, salvo en `localhost`, y cada dispositivo debe aceptar el permiso del navegador. La aplicación no cachea informes ni ofrece un modo offline: el service worker evita almacenar la interfaz para que el estado mostrado proceda siempre del servidor.

### Administración y seguridad

- Roles `reader`, `editor` y `admin`.
- 2FA TOTP por usuario, política opcional para exigirlo a editores/administradores y confirmación TOTP para acciones sensibles.
- Revocación de sesiones al cambiar contraseña, rol, estado o 2FA.
- Protección CSRF/origen, límites de peticiones, cabeceras de seguridad y confianza explícita de proxies.
- CSP con nonce por petición para scripts y SRI en Bootstrap, Bootstrap Icons y Chart.js.
- Bloqueo progresivo de IP tras intentos de login fallidos y gestión de baneos desde la web.
- Audit log de cambios administrativos.
- API keys, credenciales SMTP, tokens de Telegram y secretos TOTP cifrados en SQLite mediante una clave externa.
- Checklist de producción para HTTPS/VPN, cookie segura, 2FA, URL pública, email y retención.
- Retención automática, descarga de una copia SQLite y reinicio operativo que preserva usuarios, auditoría y migraciones.
- Servicio systemd endurecido y ejecutado como usuario dedicado `probakgo` en la instalación estándar.
- Auto-update verificado mediante `SHA256SUMS`; las releases publican procedencia firmada verificable.

CPU y RAM del cliente Windows están fuera del alcance actual.

## Arquitectura

```text
PVE vzdump hook ───────────────┐
PBS report cron (06:00) ───────┼─ probakgo-client ── POST /api/report/{pve|pbs}
PVE/PBS heartbeat (cada 5 min) ┘                  └─ POST /api/heartbeat

Windows task (cada 5 min) ─────── probakgo-windows-client
                                      ├─ POST /api/report/windows
                                      └─ POST /api/heartbeat

                                      probakgo :36748
                                      ├─ REST API /api/*
                                      ├─ Web UI /
                                      └─ SQLite + migraciones embebidas
```

## Instalación rápida

### Servidor Linux

```bash
mkdir -p /opt/probakgo
wget https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo_linux_amd64 \
  -O /opt/probakgo/probakgo
chmod +x /opt/probakgo/probakgo
cd /opt/probakgo
./probakgo
```

En el primer arranque:

- se genera y guarda `SESSION_KEY` en `.env`;
- se genera `DATA_ENCRYPTION_KEY` y se cifran API keys, SMTP, Telegram y TOTP antes de persistirlos;
- se crea el administrador `probakgo` y su contraseña aleatoria queda en un archivo `0600`, nunca en logs;
- si se ejecuta como `root` desde `/opt/probakgo`, se instala un servicio endurecido con usuario dedicado y un auto-update diario repartido durante la hora de la 01:00.

En otra terminal, recupera una sola vez la contraseña, abre `http://<ip-servidor>:36748` y cámbiala:

```bash
/opt/probakgo/probakgo initial-password
```

Después ejecuta:

```bash
/opt/probakgo/probakgo doctor
```

### Cliente Proxmox

Crea una API key en **API Keys → Nueva API Key**. El hostname indicado debe coincidir con el que enviará el nodo.

```bash
wget https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo-client_linux_amd64 \
  -O /tmp/probakgo-client
chmod +x /tmp/probakgo-client
/tmp/probakgo-client install \
  --api-url http://<ip-servidor>:36748 \
  --api-key pbk-...
```

El instalador crea `/opt/probakgo/.env`, genera el token Proxmox si no se aporta, añade `/usr/local/bin/probakgo-client`, configura logs/update/heartbeat y:

- en PVE, registra el hook de `vzdump`, sincroniza la configuración esperada y envía un reporte inicial;
- en PBS, programa además un reporte diario a las 06:00.

Verificación:

```bash
probakgo-client doctor
probakgo-client sync-backups    # PVE: resincroniza VMID, nombre y días
probakgo-client --vzdump-hook   # PVE: fuerza un reporte
probakgo-client                 # PBS: envía un reporte
```

### Cliente Windows

Abre PowerShell como administrador:

```powershell
Invoke-WebRequest `
  -Uri "https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo-windows-client_windows_amd64.exe" `
  -OutFile "$env:TEMP\probakgo-windows-client.exe"

& "$env:TEMP\probakgo-windows-client.exe" install `
  --api-url http://<ip-servidor>:36748 `
  --api-key pbk-...
```

Se instala en `C:\ProgramData\Probakgo`, restringe sus ACL a `SYSTEM` y administradores, y crea:

- `Probakgo Windows Report`, cada 5 minutos;
- `Probakgo Windows Update`, a diario a las 04:17.

Ambas tareas se ejecutan como `SYSTEM`. Los logs rotan diariamente y conservan 7 días.

```powershell
C:\ProgramData\Probakgo\probakgo-windows-client.exe doctor
Get-Content C:\ProgramData\Probakgo\probakgo-windows-client.log -Tail 80
```

## Configuración

### Servidor `.env`

| Variable | Valor por defecto | Descripción |
|---|---|---|
| `API_HOST` | `0.0.0.0` | Dirección de escucha |
| `API_PORT` | `36748` | Puerto HTTP |
| `DATABASE_PATH` | `probakgo_data.db` | Ruta de SQLite |
| `SESSION_KEY` | generada | Mínimo 32 bytes; se persiste en el primer arranque |
| `DATA_ENCRYPTION_KEY` | generada | Mínimo 32 bytes; cifra API keys, SMTP, Telegram y TOTP. Debe respaldarse junto a SQLite |
| `TIMEZONE` | `Europe/Madrid` | Zona horaria del scheduler de email |
| `SESSION_SECURE` | `false` | Debe ser `true` cuando el panel se sirve por HTTPS |
| `TRUSTED_PROXY_CIDRS` | vacío | CIDR de proxies autorizados para `X-Forwarded-*` |
| `CSRF_TRUSTED_ORIGINS` | vacío | Orígenes completos adicionales, separados por comas |
| `DEV` | `false` | Activa la barra de depuración |
| `GITHUB_TOKEN` | vacío | Necesario para releases privadas |

La URL usada en los comandos de instalación y la declaración de acceso exclusivo por VPN se guardan desde **Configuración → Sistema**.

### Cliente Proxmox `/opt/probakgo/.env`

| Variable | Descripción |
|---|---|
| `API_URL` | URL del servidor Probakgo |
| `API_KEY` | Clave `pbk-` |
| `PROXMOX_TOKEN` / `PROXMOX_SECRET` | Credenciales API de Proxmox |
| `PROXMOX_VERIFY_TLS` | `false` para el certificado autofirmado habitual |
| `PROXMOX_CA_BUNDLE` | CA personalizada opcional |
| `SERVER_TYPE` | Override opcional: `pve` o `pbs` |
| `DEBUG_MODE` / `DEBUG_API_CALLS` | Depuración opcional |
| `GITHUB_TOKEN` | Token para releases privadas |

El cliente Windows solo guarda `API_URL` y `API_KEY` en `C:\ProgramData\Probakgo\.env`.

## Comandos

| Binario | Comandos |
|---|---|
| `probakgo` | `version`, `update`, `doctor`, `initial-password`, `unlock2fa <usuario>` |
| `probakgo-client` | `install`, `uninstall`, `update`, `heartbeat`, `sync-backups`, `doctor`, `version` |
| `probakgo-windows-client.exe` | reporte por defecto, `install`, `update`, `heartbeat`, `doctor`, `version` |

El modo reporte del cliente Proxmox acepta `--server-type`, `--vzdump-hook`, `--file`, `--debug` y `--debug-api-calls`.

## Compilar y probar

Requiere Go 1.26.5.

```bash
go build -o probakgo .
go build -o probakgo-client ./client/
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -o probakgo-windows-client.exe ./client-windows/
go vet ./...
go test ./...
```

La versión única vive en `internal/version/version.go` y se inyecta en los tres binarios durante la release. Los assets publicados son:

- `probakgo_linux_amd64`
- `probakgo-client_linux_amd64`
- `probakgo-windows-client_windows_amd64.exe`
- `SHA256SUMS`

CI exige al menos un 35 % de cobertura global. El workflow de release usa el entorno `release` y genera attestations firmadas para los tres binarios.

## Documentación

| Documento | Contenido |
|---|---|
| [INSTALLATION.md](INSTALLATION.md) | Instalación completa, proxy HTTPS, clientes, seguridad y troubleshooting |
| [RELEASES.md](RELEASES.md) | CI, publicación, comprobaciones y rollback |
| [docs/DEVTEST.md](docs/DEVTEST.md) | Prueba end-to-end en laboratorio |
| [testdata/README.md](testdata/README.md) | Fixtures PVE/PBS locales |

## Licencia

[PolyForm Noncommercial 1.0.0](LICENSE). Uso permitido para fines no comerciales según sus términos.
