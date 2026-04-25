# Instalación y Configuración del Servidor probaky (API + Web)

## Instalación Automática Completa en Linux

### Requisitos Previos
- Sistema Linux (Ubuntu/Debian, CentOS/RHEL, Fedora)
- Acceso a sudo
- Conexión a internet
- Puertos 36748 y 36749 disponibles

### Opción 1: Instalación Completa Automática (Recomendada)

**Una sola línea para instalar y configurar todo:**

1. **Clonar o descargar el proyecto:**
   ```bash
   git clone https://github.com/Nestorm18/probaky.git
   cd probaky
   ```

2. **Ejecutar instalación completa:**
   ```bash
   chmod +x scripts/*.sh
   ./scripts/install_complete.sh
   ```

3. **¡Listo!** Ambos servicios están funcionando:
   - 📡 **Servidor API:** http://localhost:36748
   - 🌐 **Interfaz Web:** http://localhost:36749

### Opción 2: Transferencia con SCP (Repositorio Privado)

Si el repositorio Git es privado, puedes transferir los archivos directamente usando SCP.

#### Desde Windows (PowerShell)

**Método 1: Transferir carpeta directamente**
```powershell
# Transferir todo el proyecto al servidor
scp -r C:\ruta\al\proyecto\probaky usuario@servidor:/tmp/
```

**Método 2: Comprimir y transferir (más rápido para conexiones lentas)**
```powershell
# Crear archivo zip
Compress-Archive -Path "C:\ruta\al\proyecto\probaky\*" -DestinationPath "$env:USERPROFILE\probaky.zip" -Force

# Transferir el zip al servidor
scp "$env:USERPROFILE\probaky.zip" usuario@servidor:/tmp/
```

#### Desde Linux/macOS

```bash
# Transferir carpeta directamente
scp -r /ruta/al/proyecto/probaky usuario@servidor:/tmp/

# O comprimir primero (opcional)
tar -czvf probaky.tar.gz -C /ruta/al/proyecto probaky
scp probaky.tar.gz usuario@servidor:/tmp/
```

#### En el servidor destino

```bash
# Si transferiste la carpeta directamente
cd /tmp/probaky

# Si transferiste un archivo zip
cd /tmp
unzip probaky.zip -d probaky
cd probaky

# Si transferiste un archivo tar.gz
cd /tmp
tar -xzvf probaky.tar.gz
cd probaky
```

#### ⚠️ Importante: Convertir finales de línea (Windows → Linux)

Los archivos transferidos desde Windows tienen finales de línea CRLF que deben convertirse a LF antes de ejecutar los scripts:

```bash
# Convertir todos los scripts
cd /tmp/probaky/scripts
sed -i 's/\r$//' *.sh

# Dar permisos de ejecución
chmod +x *.sh

# Ejecutar instalación
./install_complete.sh
```

### Opción 3: Instalación Manual Paso a Paso

#### Paso 1: Instalar Servidores
```bash
chmod +x install_server.sh
./install_server.sh
```

#### Paso 2: Iniciar Servidor API
```bash
sudo systemctl enable probaky-server
sudo systemctl start probaky-server
```

#### Paso 3: Configurar Servidor Web
```bash
chmod +x configure_web_admin.sh
./configure_web_admin.sh
```

#### Paso 4: Iniciar Servidor Web
```bash
sudo systemctl enable probaky-web
sudo systemctl start probaky-web
```

## ¿Qué Instala Automáticamente?

**Servidor API:**
- ✅ Instala dependencias (Python, SQLite, pip)
- ✅ Crea usuario del sistema `probaky-server`
- ✅ Configura entorno virtual de Python
- ✅ Inicializa la base de datos SQLite
- ✅ Genera claves API iniciales
- ✅ Configura servicio systemd `probaky-server`

**Servidor Web:**
- ✅ Crea usuario del sistema `probaky-web`  
- ✅ Instala interfaz web con Flask
- ✅ Configura entorno virtual separado
- ✅ Conecta automáticamente con el servidor API
- ✅ Configura servicio systemd `probaky-web`
- ✅ Genera credenciales de acceso por defecto

