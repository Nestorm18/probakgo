# TODO - Probakgo

- [ ] **Interfaz `Store` para facilitar mocking en tests**
  El store actual es una struct concreta. Definir una interfaz `Store` con todos los metodos publicos. Permite crear mocks en tests de handlers sin SQLite real.

- [ ] **Instalación del cliente**
  Al instalar el cliente, listar las máquinas virtuales y añadirlas automáticamente a `backup-config` con el nombre y los IDs que ya tienen.

- [ ] Coordinar `update` con ejecucion activa: usar lock compartido para que `probakgo-client update` no coincida con `--vzdump-hook`, y evitar/reintentar reportes durante restart del servidor.

- [ ] Anadir tests de integracion para los handlers de la API (actualmente solo hay test helpers)