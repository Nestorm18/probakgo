# TODO — Probakgo

Tareas pendientes ordenadas por prioridad. Las ideas marcadas con 🔍 están inspiradas en patrones del proyecto **netbird-main**.

---

## TIER 1 — Alta prioridad

### Seguridad

- [ ] **API keys con expiración y fecha de último uso** 🔍
  Actualmente las `pbk-` y `adm-` keys nunca expiran y no registran cuándo se usaron por última vez.
  Añadir columnas `expires_at` (nullable) y `last_used_at` a la tabla `api_keys`.
  Mostrar en la UI y advertir cuando una key lleva >90 días sin usarse o está próxima a expirar.
  _(inspirado en: `management/server/http/middleware/auth_middleware.go:164–168`)_

- [ ] **Rotación automática de session key sin downtime**
  `SESSION_KEY` generada en `.env` no tiene mecanismo de rotación. Si se filtra, todas las sesiones son válidas indefinidamente. Soportar array de claves (primary + legacy) en gorilla/sessions para invalidar la anterior gradualmente.

- [ ] **2FA / TOTP para cuentas de administrador**
  Ningún factor adicional protege las cuentas admin. Añadir TOTP (RFC 6238) con QR de enrolamiento en `/profile`. Obligatorio para rol `admin` si está activado globalmente.

- [ ] **Brute-force protection mejorado en login**
  El rate limiter actual es por IP (ventana fija). Añadir lockout por usuario tras N intentos fallidos (registrar en DB, resetear con el tiempo o manualmente desde admin). Diferenciar 429 global de "cuenta temporalmente bloqueada".

### Auditoría y trazabilidad

- [ ] **Audit log de cambios administrativos** 🔍
  No existe registro de quién modificó qué. Crear tabla `audit_log` con campos:
  `id, user_id, action (string), target_type, target_id, meta (JSON), created_at`.
  Registrar: cambios de contraseña, creación/eliminación de usuarios, cambios en configuración de email/alertas, creación/revocación de API keys.
  Mostrar en una página `/admin/audit-log` (solo admin).
  _(inspirado en: `management/server/activity/event.go` + `codes.go`)_

- [ ] **Request ID en todos los logs** 🔍
  Los logs de slog no tienen correlación entre requests. Añadir middleware que genere un UUID por request y lo propague vía `context.WithValue`. Todos los logs dentro del handler lo incluyen automáticamente con `slog.With("request_id", id)`.
  _(inspirado en: `management/server/telemetry/http_api_metrics.go:151–153`)_

### Observabilidad

- [ ] **Endpoint `/metrics` con Prometheus** 🔍
  Exponer métricas básicas: requests totales por endpoint/método/status, latencia (histograma), conexiones activas, queries SQLite (duración), jobs de email (ok/error). Middleware que envuelve `ResponseWriter` para capturar status code.
  _(inspirado en: `management/server/telemetry/http_api_metrics.go` + `app_metrics.go`)_

- [ ] **Health check detallado en `/api/health`**
  El health check actual devuelve `{"status":"ok"}` sin más. Añadir: versión, uptime, estado de la DB (ping), hora del último email enviado, cantidad de servidores monitorizados. Útil para Prometheus blackbox exporter y dashboards.

---

## TIER 2 — Prioridad media

### Notificaciones y alertas

- [ ] **Webhooks para alertas** 🔍
  Además del email diario, enviar alertas en tiempo real a una URL configurable (Slack/Teams/Grafana OnCall/webhook genérico). Payload JSON estándar con tipo de alerta, servidor, severidad y timestamp. Configurar en `/settings/alerts` con campo webhook URL y botón de test.

- [ ] **Alertas de cliente desconectado**
  Si un nodo no reporta en X horas (configurable por servidor), disparar alerta. Distinto de "stale backup": este es sobre la conectividad del agente, no del backup en sí. Útil para detectar agentes caídos o actualizaciones fallidas.

- [ ] **Resumen semanal por email**
  Además del reporte diario, ofrecer un resumen semanal con tendencias: evolución del uso de disco (gráfico ASCII o tabla), backups exitosos vs fallidos por servidor, nuevos errores de la semana.

### UI y UX

- [x] **Filtro en tiempo real en las listas de servidores**
  Input de búsqueda por JS encima de las tablas en `servers_pve.html` y `servers_pbs.html`. Filtra por cualquier texto visible (nombre, IP, estado) sin reload.

- [x] **Timestamps relativos en las tablas**
  `formatTimeAgo` añadida a `makeFuncMap()`. Las tablas de servidores y el dashboard muestran "hace 3h" con la fecha absoluta en el tooltip `title`.

- [x] **Empty state útil cuando no hay servidores**
  Reemplazado "Sin servidores registrados" por un panel con los 3 pasos de instalación del cliente, botón de copy en cada línea y enlace a API Keys.

