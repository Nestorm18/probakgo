# probaky - Sistema de Monitoreo

Sistema completo de monitoreo de servidores compatible con Proxmox que incluye:
- 🖥️ **Servidor API** - FastAPI para recibir métricas
- 💻 **Cliente de Monitoreo** - Envía métricas del sistema
- 🌐 **Interfaz Web** - Panel de administración visual

## 🚀 Instalación Rápida

### 1) Preparación (en el servidor destino)

```bash
apt install git -y
git clone https://github.com/Nestorm18/probaky.git
cd probaky
sed -i 's/\r$//' *.sh scripts/*.sh
chmod +x probaky.sh scripts/*.sh
./probaky.sh help
```

### 2) Instalación recomendada (API + Web)

```bash
sudo ./probaky.sh install-complete
```

Al finalizar:
- API: http://localhost:36748
- Web: http://localhost:36749
- Login inicial: probaky / admin123 (cambiar en producción)

### 3) Alta de cliente (en cada nodo Proxmox)

Primero crea una API key `pbk-` desde la interfaz web.

Luego en el nodo:

```bash
sudo ./probaky.sh install-client \
    --api-url http://IP_SERVIDOR:36748 \
    --api-key pbk-tu-api-key
```

Notas:
- El instalador intenta generar automáticamente `PROXMOX_TOKEN` y `PROXMOX_SECRET` si se ejecuta como root.
- Si quieres forzarlos manualmente: usa `--proxmox-token` y `--proxmox-secret`.

### 4) Actualización sencilla (servidor + web + publicación de cliente)

Importante:
- `/opt/probaky/server` y `/opt/probaky/web` son el destino de instalación (runtime).
- El comando `update-server` se lanza desde una copia del repositorio (donde está `probaky.sh` y `.git`).

Ruta habitual tras instalación/clonado inicial (como root):

```bash
cd ~/probaky
git pull
sed -i 's/\r$//' *.sh && sed -i 's/\r$//' scripts/*.sh && chmod +x scripts/*.sh && chmod +x *.sh
chmod +x probaky.sh
./probaky.sh update-server
```

Este comando:
- Actualiza API y Web.
- Reinicia servicios.
- Publica automáticamente una versión de `client.py` en `download/` para auto-update de clientes.

### 5) Auto-update del cliente

Se configura automáticamente al instalar cliente:
- Script: `/opt/probaky/run_updater.sh`
- Cron: cada hora
- Log: `/var/log/probaky/updater.log`

Forzar update manual en un cliente:

```bash
sudo /opt/probaky/run_updater.sh
```

## 🧾 Comandos Principales

```bash
./probaky.sh install-server
./probaky.sh install-client
./probaky.sh install-complete
./probaky.sh manage-services [start|stop|restart|status|logs]
./probaky.sh update-server
./probaky.sh configure-web
./probaky.sh uninstall-server
./probaky.sh uninstall-client
./probaky.sh help
```

---

## 🌟 Características Principales

### 📊 Métricas Monitoreadas
TODO:
- **IP Address**: IP local del servidor
- **Public IP**: IP pública del servidor
- **Hostname**: Nombre del servidor

### 🌐 Interfaz Web
- 🔐 **Login seguro** con autenticación
- 📊 **Dashboard interactivo** con estado en tiempo real
- 🖥️ **Gestión de servidores** con métricas visuales
- 🔑 **Administración de API Keys** para servidores y administración
- 📈 **Reportes históricos** con filtros avanzados
- 🔄 **Actualización automática** cada 30 segundos

### 🔐 Sistema de API Keys
- **Servidor (pbk-)**: Agentes Proxmox que envían métricas
- **Administrador (adm-)**: Panel web e integraciones con la API de administración

## 🗂️ Estructura de Servicios

```
probaky Sistema Completo (/opt/probaky)
├── 📄 Configuración (.env)
├── 📡 Servidor API (server/)
│   ├── FastAPI + SQLite
│   └── Logs en /var/log/probaky/
│
└── 🌐 Servidor Web (web/)
    ├── Interfaz Flask
    └── Logs en /var/log/probaky/
```

## 🔧 Configuración

### Archivos de Configuración

**Servidor (API + Web)** - `/opt/probaky/.env`:
```env
# Configuración unificada
API_HOST=0.0.0.0
API_PORT=36748
API_URL=http://127.0.0.1:36748

ADMIN_API_KEY=adm-xxxxxxxxxxxxxxxxxxxxxxxxxx
FLASK_SECRET_KEY=change-this-secret-key-in-production
```

**Cliente de Monitoreo** - `/opt/probaky/.env`:
```env
API_URL=http://192.168.1.123:36748
API_KEY=pbk-tu-api-key-aqui
```

