# TODO - Probakgo

## Produccion / operacion

- [x] Copia descargable de `probakgo_data.db` desde la web, probablemente en `/settings/maintenance`.
- [x] Mejorar la documentacion de releases: checklist de publicar binarios, version, pruebas y rollback.

## Seguridad

- [x] 2FA/TOTP para usuarios web, especialmente si la aplicacion queda expuesta por HTTPS a internet.
- [x] Pagina de sesiones activas / ultimos accesos por usuario. Se puede mejorar en settings/audit-log 
- [x] Aviso visible si `SESSION_SECURE=false` y se accede por HTTPS publico.

## Alertas

- [x] Notificacion inmediata opcional para alertas criticas, separada del informe diario por email. Dentro de  /settings/email añadir checkbox para alertas criticas (servidor offine por ejemplo) debajo de "Habilitar envío de reportes diarios" o en otra caja dentro de esa url.
- [x] Historial de cambios de estado de alerta: cuando aparece, cuando se resuelve, cuando se suprime.

## Calidad de vida

- [x] Export CSV/JSON desde vistas principales: alertas, PVE, PBS y reportes historicos.
- [x] Filtro global por cliente/empresa cuando haya muchos nodos.
- [x] Accion rapida en servidor: copiar comandos utiles de diagnostico (`doctor`, logs, timers).
