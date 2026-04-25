# TODO — Migración Python → Go

Estado general: **completa**.

---

## API

- ~~`GET /download/scripts-metadata`~~ — obsoleto: cliente Go, despliegue via CI/CD
- ~~`GET /download/script/{filename}`~~ — obsoleto: mismo motivo
- [x] `POST /api-keys/{id}/reveal` — modal con contraseña + AJAX

---

## Web UI

- [x] `GET/POST /api-keys/{id}/edit` — formulario de edición
- ~~`GET /api-keys/show-new`~~ — cubierto: CreateAPIKeyPost renderiza `api_key_created.html`
- [x] `GET /servers/pve/{id}/reports` — reportes históricos con filtro, storages, gráfico
- ~~`POST /settings/email/send-report`~~ — cubierto por `GET /settings/email/test`

---

## Funcionalidad

- ~~**Auto-updater**~~ — obsoleto: CI/CD + cliente Go
- [x] **Vista histórica PVE**: `/servers/pve/{id}/reports?days=N`
- ~~**Envío manual de email**~~ — cubierto por `/settings/email/test`
- [x] **Reveal de API key**: modal con contraseña + AJAX
- [x] **Roles 3-tier**: reader / editor / admin (migración DB en 001_initial.up.sql)
- [x] **Migración cliente**: `client/` en Go — reemplaza `client.py`

---

## Cliente (`client/`)

- [x] `client/config.go` — carga .env, struct Config
- [x] `client/sysinfo.go` — hostname, IP, machine-id, detección PVE/PBS
- [x] `client/pve.go` — cliente API PVE, generación de reporte
- [x] `client/pbs.go` — cliente API PBS, generación de reporte
- [x] `client/sender.go` — POST al servidor Probaky
- [x] `client/httpclient.go` — TLS configurable (skip/CA bundle)
- [x] `client/main.go` — CLI flags, orquestación
- [x] `scripts/install_client.sh` — simplificado: solo copia binario, genera token Proxmox, escribe .env
- [x] `scripts/vzdump_client.sh` — simplificado: llama a `./probaky-client --vzdump-hook`

---

## Entorno de despliegue

- **Servidor:** Debian — binario `probaky` directo, sin Docker
- **Cliente:** Debian en nodo Proxmox (PVE o PBS) — instalado via `probaky-client install`

---

## Actualizaciones automáticas — 4 problemas a resolver

El paquete `internal/selfupdate/selfupdate.go` está bien implementado internamente pero el sistema completo está roto en 4 puntos:

### Problema 1 — Workflows no se disparan solos
`.github/workflows/ci.yml` y `release.yml` usan `on: workflow_dispatch` (solo manual).
```yaml
# Cambiar a:
on:
  push:
    tags: ['v*']   # se dispara solo al hacer: git tag v1.2.3 && git push --tags
```

### Problema 2 — `release.yml` no compila el cliente Go
Solo compila el servidor (`probaky-linux-amd64`) y sube `client.py` (Python, obsoleto).
Hay que añadir:
```yaml
- name: Build client Linux amd64
  run: |
    GOOS=linux GOARCH=amd64 go build \
      -ldflags="-s -w -X main.version=${{ github.ref_name }}" \
      -o probaky-client_linux_amd64 ./client/
```
Y añadirlo a la lista de `files:` del release.

### Problema 3 — Nombres de assets incorrectos
`selfupdate.go` busca el asset con formato `{nombre}_{os}_{arch}` (ej: `probaky-client_linux_amd64`).
El release actual sube `probaky-linux-amd64` (guión en vez de `_`, sin separar "client").
Hay que renombrar los outputs de build para que coincidan exactamente.

### Problema 4 — `selfupdate.Run()` no está llamado desde ningún binario
- `client/install.go` línea 155 ya menciona `probaky-client update` en el help, pero ese subcomando **no existe** en `client/main.go`.
- El servidor tampoco lo llama.

**Tareas:**
- [ ] Corregir `release.yml`: trigger por tag, compilar cliente Go, nombres de asset correctos, eliminar `client.py`
- [ ] Añadir subcomando `update` en `client/main.go` → `selfupdate.Run("Nestorm18/probaky", "probaky-client", version)`
- [ ] (Opcional) Añadir `probaky --update` en servidor o check automático al arrancar

---

## Web UI — Auditoría (2026-04-25)

- [x] 36 rutas implementadas, 17 templates completos, sin stubs ni TODOs
- [x] Gráfico de duración en `/servers/pve/{id}/reports` — Chart.js 4.4.0 via CDN, canvas renderizado
- [x] Todos los handlers con datos reales (no placeholders)
- [x] Roles 3-tier, modales de confirmación, flash messages, tema claro/oscuro

---

## Pendiente / Futuro

- [ ] Limpieza automática de reportes con más de 3 meses en BD
- [ ] Tests unitarios para server y cliente
- [ ] Simular recepción de métricas sin cliente real (fixtures de prueba para integración)
- [ ] CD/CI: pipeline de build para Debian — compilar `probaky` (server) y `probaky-client` y subir como GitHub Release assets (`probaky_linux_amd64`, `probaky-client_linux_amd64`)