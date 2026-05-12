# TODO — Probakgo

Tareas pendientes ordenadas por área y prioridad.

## DEUDA TÉCNICA MENOR

- `internal/store/pve.go` y `pbs.go` usan `s.db.Begin()` sin contexto; debería ser `s.db.BeginTx(ctx, nil)` para consistencia con el resto del store.
- El alert badge del sidebar llama a `service.ActiveAlertCounts(st)` en cada render de página (DB query). Considerar cachear con TTL de 30s si hay problemas de latencia.
- Los errores de `tx.Rollback()` se ignoran con `defer tx.Rollback()` — correcto en Go cuando hay `Commit()`, pero añadir `//nolint:errcheck` explícito o wrappear el patrón en un helper para que quede claro que es intencional.
