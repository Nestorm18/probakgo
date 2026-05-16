# TODO - Probakgo

- [ ] **Interfaz `Store` para facilitar mocking en tests**
  El store actual es una struct concreta. Definir una interfaz `Store` con todos los metodos publicos. Permite crear mocks en tests de handlers sin SQLite real.
  _(inspirado en: `management/server/store/store.go`)_

- [ ] **Errores tipados con mapeo automatico a HTTP status**
  Reemplazar retornos de `fmt.Errorf(...)` en handlers por tipos de error estructurados (`ErrNotFound`, `ErrUnauthorized`, `ErrConflict`) que middleware central convierte a status code correcto. Elimina el boilerplate de cada handler.
  _(inspirado en: `management/server/status/error.go`)_

- [ ] **Instalación del cliente**
  Al instalar el cliente, listar las máquinas virtuales y añadirlas automáticamente a `backup-config` con el nombre y los IDs que ya tienen.

- [ ] **Evitar errores de renderizado de plantillas**
  Resolver el error al renderizar `api_key_created.html` donde se produce un fallo en la fase exec.
  Error: `template: base.html:40:21: executing "base" at <.Role>: invalid value; expected string`

- [ ] Coordinar `update` con ejecucion activa: usar lock compartido para que `probakgo-client update` no coincida con `--vzdump-hook`, y evitar/reintentar reportes durante restart del servidor.

- [ ] Anadir tests de integracion para los handlers de la API (actualmente solo hay test helpers)

- [ ] El `cleanup.go` deberia respetar un contexto cancelable en todas sus rutas de error
