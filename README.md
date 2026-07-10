# probakgo

Monitor de infraestructura Proxmox y Windows: binario unico, SQLite embebido y web UI en el mismo puerto.

[![Go 1.22+](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![License: PolyForm NC](https://img.shields.io/badge/License-PolyForm_NC_1.0-blue)](LICENSE)

Probakgo monitoriza **Proxmox VE**, **Proxmox Backup Server** y servidores **Windows**. Centraliza estado de backups, discos, heartbeat y alertas en una web simple con informes por email.

## Caracteristicas

- Dashboard con estado de PVE, PBS y Windows.
- Deteccion de backups fallidos, VMs sin backup, discos casi llenos y nodos sin heartbeat.
- Alertas globales, por servidor y por VM, con supresion temporal.
- Historial de jobs PVE con detalle por VM.
- PBS con datastores, verificacion, estimacion de llenado, estado de montaje y resultado de sincronizaciones remotas/garbage collection.
- Windows con discos, SMART basico, heartbeat y estado por servidor.
- Alertas Windows por heartbeat, disco lleno, salud de disco y volumen desaparecido.
- Informe diario por email.
- Auto-update del servidor y de los clientes via GitHub Releases.
- Control de acceso por roles: `reader`, `editor`, `admin`.

## Arquitectura

```text
probakgo-client          -> POST /api/report/pve      -> probakgo
probakgo-client          -> POST /api/report/pbs      -> probakgo
probakgo-windows-client  -> POST /api/report/windows  -> probakgo
probakgo-client          -> POST /api/heartbeat       -> probakgo
                                                        |
                                                     SQLite
                                                        |
                                                 Web UI :36748
```

El cliente Proxmox se ejecuta como hook de vzdump y con heartbeat systemd cada 5 minutos. El cliente Windows se instala como tarea programada cada 5 minutos.

## Instalacion rapida

### Servidor

```bash
mkdir -p /opt/probakgo && cd /opt/probakgo
wget https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo_linux_amd64 -O probakgo
chmod +x probakgo
./probakgo
```

Abre `http://localhost:36748`. El usuario inicial es `probakgo`; la contrasena se genera en el primer arranque y aparece en los logs del servidor.

Diagnostico:

```bash
/opt/probakgo/probakgo doctor
```

### Cliente Proxmox

```bash
# Crea una API key pbk- en la web UI: API Keys -> Nueva
wget https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo-client_linux_amd64 -O /tmp/probakgo-client
chmod +x /tmp/probakgo-client
/tmp/probakgo-client install --api-url http://tu-servidor:36748 --api-key pbk-...
```

Diagnostico:

```bash
/opt/probakgo/probakgo-client doctor
```

### Cliente Windows

En Windows, abre PowerShell como administrador:

```powershell
# Crea una API key pbk- en la web UI: API Keys -> Nueva
Invoke-WebRequest -Uri "https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo-windows-client_windows_amd64.exe" -OutFile "$env:TEMP\probakgo-windows-client.exe"
& "$env:TEMP\probakgo-windows-client.exe" install --api-url http://tu-servidor:36748 --api-key pbk-...
```

El instalador copia el binario a `C:\ProgramData\Probakgo`, escribe `.env`, crea la tarea programada `Probakgo Windows Report` cada 5 minutos y deja logs en `C:\ProgramData\Probakgo\probakgo-windows-client.log`. El log rota a diario como `probakgo-windows-client-YYYY-MM-DD.log` y conserva solo los ultimos 7 dias.

Diagnostico:

```powershell
C:\ProgramData\Probakgo\probakgo-windows-client.exe doctor
Get-Content C:\ProgramData\Probakgo\probakgo-windows-client.log -Tail 80
```

Actualizar cliente Windows:

```powershell
C:\ProgramData\Probakgo\probakgo-windows-client.exe update
```

## Configuracion

### Servidor `.env`

| Variable | Por defecto | Descripcion |
|---|---:|---|
| `SESSION_KEY` | auto | Clave de sesion generada en el primer arranque |
| `API_HOST` | `0.0.0.0` | Interfaz de escucha |
| `API_PORT` | `36748` | Puerto |
| `DATABASE_PATH` | `probakgo_data.db` | Ruta SQLite |
| `TIMEZONE` | `Europe/Madrid` | Zona horaria |
| `SESSION_SECURE` | `false` | Usar `true` si hay HTTPS delante |
| `TRUSTED_PROXY_CIDRS` | - | CIDR de nginx/proxy que puede enviar `X-Forwarded-*`; por ejemplo `127.0.0.1/32,::1/128` |
| `CSRF_TRUSTED_ORIGINS` | - | Origenes web adicionales con esquema completo, por ejemplo `https://monitor.example` |
| `DEV` | `false` | Barra debug |

### Cliente Proxmox `/opt/probakgo/.env`

| Variable | Descripcion |
|---|---|
| `API_URL` | URL del servidor Probakgo |
| `API_KEY` | API key `pbk-` |
| `PROXMOX_TOKEN` | Token API generado por `install` |
| `PROXMOX_SECRET` | Secret del token |
| `PROXMOX_VERIFY_TLS` | `false` para certificados auto-firmados |
| `PROXMOX_CA_BUNDLE` | CA personalizada opcional |

### Cliente Windows `C:\ProgramData\Probakgo\.env`

| Variable | Descripcion |
|---|---|
| `API_URL` | URL del servidor Probakgo |
| `API_KEY` | API key `pbk-` |

## Compilar desde codigo fuente

Requiere Go. Los binarios se compilan sin CGO.

```bash
# Servidor
go build -o probakgo .

# Cliente Proxmox
go build -o probakgo-client ./client/

# Cliente Windows
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o probakgo-windows-client.exe ./client-windows/
```

Inyectar version:

```bash
CGO_ENABLED=0 go build -ldflags "-X main.version=1.0.0" -o probakgo .
CGO_ENABLED=0 go build -ldflags "-X main.version=1.0.0" -o probakgo-client ./client/
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.version=1.0.0" -o probakgo-windows-client.exe ./client-windows/
```

## Documentacion

| Documento | Descripcion |
|---|---|
| [INSTALLATION.md](INSTALLATION.md) | Instalacion completa, nginx, HTTPS, clientes y troubleshooting |
| [RELEASES.md](RELEASES.md) | Checklist de publicacion y rollback |
| [docs/DEVTEST.md](docs/DEVTEST.md) | Pruebas end-to-end |

## Licencia

[PolyForm Noncommercial 1.0.0](LICENSE). Uso libre para fines no comerciales.
