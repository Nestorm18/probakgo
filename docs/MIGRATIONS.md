# Gestión de Migraciones de Base de Datos

Este proyecto utiliza **Alembic** para gestionar las migraciones de la base de datos. Esto permite actualizar la estructura de la base de datos en producción de manera segura y controlada.

## Configuración Inicial

El sistema de migraciones ya ha sido inicializado. Los archivos de configuración se encuentran en la carpeta `alembic/` y el archivo `alembic.ini` en la raíz del proyecto.

## Comandos Básicos

### 1. Crear una nueva migración

Cuando realices cambios en los modelos (`models.py`), debes generar un script de migración que refleje esos cambios en la base de datos.

```bash
uv run alembic revision --autogenerate -m "Descripción del cambio"
```

Esto creará un nuevo archivo en `alembic/versions/` con las instrucciones SQL necesarias.

**Importante:** Revisa siempre el archivo generado para asegurarte de que los cambios son correctos.

### 2. Aplicar migraciones (Actualizar base de datos)

Para aplicar los cambios pendientes a la base de datos (tanto en desarrollo como en producción):

```bash
uv run alembic upgrade head
```

Este comando ejecutará todas las migraciones que aún no se hayan aplicado.

### 3. Ver historial de migraciones

Para ver el historial de migraciones y cuál es la actual:

```bash
uv run alembic history
uv run alembic current
```

## Flujo de Trabajo en Producción

1.  **Desarrollo:**
    *   Modifica `models.py`.
    *   Genera la migración: `uv run alembic revision --autogenerate -m "..."`.
    *   Aplica la migración localmente: `uv run alembic upgrade head`.
    *   Verifica que todo funcione.
    *   Haz commit de los cambios (incluyendo el nuevo archivo en `alembic/versions/`).

2.  **Despliegue (Producción):**
    *   Descarga el código actualizado (git pull).
    *   Ejecuta las migraciones: `uv run alembic upgrade head`.
    *   Reinicia el servidor si es necesario.

## Estado Actual

Se ha generado una migración inicial (`alembic/versions/...initial_migration.py`) que detecta el estado actual de la base de datos y lo sincroniza con los modelos definidos en `models.py`.

Si ejecutas `uv run alembic upgrade head` ahora, se aplicarán estos cambios (por ejemplo, eliminando tablas antiguas no utilizadas como `backup_config` si existen).