- [x] **Exportar historial a CSV**
  Botón "CSV" en `reports_pve.html` → `GET /servers/pve/{id}/reports/csv`. Descarga todas las filas del período seleccionado.

- [x] **Estado en las tarjetas del dashboard**
  `dashboard.html` ahora muestra badge "Activo"/"Sin reporte" y el timestamp relativo en cada tarjeta de servidor.

- [x] **`api_key_created.html` integrado en el layout base**
  Refactorizado para usar `base.html`. Mismo sidebar, mismo tema, misma consistencia visual que el resto de la app.

- [x] **Página `/about` con estado de la instancia**
  Nueva página admin-only `/about` con versión, uptime, nodos monitorizados, tamaño de BD. Enlace "Acerca de" en el sidebar (sección Sistema).

- [x] **Ordenación de columnas en tablas**
  Click en headers "Servidor", "Dirección", "Último reporte" y "Estado" ordena las filas del DOM. Icono de dirección en el header activo.

- [x] **Perfil: mostrar último acceso y sesiones activas**
  Migración `012` añade `last_login_at` y `last_login_ip` a `users`. `LoginPost` los guarda. `profile.html` muestra último acceso (relativo + absoluto en tooltip) e IP.

- [x] **Toasts en lugar de flash en acciones no críticas**
  `base.html` muestra todas las notificaciones como Bootstrap toasts: éxitos (4s, esquina inferior derecha) y errores (10s, parte superior centrada). Eliminados los bloques inline `{{if .Flash}}` de todas las páginas.

### Operaciones

- [ ] **Builds multi-plataforma en releases** 🔍
  El workflow de release actual solo compila `linux_amd64`. Añadir con GoReleaser: `linux_arm64` (Raspberry Pi, AWS Graviton), `darwin_arm64` (Mac M1/M2 para desarrollo). Incluir checksums SHA256 ya existentes en el proceso.
  _(inspirado en: `netbird-main/.goreleaser.yaml`)_

- [ ] **Backup y restauración de la BD desde la UI**
  Añadir en `/settings/maintenance` un botón "Descargar backup de BD" que sirve el archivo SQLite como descarga (con Content-Disposition). También botón "Restaurar" que acepta upload de un `.db`.

- [ ] **Configuración del nivel de log en runtime**
  Añadir endpoint `POST /api/admin/log-level` (solo admin) para cambiar el nivel de slog en caliente entre `info` y `debug` sin reiniciar. Útil para debugging en producción.

---

## TIER 3 — Baja prioridad / futuro

### Arquitectura y testing

- [ ] **Interfaz `Store` para facilitar mocking en tests** 🔍
  El store actual es una struct concreta. Definir una interfaz `Store` con todos los métodos públicos. Permite crear mocks en tests de handlers sin SQLite real.
  _(inspirado en: `management/server/store/store.go`)_

- [ ] **Errores tipados con mapeo automático a HTTP status** 🔍
  Reemplazar retornos de `fmt.Errorf(...)` en handlers por tipos de error estructurados (`ErrNotFound`, `ErrUnauthorized`, `ErrConflict`) que middleware central convierte a status code correcto. Elimina el boilerplate de cada handler.
  _(inspirado en: `management/server/status/error.go`)_

- [ ] **Validación de datos de entrada con tags struct**
  Los handlers parsean JSON sin validación declarativa (longitudes mínimas, formatos). Añadir una función `validate(v any) error` simple basada en reflect + tags o usar `go-playground/validator` para campos críticos (email, contraseñas, URLs).

### Features avanzados

- [ ] **Multi-tenant (múltiples organizaciones)**
  Actualmente todos los usuarios comparten los mismos servidores. Añadir concepto de "organización" con aislamiento completo. Solo si el caso de uso lo requiere (SaaS hosted).

- [ ] **OIDC / SSO login**
  Permitir login via proveedor externo (Google, Keycloak, Authentik). Útil en entornos con SSO corporativo. Implementar como opción adicional, manteniendo el login local.

- [ ] **API OpenAPI/Swagger**
  Generar documentación de la API REST automáticamente (usando `swaggo/swag` o escribiendo el spec a mano). Facilita integraciones de terceros.

- [ ] **Cliente para BSD / macOS**
  El cliente actual solo soporta Linux (usa `/etc/issue`, `/etc/machine-id`, `pveum`). Evaluar soporte para otros SO donde pueda correr Proxmox o herramientas compatibles.

---

## Deuda técnica

- [ ] Eliminar referencias a "probaky" que queden en comentarios o strings
- [ ] `internal/web/session.go` y `internal/session/session.go` — revisar si hay duplicidad después de la extracción para evitar import cycles
- [ ] Añadir tests de integración para los handlers de la API (actualmente solo hay test helpers)
- [ ] El `cleanup.go` debería respetar un contexto cancelable en todas sus rutas de error