### Editar Configuración

```bash
# Servidor (API y Web)
sudo nano /opt/probaky/.env

# Cliente
sudo nano /opt/probaky/.env
```

## 🛠️ Gestión de Servicios

### Ver Estado
# Ambos servicios
```bash
./probaky.sh manage-services status
```

# Individual
```bash
sudo systemctl status probaky-server  # API
sudo systemctl status probaky-web     # Web
sudo systemctl status probaky         # Cliente
```

### Ver Logs
# Ambos servicios en tiempo real
```bash
./probaky.sh manage-services logs
```

# Individual
```bash
sudo journalctl -u probaky-server -f  # API
sudo journalctl -u probaky-web -f     # Web
sudo journalctl -u probaky -f         # Cliente
```

### Reiniciar Servicios
```bash
# Ambos servicios
./probaky.sh manage-services restart

# Individual
sudo systemctl restart probaky-server
sudo systemctl restart probaky-web
sudo systemctl restart probaky
```

## 🔄 Características Avanzadas

### Reinicio Automático
- ✅ Reinicio automático del servicio si falla
- ✅ Inicio automático al arrancar el sistema  
- ✅ Servicios systemd dedicados
- ✅ Logs centralizados con systemd

### Servidor API:
- 🌐 API REST con FastAPI y documentación automática
- 🗄️ Base de datos SQLite con migraciones automáticas
- 🔐 Gestión completa de claves API con tipos
- 🔧 Usuario dedicado `probaky-server`
### Servidor Web:
- 🔐 Sistema de login con hash de contraseñas
- 📊 Dashboard con actualización en tiempo real
- 🎨 Interfaz moderna con Bootstrap 5
- 📱 Responsive design para móviles
- 🔧 Usuario dedicado `probaky-web`

### Cliente:
- 📊 Reportes vez que termina un backup
- 🔧 Usuario dedicado `probaky`
- 🔁 Reintentos automáticos si falla la conexión

## 🚨 Resolución de Problemas

### Error: "no se puede ejecutar: no se ha encontrado el fichero requerido"
```bash
# Opción 1: Comando rápido
./probaky.sh fix-permissions

# Opción 2: Manual
chmod +x probaky.sh
chmod +x scripts/*.sh
```

### Error: "No existe el fichero o el directorio" (al instalar)
```bash
# Asegúrate de estar en el directorio correcto
cd /ruta/a/probaky

# Corrige permisos y vuelve a intentar
./probaky.sh fix-permissions
./probaky.sh install-complete
```

### Error: "No se puede conectar al API"
```bash
./probaky.sh manage-services status      # Verificar estado
./probaky.sh manage-services restart-api # Reiniciar API
./probaky.sh manage-services logs-api    # Ver logs
```

### Error: "Interfaz web no carga"
```bash
./probaky.sh manage-services restart-web # Reiniciar Web
./probaky.sh manage-services logs-web    # Ver logs
```

### Reconfigurar API key del Web
```bash
./probaky.sh configure-web       # Reconfigurar automáticamente
```

### Instalación desde cero
```bash
./probaky.sh uninstall-server   # Desinstalar todo
./probaky.sh install-complete   # Reinstalar todo
```

## 🔒 Seguridad

### Producción:
- Cambia las credenciales por defecto de la interfaz web
- Usa HTTPS para todas las comunicaciones
- Configura `FLASK_DEBUG=False`
- Usa claves secretas seguras
- Restringe acceso a puertos de API (incluyendo `/docs` si es necesario)

### API Keys:
- Prefijos según tipo (pbk-, adm-)
- Activación/desactivación sin eliminar
- Seguimiento de último uso
- Generación criptográficamente segura

### Cambiar Credenciales Web (Producción):
1. **Editar `/opt/probaky-web/app.py`**
2. **Cambiar usuario/contraseña en la variable `USERS`**
3. **Cambiar `FLASK_SECRET_KEY` en `/opt/probaky-web/.env`**
4. **Reiniciar:** `./probaky.sh manage-services restart-web`

## 🗑️ Desinstalación

```bash
# Desinstalar servidores (API + Web)
./probaky.sh uninstall-server

# Desinstalar cliente
./probaky.sh uninstall-client
```

Los scripts de desinstalación preguntarán si deseas:
- ❓ Conservar o eliminar la base de datos
- ❓ Conservar o eliminar los usuarios del sistema

## 📋 Requisitos del Sistema

### Sistema Base:
- Sistema Linux (Ubuntu/Debian recomendado; compatible con CentOS/RHEL/Fedora)
- Python 3.13+
- Acceso sudo
- Puertos 36748 y 36749 disponibles