## Configuración

### Archivos de Configuración

**Configuración Unificada** - `/opt/probaky/.env`:
El sistema utiliza un único archivo de configuración para ambos servicios.

```env
# ================================
# CONFIGURACIÓN PARA EL SERVIDOR
# ================================
API_HOST=0.0.0.0
API_PORT=36748
API_URL=http://127.0.0.1:36748

# ================================
# CONFIGURACIÓN DE LA INTERFAZ WEB
# ================================
# API Key de administrador (se genera automáticamente)
ADMIN_API_KEY=adm-xxxxxxxxxxxxxxxxxxxxxxxxxx

# Configuración de Flask
FLASK_SECRET_KEY=change-this-secret-key-in-production
FLASK_DEBUG=False
FLASK_HOST=0.0.0.0
FLASK_PORT=36749
```

### Editar Configuración

```bash
sudo nano /opt/probaky/.env
```

## Gestión de Servicios

### Comandos para Ambos Servicios

```bash
# Ver estado de ambos servicios
sudo systemctl status probaky-server probaky-web

# Iniciar ambos servicios
sudo systemctl start probaky-server probaky-web

# Detener ambos servicios
sudo systemctl stop probaky-server probaky-web

# Reiniciar ambos servicios
sudo systemctl restart probaky-server probaky-web

# Habilitar inicio automático
sudo systemctl enable probaky-server probaky-web
```

### Comandos Individuales

**Servidor API:**
```bash
# Ver estado del servidor API
sudo systemctl status probaky-server

# Iniciar servidor API
sudo systemctl start probaky-server

# Detener servidor API
sudo systemctl stop probaky-server

# Reiniciar servidor API
sudo systemctl restart probaky-server

# Ver logs del servidor API en tiempo real
sudo journalctl -u probaky-server -f
```

**Servidor Web:**
```bash
# Ver estado del servidor Web
sudo systemctl status probaky-web

# Iniciar servidor Web
sudo systemctl start probaky-web

# Detener servidor Web
sudo systemctl stop probaky-web

# Reiniciar servidor Web
sudo systemctl restart probaky-web

# Ver logs del servidor Web en tiempo real
sudo journalctl -u probaky-web -f
```

## Desinstalación

```bash
chmod +x uninstall_server.sh
./uninstall_server.sh
```

El script de desinstalación preguntará si deseas:
- ❓ Conservar o eliminar la base de datos
- ❓ Conservar o eliminar los usuarios del sistema
- ❓ Desinstalar solo el servidor API o ambos servicios

## Monitoreo y Logs

### Ver Actividad del Servidor
```bash
# Logs en tiempo real
sudo journalctl -u probaky-server -f

# Logs con filtros
sudo journalctl -u probaky-server --since "1 hour ago"
sudo journalctl -u probaky-server --since "today"
```

### Verificar Conexiones
```bash
# Ver puertos en uso
sudo netstat -tlnp | grep :36748

# Ver procesos del servidor
ps aux | grep probaky-server
```

## URLs de Acceso

- **🔧 Servidor API:** `http://tu-servidor:36748`
- **📚 Documentación API:** `http://tu-servidor:36748/docs`
- **🌐 Interfaz Web:** `http://tu-servidor:36749`

### Credenciales de la Interfaz Web

- **Usuario:** `admin`
- **Contraseña:** `admin123`
- **⚠️ Importante:** Cambiar estas credenciales en producción

## Características del Sistema

### Servidor API
- 🔄 **Reinicio automático** en caso de fallos
- 🗄️ **Base de datos SQLite** integrada
- 🔐 **Gestión de claves API** segura
- 📊 **API REST** con FastAPI
- 📝 **Documentación automática** con Swagger

### Servidor Web
- 🌐 **Interfaz web intuitiva** con Flask
- 📊 **Dashboard en tiempo real** con métricas
- 🔑 **Gestión visual de API keys**
- 📈 **Reportes históricos** con filtros
- 🔐 **Sistema de autenticación** integrado
- 🔄 **Actualización automática** de datos
