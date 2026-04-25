# Migraciones de Base de Datos

probakgo usa migraciones SQL embebidas en el binario que se aplican automáticamente al arrancar el servidor. No se requieren herramientas externas.

## Cómo funciona

Los archivos de migración viven en `internal/db/migrations/` y se embeben en el binario en tiempo de compilación.

Al arrancar, el servidor aplica en orden todas las migraciones que aún no se hayan ejecutado. El estado se rastrea en la tabla `schema_migrations`.

## Migraciones actuales

| Archivo | Descripción |
|---------|-------------|
| `001_initial.up.sql` | Esquema completo inicial: usuarios, api_keys, reportes PVE/PBS, backup_config, email_config |

## Añadir una nueva migración

1. Crear el archivo: `internal/db/migrations/002_descripcion.up.sql`
2. Escribir el SQL del cambio de esquema.
3. Recompilar y reiniciar el servidor - la migración se aplica sola al arrancar.

**Convención de nombres:** `NNN_descripcion.up.sql` donde `NNN` es un número secuencial con ceros a la izquierda.

## Rollback

No hay rollback automático. Para revertir:

1. Restaura el backup de `probakgo_data.db` (haz backup antes de cada actualización).
2. Despliega la versión anterior del binario.