### Dependencias (se instalan automáticamente):
- FastAPI, SQLAlchemy, Uvicorn
- Flask, Werkzeug
- psutil, requests, python-dotenv
- pytz

## 🔧 Desarrollo

Para desarrollo local:

```bash
# Activar entorno virtual (opcional pero recomendado)
python -m venv .venv

source .venv/bin/activate  # Linux/Mac

# Windows (PowerShell / CMD)
# PowerShell puede bloquear la ejecución de scripts por la política de ejecución.
# Opciones para activar el entorno en Windows:
# 1) Usar CMD (más sencillo, no requiere cambiar políticas):
#    .venv\Scripts\activate.bat
# 2) PowerShell (permitir scripts para el usuario - una vez):
#    Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
#    .\.venv\Scripts\Activate.ps1
# 3) Activar solo en el proceso actual (sin cambiar la política global):
#    powershell -ExecutionPolicy Bypass -NoProfile -Command ". .\.venv\Scripts\Activate.ps1"
# 4) Usar Git Bash / WSL:
#    source .venv/Scripts/activate

# Instalar dependencias
pip install -r requirements.txt

# Configurar variables de entorno
cp .env.example .env  # Si existe, o crear el archivo .env

# Ejecutar servidor API o usar .\tests\test_populate_data.py para datos de prueba
python server.py

# Ejecutar cliente (en otra terminal)
python client.py

## Ejecutar interfaz web (en otra terminal)

# Copiar la clave admin de accesos.txt a .env
# API Key ADMIN: adm-3aKTW9FXbI5UW6eZ1D9xRfLP00VdWQ1A

cd web_interface

# Desarrollo (rápido, no recomendado en producción)
python app.py

## Uso con Waitress (recomendado para producción)
Para producción o despliegues en servidores Windows/Linux se recomienda usar Waitress (WSGI server).

# Opción A: usar el script incluido `run_waitress.py` desde la raíz del proyecto
# (el instalador ya copia `run_waitress.py` al directorio web durante la instalación)
python ../run_waitress.py

# Opción B: usar el comando `waitress-serve` apuntando al objeto WSGI
waitress-serve --listen=127.0.0.1:36749 "web_interface.app:app"
```

### URLs de Desarrollo:
- **API**: http://localhost:36748
- **Documentación API**: http://localhost:36748/docs
- **Interfaz Web**: http://localhost:36749

## 👤 Sistema de Usuarios

La interfaz web incluye un **sistema de usuarios flexible** con control de acceso basado en roles:

### 🔐 Características
- ✅ **Múltiples usuarios** con diferentes roles
- ✅ **Contraseñas seguras** y personalizables  
- ✅ **Control de acceso** basado en roles (Admin/Viewer)
- ✅ **Gestión completa** desde interfaz web y línea de comandos
- ✅ **Auto-configuración** - sin scripts manuales

### 🚀 Uso Inmediato

Al iniciar la aplicación web por primera vez se **auto-configura**:
- Crea automáticamente la tabla de usuarios
- Genera usuario administrador por defecto
- ¡Solo necesitas cambiar la contraseña!

**Credenciales iniciales (solo primera vez):**
- 👤 Usuario: `probaky`
- 🔐 Contraseña: `admin123`
- ⚠️ **¡Cambiar inmediatamente por seguridad!**

### 👑 Roles Disponibles

| Rol | Permisos |
|-----|----------|
| **👑 Admin** | • Gestión completa de usuarios<br>• Gestión de API Keys<br>• Acceso a todas las funciones |
| **👤 Viewer** | • Solo lectura<br>• Ver dashboards y reportes<br>• Sin permisos de gestión |

### 🛠️ Gestión de Usuarios

**Desde la Interfaz Web:**
- Ver usuarios: Menú "Usuarios" (solo admins)
- Crear usuarios: Botón "Nuevo Usuario"  
- Editar perfil: Dropdown usuario → "Mi Perfil"

## 📞 Documentación Adicional

Para información más detallada:
- **Instalación del Servidor**: [docs/INSTALL_SERVER.md](docs/INSTALL_SERVER.md)
- **Notificaciones por Email**: [docs/EMAIL_NOTIFICATIONS.md](docs/EMAIL_NOTIFICATIONS.md)
- **Migraciones de Base de Datos**: [docs/MIGRATIONS.md](docs/MIGRATIONS.md)
- **Scripts de Instalación**: [scripts/README.md](scripts/README.md)
- **Interfaz Web**: [web_interface/README.md](web_interface/README.md)

## 📞 Soporte

### Issues y Contribuciones:
- Reporta problemas en GitHub Issues
- Pull requests son bienvenidos
- Sigue las convenciones de código existentes

---

🎯 **¡probaky está listo para usar!** - Sistema completo de monitoreo con instalación automática en una sola línea.