# Sistema de Notificaciones por Email

Sistema automático de notificaciones por email que envía reportes diarios del estado de los servidores monitorizados.

## 📋 Características

- ✅ **Envío automático diario** a las 8:00 AM (Europe/Madrid)
- ✅ **Validación completa** de servidores PVE y PBS
- ✅ **Validación granular de VMs** respetando programación semanal
- ✅ **Email HTML profesional** con diseño responsive
- ✅ **Resumen ejecutivo** con estadísticas globales
- ✅ **Detalle completo** de todos los servidores (con problemas y operativos)

## ⚙️ Configuración

### 1. Variables de entorno

Añade las siguientes variables a tu archivo `.env`:

```bash
# Activar/desactivar sistema de emails
EMAIL_ENABLED=True

# Configuración SMTP
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=tu-email@gmail.com
SMTP_PASSWORD=tu-contraseña-o-app-password

# Remitente
SMTP_FROM_EMAIL=probaky-monitor@tudominio.com
SMTP_FROM_NAME=Probaky Monitor

# Destinatarios (separados por comas)
EMAIL_RECIPIENTS=admin@tudominio.com,soporte@tudominio.com

# Hora de envío (formato 24h)
EMAIL_SEND_HOUR=8
EMAIL_SEND_MINUTE=0
```

### 2. Configuración para Gmail

Si usas Gmail, necesitas una **Contraseña de aplicación**:

1. Ve a https://myaccount.google.com/apppasswords
2. Selecciona "Correo" y el dispositivo
3. Copia la contraseña generada (16 caracteres)
4. Úsala en `SMTP_PASSWORD`

### 3. Otros proveedores SMTP

| Proveedor | SMTP_HOST | SMTP_PORT |
|-----------|-----------|-----------|
| Gmail | smtp.gmail.com | 587 |
| Office 365 | smtp.office365.com | 587 |
| Outlook | smtp-mail.outlook.com | 587 |
| Yahoo | smtp.mail.yahoo.com | 587 |

## 🚀 Uso

### Inicio automático

El sistema se inicia automáticamente con el servidor API:

```bash
python server.py
```

Verás en la consola:

```
📧 Configurando sistema de notificaciones por email...
   ⏰ Hora de envío: 08:00 (Europe/Madrid)
✅ Sistema de notificaciones por email ACTIVADO
   📬 Destinatarios: admin@ejemplo.com,soporte@ejemplo.com
```

### Desactivar temporalmente

Para desactivar sin borrar la configuración:

```bash
EMAIL_ENABLED=False
```

### Prueba manual

Para probar el envío de email sin esperar a las 8:00 AM:

```bash
python email_notifier.py
```

## 📧 Contenido del Email

### Sección 1: Encabezado
- Color verde si todo OK, rojo si hay problemas
- Fecha y hora del reporte

### Sección 2: Resumen General
- Total de servidores PVE
- Total de servidores PBS
- Servidores con problemas
- Servidores operativos

### Sección 3: Servidores con Problemas

**Servidores PVE:**
- Nombre del servidor
- IP y hostname
- Motivo del problema:
  - "Reporte no recibido antes de las 8:00 AM"
  - "Faltan backups de X VM(s) configuradas"
- Lista detallada de VMs sin backup:
  - VM 100 - Nombre VM
  - Días programados (L, M, X, J, V, S, D)

**Servidores PBS:**
- Nombre del servidor
- IP y hostname
- Motivo: "Reporte no recibido antes de las 8:00 AM"

### Sección 4: Servidores Operativos

**Servidores PVE:**
- Información básica
- VMs configuradas vs VMs con backup
- Mensaje: "Todas las VMs programadas tienen backup"

**Servidores PBS:**
- Información básica
- Mensaje: "Reporte recibido correctamente"

## 🔍 Lógica de Validación

### Servidores PVE

