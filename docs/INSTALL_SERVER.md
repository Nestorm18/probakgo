# Instalación del Servidor probakgo

El servidor es un único binario Go que sirve tanto la API REST como la interfaz web en el mismo puerto.

## Requisitos

- Linux x86-64 o arm64
- Puerto 36748 disponible
- Sin dependencias externas (SQLite CGO-free incluido)

## Instalación desde binario (recomendado)

Descarga el binario de la última release en GitHub:

```bash
# Descargar binario
wget https://github.com/Nestorm18/probakgo/releases/latest/download/probakgo_linux_amd64 -O probakgo
chmod +x probakgo

# Configurar
cp .env.server.example .env
nano .env   # ajustar SESSION_KEY al menos

# Ejecutar
./probakgo
```

## Instalación desde código fuente

```bash
git clone https://github.com/Nestorm18/probakgo.git
cd probakgo
go build -o probakgo .
cp .env.server.example .env
./probakgo
```

## Configuración

Copia `.env.server.example` a `.env` en el mismo directorio del binario:

```env
API_HOST=0.0.0.0
API_PORT=36748
DATABASE_PATH=./probakgo_data.db
SESSION_KEY=genera-una-clave-segura-de-32bytes
TIMEZONE=Europe/Madrid
```

Genera una clave de sesión segura:

```bash
openssl rand -base64 32
```

## Servicio systemd

Crea `/etc/systemd/system/probakgo.service`:

```ini
[Unit]
Description=probakgo - Proxmox Monitor
After=network.target

[Service]
Type=simple
User=probakgo
WorkingDirectory=/opt/probakgo
ExecStart=/opt/probakgo/probakgo
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
# Crear usuario dedicado
useradd -r -s /sbin/nologin probakgo

# Copiar binario y .env
mkdir /opt/probakgo
cp probakgo /opt/probakgo/
cp .env /opt/probakgo/

# Activar servicio
systemctl daemon-reload
systemctl enable --now probakgo
```

## Acceso inicial

- **URL:** `http://tu-servidor:36748`
- **Usuario:** `probakgo`
- **Contraseña:** `admin123` - **cambiar inmediatamente en producción**

El binario crea el usuario y una API key de administración en el primer arranque. Las credenciales aparecen en el log.

## Actualización

El servidor se auto-actualiza diariamente a las 01:00 via cron (instalado en el primer arranque como root).

Actualización manual:

```bash
/opt/probakgo/probakgo update
```

El comando descarga el binario más reciente de GitHub y reinicia el servicio via `systemctl restart probakgo`.

Para actualizar manualmente sin auto-update:

```bash
systemctl stop probakgo
cp nuevo-probakgo /opt/probakgo/probakgo
systemctl start probakgo
```

Las migraciones de BD se aplican automáticamente al arrancar. Ver [MIGRATIONS.md](MIGRATIONS.md).

## Logs

```bash
journalctl -u probakgo -f
```

## Configuración de email

El email se configura desde la interfaz web en **Ajustes → Email**. No se necesitan variables de entorno para SMTP.

Ver [EMAIL_NOTIFICATIONS.md](EMAIL_NOTIFICATIONS.md).
