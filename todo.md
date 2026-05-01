# TODO - probakgo

---

## Bugs

- [x] **`download.go` referencia `client.py`** — resto del predecessor Python. El endpoint `/download/latest` devuelve 404 siempre. Eliminados `/download/latest` y `/download/latest-metadata`; el cliente usa self-update vía GitHub releases.

---

## Seguridad

- [ ] **Content-Security-Policy** — faltan las cabeceras CSP. `X-Frame-Options` y `X-Content-Type-Options` ya están, pero CSP es la más efectiva. Bootstrap e Icons se sirven desde jsdelivr, hay que incluirlos en la política.
- [ ] **CSRF con proxy inverso** — gorilla/csrf valida la cabecera `Referer` en HTTPS. Si se accede por IP y por dominio a la vez puede rechazar formularios. Configurar `csrf.TrustedOrigins` si se usa nginx.
- [ ] **[HIGH] IDOR endpoints QR** — `/api-keys/{id}/qr` y `/api-keys/{id}/qr-image` solo requieren `RequireLogin`, no `RequireAdmin`. Un usuario `reader` puede obtener la clave completa de cualquier API key cambiando el `{id}`. Mover a `RequireAdmin` o eliminar (ver tarea de limpieza abajo).
- [ ] **[MEDIUM] Path sin comillas en cron/systemd** — `main.go:ensureUpdateCron` y `ensureSystemdService` interpolan `os.Executable()` directamente en el fichero cron y en `ExecStart=` sin comillas. Si el path contiene espacios, el servicio/cron se rompe. Envolver el path entre comillas.
- [ ] **[LOW] API key completa en logs** — en el primer arranque, `main.go:ensureDefaults` loguea la clave `adm-...` completa con `slog.Warn`. Queda en `journalctl` accesible a otros usuarios del servidor. Mostrar solo el prefijo o indicar dónde consultarla en la UI.

---

## Operacional

- [ ] **Backup de la BD** — SQLite es un único fichero, no hay ningún cron que lo respalde. Añadir copia diaria antes del auto-update o documentar cómo hacerlo.

---

## Limpieza

- [x] **`docs/INSTALL_SERVER.md` obsoleto** — supersedido por `INSTALLATION.md`. Borrar o redirigir para evitar confusión.
- [x] **Eliminar QR code** — las páginas `/api-keys/{id}/qr` y `/api-keys/{id}/qr-image` no se usan en el flujo normal. Eliminar rutas, handlers (`QRPage`, `QRImageServe`), template `qr_code.html`, dependencia `github.com/skip2/go-qrcode`, y cualquier enlace en templates. Resuelve también el IDOR de seguridad anterior.