1. **Timestamp del reporte:**
   - ✅ OK: Reporte de hoy
   - ❌ FALLO: Reporte de ayer y ya pasaron las 8:00 AM

2. **VMs configuradas:**
   - Solo valida VMs programadas para el día actual
   - Respeta configuración semanal (lunes, martes, etc.)
   - Excluye VMs marcadas como `is_excluded=True`

### Servidores PBS

1. **Solo timestamp del reporte:**
   - ✅ OK: Reporte de hoy
   - ❌ FALLO: Reporte de ayer y ya pasaron las 8:00 AM

## 🛠️ Solución de Problemas

### No se envían emails

1. Verifica que `EMAIL_ENABLED=True`
2. Comprueba credenciales SMTP
3. Revisa logs del servidor:
   ```
   ❌ Error al enviar email: [mensaje de error]
   ```

### Gmail rechaza la conexión

- Asegúrate de usar **Contraseña de aplicación**, no tu contraseña normal
- Verifica que la verificación en 2 pasos esté activada

### Destinatarios no reciben email

1. Verifica `EMAIL_RECIPIENTS`:
   ```bash
   EMAIL_RECIPIENTS=email1@dominio.com,email2@dominio.com
   ```
   ⚠️ **Sin espacios** entre emails

2. Revisa carpeta de spam

### Email sin formato

- El email es HTML, algunos clientes necesitan activar "Ver como HTML"
- Verifica que `Content-Type: text/html` esté presente

## 📊 Ejemplos de Asuntos

```
✅ Probaky Report: Todos los sistemas operativos - 2024-11-29
⚠️ Probaky Alert: 3 servidor(es) con problemas - 2024-11-29
⚠️ Probaky Alert: 1 servidor(es) con problemas - 2024-11-29
```

## 🔒 Seguridad

- **No guardes contraseñas en el código**: Usa `.env`
- **Usa contraseñas de aplicación**: Nunca tu contraseña principal
- **TLS/STARTTLS**: Todas las conexiones están cifradas
- **Variables sensibles**: Añade `.env` a `.gitignore`

## 📝 Logs

El sistema muestra logs detallados en consola:

```
📊 Iniciando generación de reporte diario
🕐 Hora: 2024-11-29 08:00:00 (Europe/Madrid)
📥 Obteniendo reportes de servidores...
   - 5 servidores PVE encontrados
   - 2 servidores PBS encontrados
🔍 Validando estado de servidores...
   - Servidores con problemas: 2
   - Servidores operativos: 5
📝 Generando contenido del email...
📧 Enviando email...
   Asunto: ⚠️ Probaky Alert: 2 servidor(es) con problemas - 2024-11-29
✅ Email enviado correctamente a: admin@ejemplo.com, soporte@ejemplo.com
✅ Reporte diario completado exitosamente
```

## 🧪 Testing

Para probar el sistema sin modificar la configuración:

1. **Modo testing** (no envía email):
   ```bash
   EMAIL_ENABLED=False
   python email_notifier.py
   ```

2. **Cambiar hora temporalmente**:
   ```bash
   EMAIL_SEND_HOUR=14  # Enviar a las 14:00
   EMAIL_SEND_MINUTE=30  # y 30 minutos
   ```

3. **Email de prueba a ti mismo**:
   ```bash
   EMAIL_RECIPIENTS=tu-email@gmail.com
   ```

## 🔄 Integración con el Sistema

El sistema usa `report_utils.py` para:
- ✅ Parsear timestamps
- ✅ Validar deadline de 8:00 AM
- ✅ Validar VMs configuradas
- ✅ Respetar días de la semana

Esto garantiza que **la validación es idéntica** en:
- Interfaz web
- Dashboard
- Reportes
- **Emails**

## 📚 Referencias

- **Código**: `email_notifier.py`
- **Scheduler**: `server.py` (función `start_email_scheduler()`)
- **Configuración**: `.env` o `.env.server.example`
- **Validación**: `report_utils.py`
