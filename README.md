# probakgo

Monitor de infraestructura Proxmox - binario único, SQLite embebido, sin dependencias externas.

[![Go 1.22+](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![License: PolyForm NC](https://img.shields.io/badge/License-PolyForm_NC_1.0-blue)](LICENSE)

Monitoriza nodos **Proxmox VE** y **Proxmox Backup Server**, muestra el estado de los backups en un dashboard web y envía informes diarios por email. Alertas configurables por servidor y por VM con supresión de falsos positivos.

---

## Características

- Dashboard con estado en tiempo real de todos los nodos, storages y backups
- Detección de backups fallidos, VMs sin backup, discos casi llenos y snapshots obsoletos
- Alertas por servidor y por VM con umbrales personalizables y supresión temporal
- Historial de jobs de backup con detalle por VM y gráfico de duración
- Seguimiento de snapshots PBS: verificación, estimación de llenado y staleness configurable
- Informe diario por email con resumen de problemas (STARTTLS, compatible con Gmail / Office 365)
- Heartbeat cada 5 minutos para detectar nodos PVE offline
- Autoconfiguracion de VMs esperadas desde los jobs de backup de Proxmox (`/cluster/backup`)
- Control de acceso por roles: `reader`, `editor`, `admin`
- API keys con prefijo por tipo (`pbk-` clientes, `adm-` API externa)
- Auto-actualización del servidor y del cliente vía GitHub Releases
- Sin Docker, sin agentes externos - un binario con todo embebido

## Arquitectura

```
probakgo-client  ── POST /api/report/pve ──>  probakgo
probakgo-client  ── POST /api/report/pbs ──>  probakgo
                                                  |
                                             SQLite DB
                                                  |
                                 Dashboard web (puerto 36748)
```

El cliente corre en cada nodo Proxmox como hook de vzdump (se ejecuta al terminar cada backup). El servidor centraliza todos los datos y sirve la interfaz web desde el mismo binario.

---

## Instalación rápida

### Servidor

```bash
mkdir -p /opt/probakgo && cd /opt/probakgo
wget https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo_linux_amd64 -O probakgo
chmod +x probakgo
./probakgo
```

Abre `http://localhost:36748`. El usuario inicial es **probakgo** y la contraseña se genera automáticamente en el primer arranque; revisa los logs del servidor y cámbiala inmediatamente.

El primer arranque como root instala automáticamente el servicio systemd y el cron de auto-actualización.

Para diagnosticar el servidor:

```bash
/opt/probakgo/probakgo doctor
```

El comando revisa configuración, base de datos, migraciones, URL pública, `SESSION_SECURE`, administradores con 2FA, servicio systemd y cron de actualización.

### Cliente (en cada nodo Proxmox)

```bash
# 1. Crea una API key de tipo pbk- en la web UI (API Keys → Nueva)
# 2. En el nodo Proxmox, como root:
wget https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo-client_linux_amd64 -O /tmp/probakgo-client
chmod +x /tmp/probakgo-client
/tmp/probakgo-client install --api-url http://tu-servidor:36748 --api-key pbk-...
```

El nodo aparece en el dashboard en segundos. Para múltiples nodos, repite el proceso creando una API key por nodo.

Para diagnosticar una instalación:

```bash
/opt/probakgo/probakgo-client doctor
```

El comando revisa `.env`, API key, conexión con Probakgo, conexión con Proxmox, hook de vzdump y timer de heartbeat.

Ver [INSTALLATION.md](INSTALLATION.md) para la guía completa: proxy nginx, HTTPS, múltiples nodos, TLS personalizado.
Ver [RELEASES.md](RELEASES.md) para publicar versiones y binarios.

---

## Configuración

### Servidor (`.env`)

| Variable | Por defecto | Descripción |
|---|---|---|
| `SESSION_KEY` | *(auto)* | Clave de sesión - se genera y persiste automáticamente en el primer arranque |
| `API_HOST` | `0.0.0.0` | Interfaz de escucha |
| `API_PORT` | `36748` | Puerto |
| `DATABASE_PATH` | `probakgo_data.db` | Ruta al archivo SQLite |
| `TIMEZONE` | `Europe/Madrid` | Zona horaria para el scheduler de email |
| `SESSION_SECURE` | `false` | Activar a `true` cuando hay un proxy HTTPS delante |
| `CSRF_TRUSTED_ORIGINS` | - | Orígenes CSRF adicionales separados por comas (`host:puerto`) |
| `DEV` | `false` | Activa la barra de debug en el navegador |

El email (SMTP, destinatarios, hora de envío) y los umbrales de alerta se configuran desde la interfaz web en **Ajustes**.

### Cliente (`/opt/probakgo/.env`)

| Variable | Descripción |
|---|---|
| `API_URL` | URL completa del servidor (`http://ip:36748`) |
| `API_KEY` | API key `pbk-` creada en la web UI |
| `PROXMOX_TOKEN` | Token API de Proxmox - generado automáticamente por `install` |
| `PROXMOX_SECRET` | Secret del token |
| `PROXMOX_VERIFY_TLS` | `false` para certificados auto-firmados (habitual en Proxmox) |
| `PROXMOX_CA_BUNDLE` | Ruta a CA personalizada (opcional) |

---

## Roles de usuario

| Rol | Acceso |
|---|---|
| `reader` | Dashboard, servidores y backups - solo lectura |
| `editor` | reader + configuración de backup y alertas por servidor/VM |
| `admin` | Acceso completo: usuarios, API keys, ajustes de email, alertas e IP bans |

---

## Compilar desde código fuente

Requiere Go 1.22+. Sin CGO - compila en cualquier plataforma.

```bash
# Servidor
go build -o probakgo .

# Cliente
go build -o probakgo-client ./client/

# Cross-compile para Linux desde Windows (PowerShell)
$env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"
go build -o probakgo .
go build -o probakgo-client ./client/
```

Para inyectar la versión en el binario:

```bash
CGO_ENABLED=0 go build -ldflags "-X main.version=1.0.0" -o probakgo .
CGO_ENABLED=0 go build -ldflags "-X main.version=1.0.0" -o probakgo-client ./client/
```

---

## Documentación

| Documento | Descripción |
|---|---|
| [INSTALLATION.md](INSTALLATION.md) | Guía completa: systemd, nginx, múltiples nodos, resolución de problemas |
| [RELEASES.md](RELEASES.md) | Checklist de versionado, binarios, publicación y rollback |
| [docs/DEVTEST.md](docs/DEVTEST.md) | Entorno de pruebas end-to-end desde Windows |

---

## Licencia

[PolyForm Noncommercial 1.0.0](LICENSE) - uso libre para cualquier fin no comercial. No está permitido vender el software, ofrecerlo como servicio de pago ni redistribuirlo como producto propio.
