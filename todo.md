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



## MEJORA: Propagación de contexto en servicios

`internal/service/report.go` y `internal/service/alertengine.go` crean `context.Background()` internamente en lugar de recibir el contexto de quien llama.

Consecuencias:
- Las operaciones de DB no pueden cancelarse si el request HTTP se cancela
- No se pueden añadir timeouts por request
- Bloquea instrumentación futura (tracing, logging con request-id)

**Fix**: Cambiar firmas para que los métodos reciban `ctx context.Context` como primer parámetro y lo propagen a las llamadas de store. Empezar por `report.SavePVEReport` y `report.SavePBSReport` ya que son los puntos de entrada del cliente.

---




---

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
