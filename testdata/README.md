# testdata - Fixtures de prueba

Simula la recepción de reportes sin necesitar un nodo Proxmox real.

## Requisitos

- El servidor `probakgo` en ejecución (`./probakgo`)
- Una API key `pbk-` activa (créala en la web UI → API Keys)
- `curl` instalado

## Uso

```bash
bash testdata/seed.sh http://localhost:36748 pbk-tuclaveaqui
```

O si tienes `API_KEY=pbk-...` en tu `.env`:

```bash
bash testdata/seed.sh
```

Tras ejecutarlo, abre `http://localhost:36748` y verás en el dashboard:
- 1 servidor PBS (`pbs-test`) con el datastore `synology`
- 1 servidor PVE (`soporte1`) con 3 storages y 5 backups de VMs/contenedores

## Fixtures incluidos

| Archivo | Tipo | Contenido |
|---------|------|-----------|
| `fixture_pbs.json` | PBS | 1 datastore (synology, 2.7 TB usados de 2.9 TB) |
| `fixture_pve.json` | PVE | 3 storages, VMs 100–103 + CT 200 |

Puedes ejecutar `seed.sh` varias veces para actualizar el timestamp del último reporte.

## Historial de reportes

Para poblar el gráfico de duración y la vista de historial con datos de los últimos 7 días, ejecuta después del seed normal:

```bash
go run testdata/seed_history.go
```

Inserta 6 días adicionales con distintos estados (OK / warning / error) y duraciones, sin necesitar un cliente real ni `sqlite3` instalado.
