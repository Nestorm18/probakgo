# Notificaciones por Email

probakgo envía un reporte diario por email con el estado de todos los servidores Proxmox monitorizados.

## Configuración

El email se configura **desde la interfaz web** en **Ajustes → Email**. No se requieren variables de entorno.

Campos configurables:
- Host y puerto SMTP
- Usuario y contraseña SMTP
- Destinatarios (separados por comas)
- Hora de envío (formato HH:MM, zona horaria definida por `TIMEZONE` en `.env`)

### Gmail - Contraseña de aplicación

Gmail requiere una contraseña de aplicación (no la contraseña normal):

1. Ir a https://myaccount.google.com/apppasswords
2. Seleccionar "Correo" y el dispositivo
3. Copiar la contraseña generada (16 caracteres)
4. Usarla como contraseña SMTP en la interfaz web

### Proveedores SMTP comunes

| Proveedor | Host | Puerto |
|-----------|------|--------|
| Gmail | smtp.gmail.com | 587 |
| Office 365 | smtp.office365.com | 587 |
| Outlook | smtp-mail.outlook.com | 587 |

Todos usan STARTTLS en el puerto 587.

## Lógica del reporte diario

El scheduler se ejecuta a la hora configurada y evalúa:

**Servidores PVE:**
- Reporte recibido hoy → OK
- Reporte de ayer (no actualizado) → FALLO
- VMs con backup programado para hoy sin backup → FALLO

**Servidores PBS:**
- Reporte recibido hoy → OK
- Sin reporte de hoy → FALLO

## Contenido del email

- Resumen global: total PVE, total PBS, servidores con problemas vs operativos
- Detalle de servidores con problemas (motivo del fallo)
- Detalle de servidores operativos

## Asuntos de ejemplo

```
✅ Probago Report: Todos los sistemas operativos - 2025-04-25
⚠️ Probago Alert: 2 servidor(es) con problemas - 2025-04-25
```

## Prueba manual

Desde la interfaz web, **Ajustes → Email → Enviar reporte de prueba** envía el reporte inmediatamente sin esperar a la hora programada.
