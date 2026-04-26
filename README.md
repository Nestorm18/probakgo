# probakgo - Monitor de Proxmox

Sistema de monitoreo para Proxmox VE (PVE) y Proxmox Backup Server (PBS) con dashboard web e informes diarios por email.

```
probakgo-client  ──POST /report/pve──▶  probakgo (servidor)
probakgo-client  ──POST /report/pbs──▶  probakgo (servidor)
                                             │
                                        SQLite DB
                                             │
                              Web UI (navegador ← puerto 36748)
```

## Inicio rápido

### 1. Servidor

```bash
# Descargar binario (Linux amd64)
wget https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo_linux_amd64 -O probakgo
chmod +x probakgo

# Configurar
cp .env.server.example .env
# Editar .env: cambiar SESSION_KEY por una clave segura
# openssl rand -base64 32

./probakgo
```

Acceso: `http://localhost:36748` - usuario `probakgo` / contraseña `admin123` **(cambiar en producción)**

### 2. Cliente (en cada nodo Proxmox)

Primero crea una API key `pbk-` desde la interfaz web.

```bash
# Copiar binario al nodo
scp probakgo-client root@nodo-proxmox:/tmp/

# En el nodo - el binario se instala solo
/tmp/probakgo-client install --api-url http://tu-servidor:36748 --api-key pbk-...
```

El subcomando `install` (ejecutado como root):
- Copia el binario a `/opt/probakgo/`
- Genera token Proxmox automáticamente via `pveum` (PVE) o `proxmox-backup-manager` (PBS)
- Escribe `/opt/probakgo/.env`
- Registra el hook vzdump en `/etc/vzdump.conf`
- Configura logrotate
- Instala cron de auto-actualización a las 01:00 (`/etc/cron.d/probakgo-client`)

## Compilar desde código fuente

```bash
# Servidor
go build -o probakgo .
./probakgo

# Cliente
go build -o probakgo-client ./client/
```

## Configuración

### Servidor - `.env`

| Variable | Por defecto | Descripción |
|----------|-------------|-------------|
| `API_HOST` | `0.0.0.0` | Interfaz de escucha |
| `API_PORT` | `36748` | Puerto |
| `DATABASE_PATH` | `probakgo_data.db` | Ruta al archivo SQLite |
| `SESSION_KEY` | *(inseguro)* | Clave de sesión - **cambiar en producción** |
| `TIMEZONE` | `Europe/Madrid` | Zona horaria para el scheduler de email |

El email (SMTP, destinatarios, hora de envío) se configura desde la interfaz web en **Ajustes → Email**.

### Cliente - `/opt/probakgo/.env`

| Variable | Descripción |
|----------|-------------|
| `API_URL` | URL del servidor probakgo |
| `API_KEY` | API key `pbk-` creada en la interfaz web |
| `PROXMOX_TOKEN` | Token API de Proxmox (`usuario@realm!nombre`) |
| `PROXMOX_SECRET` | Secret del token Proxmox |
| `PROXMOX_VERIFY_TLS` | `false` para certificados auto-firmados (default Proxmox) |
| `PROXMOX_CA_BUNDLE` | Ruta a CA personalizada (opcional) |

## Roles de usuario

| Rol | Acceso |
|-----|--------|
| `reader` | Solo lectura: dashboard, servidores, backups |
| `editor` | reader + edición de configuración de backups |
| `admin` | Acceso completo: usuarios, API keys, email |

## Tipos de API key

| Prefijo | Uso |
|---------|-----|
| `pbk-` | Clientes Proxmox (agentes que envían reportes) |
| `app-` | Aplicaciones móviles u otras integraciones |
| `adm-` | Acceso admin a la API REST |

## Estructura del proyecto

```
main.go              - binario servidor: API + web en puerto 36748
client/              - binario cliente: se ejecuta en nodos Proxmox
internal/
  api/               - API REST (chi router, prefijo /api/)
  web/               - Interfaz web (chi router, gorilla/sessions)
  service/           - auth, report, email scheduler
  store/             - consultas SQLite
  db/migrations/     - migraciones SQL embebidas
  domain/            - modelos de dominio y esquemas API
  config/            - configuración desde env
  session/           - helpers de sesión (extraído para evitar ciclo de imports)
web/
  templates/         - plantillas html/template (embebidas)
  static/            - CSS/JS (embebidos)
```

## Documentación adicional

- [Instalación del servidor](docs/INSTALL_SERVER.md)
- [Notificaciones por email](docs/EMAIL_NOTIFICATIONS.md)
- [Migraciones de base de datos](docs/MIGRATIONS.md)

## Auto-actualización

Tanto el servidor como el cliente se actualizan solos:

- **Servidor:** al arrancar como root instala `/etc/cron.d/probakgo` (01:00 diario). Manual: `./probakgo update`
- **Cliente:** `probakgo-client install` instala `/etc/cron.d/probakgo-client` (01:00 diario). Manual: `probakgo-client update`

## Requisitos

- Go 1.22+ (para compilar)
- Linux x86-64 o arm64 (para ejecutar)
- Sin dependencias externas en runtime (SQLite incluido)
