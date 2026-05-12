# TODO — Probakgo

Tareas pendientes ordenadas por área y prioridad.

---

## PLAN: Sistema de Alertas por Servidor/VM

Arquitectura implementada. El motor unificado `alertengine.go → RunAll()` es la única fuente de alertas tanto en la web UI como en el email.

### Trabajo pendiente

**UI de configuración por servidor**
- `/servers/pve/{id}/alerts` — umbrales por servidor PVE
- `/servers/pbs/{id}/alerts` — umbrales por servidor PBS
- Enlace desde la tarjeta del servidor en la lista

---

## PLAN: Refactor de duplicidades — domain shared

### Problema
Hay tres implementaciones idénticas de la misma lógica de días de backup:

| Función | Fichero |
|---|---|
| `vmScheduledForDay` | `internal/web/handlers/servers.go:347` |
| `alertVMScheduledForDay` | `internal/service/alertengine.go:624` |
| `emailVMScheduledForDay` | `internal/service/email.go:383` |

### Plan
- Mover a `internal/domain/schedule.go → VMScheduledForDay(c VMBackupConfig, day time.Weekday) bool`
- Actualizar los tres call-sites para importar desde domain
- Añadir un test unitario en domain

---


## MEJORA: Propagación de contexto en servicios

`internal/service/report.go` y `internal/service/alertengine.go` crean `context.Background()` internamente en lugar de recibir el contexto de quien llama.

Consecuencias:
- Las operaciones de DB no pueden cancelarse si el request HTTP se cancela
- No se pueden añadir timeouts por request
- Bloquea instrumentación futura (tracing, logging con request-id)

**Fix**: Cambiar firmas para que los métodos reciban `ctx context.Context` como primer parámetro y lo propagen a las llamadas de store. Empezar por `report.SavePVEReport` y `report.SavePBSReport` ya que son los puntos de entrada del cliente.

---

## SEGURIDAD: Filtración de errores internos al cliente web

Los handlers web devuelven `http.Error(w, err.Error(), ...)` con mensajes de error internos del store/DB expuestos al navegador:

- `internal/web/handlers/alerts.go:19, 24`
- `internal/web/handlers/email.go:35, 92, 144`
- `internal/web/handlers/servers.go:19, 262`
- `internal/web/handlers/dashboard.go:15, 20`

**Fix**: Loggear el error internamente con `slog.Error(...)` y devolver un mensaje genérico al cliente: `http.Error(w, "error interno del servidor", http.StatusInternalServerError)`.

---


## RENDIMIENTO: Falta índice en `alert_suppressions`

`internal/db/migrations/008_alert_suppressions.up.sql` crea la tabla sin índice en `suppressed_until`:

```sql
-- migration 009 o siguiente
CREATE INDEX IF NOT EXISTS idx_alert_suppressions_until
ON alert_suppressions(suppressed_until);
```

La query `WHERE suppressed_until > ?` hace table scan. Con muchas supresiones acumuladas el coste crece linealmente.

---

## CALIDAD: Sesiones no se invalidan al cambiar rol o desactivar usuario

Si un admin degrada un usuario a `reader`, o desactiva su cuenta, la sesión activa sigue válida hasta que expire (horas/días).

**Fix simple**: Añadir un check en el middleware de sesión (`internal/web/handlers/auth.go`) que verifique el estado del usuario en DB en cada request (o con TTL corto). Alternativa: al guardar cambios de usuario, incrementar un `session_version` en la tabla `users` y validarlo en sesión.

---

## CALIDAD: Validación de configuración al arranque

`internal/config/config.go` no valida los valores cargados. Errores de configuración se descubren tarde:

- Timezone inválida: crash en `main.go:70` en lugar de error claro al arrancar
- SESSION_KEY < 32 bytes: sesiones débiles sin aviso
- Puerto fuera de rango: fallo al bind sin contexto

**Fix**: Añadir `func (c *Config) Validate() error` que valide los campos críticos y llamarla en `main.go` justo después de `config.Load()`. Salir con mensaje descriptivo si salida falla.

---

## COBERTURA DE TESTS

### Store — sin tests
- `internal/store/pve.go` — funciones de gestión de servidores, reportes, tasks (solo las de backup tienen tests)
- `internal/store/pbs.go` — funciones de gestión PBS (solo snaps básicos)
- `internal/store/email.go` — `GetEmailConfig`, `SaveEmailConfig`

### Service — sin tests
- `internal/service/cleanup.go` — lógica de retención (solo paths de éxito)
- `internal/service/report.go → IsStaleForServer` — lógica compleja de staleness, crítica

### Web handlers — sin tests de integración
Ningún handler web tiene tests. Mínimo útil sería cubrir los handlers que tienen lógica de negocio no trivial: `PVEServerDetail`, `Dashboard`, `CreateAPIKeyPost`.

---

## DEUDA TÉCNICA MENOR

- `internal/store/pve.go` y `pbs.go` usan `s.db.Begin()` sin contexto; debería ser `s.db.BeginTx(ctx, nil)` para consistencia con el resto del store.
- El alert badge del sidebar llama a `service.ActiveAlertCounts(st)` en cada render de página (DB query). Considerar cachear con TTL de 30s si hay problemas de latencia.
- Los errores de `tx.Rollback()` se ignoran con `defer tx.Rollback()` — correcto en Go cuando hay `Commit()`, pero añadir `//nolint:errcheck` explícito o wrappear el patrón en un helper para que quede claro que es intencional.
