# testdata: fixtures PVE/PBS

Payloads de referencia para probar reportes sin consultar una API Proxmox real.

## Contenido

| Archivo | Tipo | Hostname / Machine ID | Datos |
|---|---|---|---|
| `fixture_pve.json` | PVE | `soporte1` / `11223344-5566-7788-99aa-bbccddeeff00` | Storages, contenidos y último backup |
| `fixture_pbs.json` | PBS | `pbs-test` / `aabbccdd-eeff-0011-2233-445566778899` | Datastore, histórico y GC |
| `seed_history.go` | SQLite | `soporte1` y `pbs-test` | Seis días adicionales de históricos |
| `seed.sh` | Script de carga HTTP | ambos | Envía ambos fixtures con claves y Machine ID independientes |

Los JSON no cubren todas las funciones recientes: no incluyen heartbeat separado, tareas PVE del último job, swap ni tareas PBS de sync/GC.

## Requisitos de la API actual

Cada petición necesita:

- una API key `pbk-` activa;
- `X-Machine-ID`;
- un hostname igual al asociado a la key;
- una key distinta por equipo.

`seed.sh` cumple estos requisitos: acepta una key PVE y otra PBS, envía `X-Machine-ID` y mantiene una identidad distinta para cada fixture.

## Carga rápida

```bash
bash testdata/seed.sh \
  http://localhost:36748 \
  pbk-CLAVE-PVE \
  pbk-CLAVE-PBS
```

También puede leer `API_URL`, `PVE_API_KEY` y `PBS_API_KEY` desde `.env`.

## Cargar los fixtures manualmente

1. Arranca Probakgo.
2. Crea dos API keys:
   - hostname `soporte1`;
   - hostname `pbs-test`.
3. Envía cada fixture con su key y Machine ID.

```bash
curl -fS -X POST http://localhost:36748/api/report/pve \
  -H "Authorization: Bearer pbk-CLAVE-PVE" \
  -H "X-Machine-ID: 11223344-5566-7788-99aa-bbccddeeff00" \
  -H "Content-Type: application/json" \
  --data-binary @testdata/fixture_pve.json

curl -fS -X POST http://localhost:36748/api/report/pbs \
  -H "Authorization: Bearer pbk-CLAVE-PBS" \
  -H "X-Machine-ID: aabbccdd-eeff-0011-2233-445566778899" \
  -H "Content-Type: application/json" \
  --data-binary @testdata/fixture_pbs.json
```

Después abre `http://localhost:36748`. Deben aparecer:

- PVE `soporte1`, con cuatro storages de ejemplo y backups de VM/CT;
- PBS `pbs-test`, con el datastore `synology`.

## Añadir histórico

Con ambos servidores ya creados:

```bash
go run testdata/seed_history.go
```

El programa:

- lee `DATABASE_PATH` de `.env` o usa `probakgo_data.db`;
- busca `soporte1` y `pbs-test`;
- inserta seis días adicionales con estados/duraciones PVE y crecimiento PBS.

Detén el servidor antes de manipular directamente la base si quieres evitar que la UI o los schedulers lean datos a mitad de la carga. Hazlo solo sobre una base de laboratorio.

## Limpieza

Usa **Configuración → Reiniciar BD** después de descargar una copia si necesitas conservar el escenario. Los usuarios se mantienen.
