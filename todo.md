# TODO - probakgo

---

## Bugs

- [x] **`download.go` referencia `client.py`** — resto del predecessor Python. El endpoint `/download/latest` devuelve 404 siempre. Eliminados `/download/latest` y `/download/latest-metadata`; el cliente usa self-update vía GitHub releases.

---

## Seguridad

- [x] **Content-Security-Policy** — añadida CSP en `securityHeaders`: permite scripts/estilos de jsdelivr y self, bloquea todo lo demás.
- [x] **CSRF con proxy inverso** — `CSRF_TRUSTED_ORIGINS` propagado desde config a `csrf.Protect`; documentado en `.env.example`.
- [x] **[HIGH] IDOR endpoints QR** — resuelto eliminando el feature QR completo.
- [x] **[MEDIUM] Path sin comillas en cron/systemd** — path del ejecutable entrecomillado en `ensureUpdateCron` y `ensureSystemdService`.
- [x] **[LOW] API key completa en logs** — sustituido por preview (`service.KeyPreview`) con instrucción de ir a la UI.

---

## Operacional

- [ ] **Backup de la BD** — SQLite es un único fichero, no hay ningún cron que lo respalde. Añadir copia diaria antes del auto-update o documentar cómo hacerlo.

---

## Limpieza

- [x] **`docs/INSTALL_SERVER.md` obsoleto** — supersedido por `INSTALLATION.md`. Borrar o redirigir para evitar confusión.
- [x] **Eliminar QR code** — las páginas `/api-keys/{id}/qr` y `/api-keys/{id}/qr-image` no se usan en el flujo normal. Eliminar rutas, handlers (`QRPage`, `QRImageServe`), template `qr_code.html`, dependencia `github.com/skip2/go-qrcode`, y cualquier enlace en templates. Resuelve también el IDOR de seguridad anterior.
