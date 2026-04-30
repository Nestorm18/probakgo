# probakgo

Monitor de Proxmox VE y Proxmox Backup Server con dashboard web e informes diarios por email.

```
probakgo-client  ──POST /report/pve──▶  probakgo (servidor)
probakgo-client  ──POST /report/pbs──▶  probakgo (servidor)
                                             │
                                        SQLite DB
                                             │
                              Web UI (navegador ← puerto 36748)
```

## Instalación rápida

```bash
# Servidor
wget https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo_linux_amd64 -O probakgo
chmod +x probakgo
echo "SESSION_KEY=$(openssl rand -hex 32)" > .env
./probakgo
# → http://localhost:36748   probakgo / admin123
```

```bash
# Cliente (en cada nodo Proxmox, como root)
wget https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo-client_linux_amd64 -O probakgo-client
chmod +x probakgo-client
./probakgo-client install --api-url http://tu-servidor:36748 --api-key pbk-...
```

Ver [INSTALLATION.md](INSTALLATION.md) para la guía completa con systemd, nginx y múltiples nodos.

## Configuración del servidor (`.env`)

| Variable | Por defecto | Descripción |
|----------|-------------|-------------|
| `SESSION_KEY` | *(aleatorio)* | Clave de sesión — **definir para persistir sesiones entre reinicios** |
| `API_HOST` | `0.0.0.0` | Interfaz de escucha |
| `API_PORT` | `36748` | Puerto |
| `DATABASE_PATH` | `probakgo_data.db` | Ruta al archivo SQLite |
| `TIMEZONE` | `Europe/Madrid` | Zona horaria para el scheduler de email |
| `SESSION_SECURE` | `false` | Activar a `true` si el servidor está detrás de HTTPS |

El email (SMTP, destinatarios, hora de envío) se configura desde la interfaz web en **Ajustes → Email**.

## Configuración del cliente (`/opt/probakgo/.env`)

| Variable | Descripción |
|----------|-------------|
| `API_URL` | URL del servidor probakgo |
| `API_KEY` | API key `pbk-` creada en la interfaz web |
| `PROXMOX_TOKEN` | Token API Proxmox (`usuario@realm!nombre`) — generado por `install` |
| `PROXMOX_SECRET` | Secret del token — generado por `install` |
| `PROXMOX_VERIFY_TLS` | `false` para certificados auto-firmados (habitual en Proxmox) |
| `PROXMOX_CA_BUNDLE` | Ruta a CA personalizada (opcional) |

## Roles de usuario

| Rol | Acceso |
|-----|--------|
| `reader` | Solo lectura: dashboard, servidores, backups |
| `editor` | reader + edición de configuración de backups |
| `admin` | Acceso completo: usuarios, API keys, ajustes |

## Tipos de API key

| Prefijo | Uso |
|---------|-----|
| `pbk-` | Agentes cliente en nodos Proxmox |
| `app-` | Integraciones externas (solo lectura) |
| `adm-` | Acceso admin a la API REST |

## Auto-actualización

- **Servidor:** instala `/etc/cron.d/probakgo` en el primer arranque como root (01:00 diario). Manual: `./probakgo update`
- **Cliente:** `install` instala `/etc/cron.d/probakgo-client` (01:00 diario). Manual: `probakgo-client update`

## Compilar desde código fuente

Requiere Go 1.22+.

```bash
go build -o probakgo .
go build -o probakgo-client ./client/
```

## Documentación

- [Guía de instalación completa](INSTALLATION.md)
- [Notificaciones por email](docs/EMAIL_NOTIFICATIONS.md)
- [Migraciones de base de datos](docs/MIGRATIONS.md)
