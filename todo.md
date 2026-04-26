# TODO - probakgo

---

## Seguridad (pendiente)

- [ ] **Open redirect en login** — `?next=` no valida que sea una ruta relativa interna; un atacante puede redirigir a `?next=https://evil.com`. Validar que empiece por `/` y no por `//`.
- [ ] **Cookie `Secure` flag** — las sesiones no tienen `Secure: true`; si el servidor se expone con HTTPS las cookies viajan sin protección. Activar en producción o documentar que se requiere proxy HTTPS.
- [ ] **SESSION_KEY inseguro por defecto** — si `.env` no define `SESSION_KEY` se usa un valor fijo del código. Generar uno aleatorio al arrancar si no está configurado, o fallar con error claro.
- [ ] **Sin CSRF en formularios web** — todos los POST de la UI (cambio de contraseña, usuarios, API keys, etc.) carecen de token CSRF. Añadir middleware CSRF (p.ej. `gorilla/csrf`).
- [ ] **Detalles de error expuestos en API** — algunos handlers devuelven `err.Error()` directamente al cliente. Sustituir por mensajes genéricos y loguear el detalle en servidor.
- [ ] **Sin cabeceras de seguridad HTTP** — no se emiten `X-Frame-Options`, `X-Content-Type-Options`, `Content-Security-Policy`, etc. Añadir middleware de cabeceras seguras.
- [ ] **Sin rate limiting** — el endpoint `/login` y todos los endpoints de API no tienen límite de peticiones. Riesgo de fuerza bruta y abuso.
- [ ] **Verificación SHA256 en selfupdate** — el proceso de auto-actualización descarga el binario sin verificar el hash contra `SHA256SUMS`. Si el repositorio es público, añadir verificación de firma/hash.

---

## CI — version enforcement

- [ ] Añadir step en `ci.yml` que falle si `var version` en `main.go` y `client/main.go` coincide con el último git tag (recordatorio de bumpar versión antes de cada release).
- [ ] Verificar también que `var version` es idéntica en servidor y cliente.
