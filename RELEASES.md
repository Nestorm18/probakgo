# Releases

Proceso actual para publicar servidor, cliente Proxmox y cliente Windows.

## Flujo automático

El workflow `CI` se ejecuta en cada push y pull request:

1. usa Go 1.26.5;
2. lee la única versión desde `internal/version/version.go`;
3. ejecuta `go build ./...`, `go vet ./...` y la suite con perfil de cobertura;
4. exige al menos un 35 % de cobertura global.

Después de un push correcto a `master`, CI llama al workflow `Release` con:

- tag `v<versión-del-código>`;
- SHA exacto del push.

Si esa release ya existe, el workflow no vuelve a publicarla. Por eso todo cambio de código que deba generar una release necesita una versión nueva.

`Release` también se puede ejecutar:

- manualmente con `workflow_dispatch`, indicando un tag;
- al subir un tag `v*`.

En todos los casos, el tag debe tener formato `vX.Y.Z` y coincidir con `internal/version.Version`.

El job usa el entorno de GitHub `release`. Configura en el repositorio sus *required reviewers* si quieres aprobación humana antes de publicar.

## Antes de publicar

### 1. Actualizar versión

Modifica una sola vez:

- `internal/version/version.go`

### 2. Verificar el árbol

```bash
go build ./...
go vet ./...
go test ./...
git status --short
```

### 3. Probar los binarios de release

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w -X probakgo/internal/version.Version=<version>" \
  -o probakgo_linux_amd64 .

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w -X probakgo/internal/version.Version=<version>" \
  -o probakgo-client_linux_amd64 ./client

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w -X probakgo/internal/version.Version=<version>" \
  -o probakgo-windows-client_windows_amd64.exe ./client-windows
```

Linux:

```bash
./probakgo_linux_amd64 version
./probakgo-client_linux_amd64 version
./probakgo-client_linux_amd64 doctor
```

Windows:

```powershell
.\probakgo-windows-client_windows_amd64.exe version
.\probakgo-windows-client_windows_amd64.exe doctor
```

### 4. Revisar migraciones

- Comprueba que cada migración nueva está en `internal/db/migrations/`.
- Prueba una base vacía.
- Prueba una copia reciente de producción.
- Conserva una copia de `probakgo_data.db` y `.env` antes de desplegar.

## Publicación

La vía habitual es fusionar o subir el commit listo a `master`. CI crea la release automáticamente.

Publicación explícita por tag:

```bash
git tag v<version>
git push origin v<version>
```

El workflow debe publicar exactamente:

- `probakgo_linux_amd64`
- `probakgo-client_linux_amd64`
- `probakgo-windows-client_windows_amd64.exe`
- `SHA256SUMS`

Las acciones de GitHub están fijadas por SHA y el workflow valida que el tag coincide con el código antes de compilar. Antes de crear la release, `actions/attest` firma la procedencia de los tres binarios usando sus entradas de `SHA256SUMS`.

## Comprobación posterior

1. Revisa la página de la release y los cuatro assets.
2. Compara localmente los hashes con `SHA256SUMS`.
3. Verifica la procedencia firmada:

```bash
gh attestation verify --repo Nestorm18/probakgo probakgo_linux_amd64
gh attestation verify --repo Nestorm18/probakgo probakgo-client_linux_amd64
gh attestation verify --repo Nestorm18/probakgo probakgo-windows-client_windows_amd64.exe
```

4. Comprueba el servidor:

```bash
/opt/probakgo/probakgo update
/opt/probakgo/probakgo version
/opt/probakgo/probakgo doctor
```

5. Actualiza primero un PVE/PBS no crítico:

```bash
probakgo-client update
probakgo-client version
probakgo-client doctor
```

6. Verifica heartbeat, reporte PVE/PBS y logs.
7. En Windows:

```powershell
C:\ProgramData\Probakgo\probakgo-windows-client.exe update
C:\ProgramData\Probakgo\probakgo-windows-client.exe version
C:\ProgramData\Probakgo\probakgo-windows-client.exe doctor
Get-Content C:\ProgramData\Probakgo\probakgo-windows-client.log -Tail 80
```

8. Confirma desde la web:

- versión en **Acerca de**;
- clientes y heartbeat;
- alertas y modo mantenimiento;
- envío SMTP de prueba;
- descarga Linux/Windows desde la pantalla de API key.

El actualizador y los endpoints de descarga verifican tamaño y checksum. Un fallo en `SHA256SUMS` debe bloquear la sustitución.

## Rollback

Las migraciones solo avanzan. Antes de retroceder un servidor, determina si la versión anterior entiende el esquema ya migrado.

### Servidor

```bash
systemctl stop probakgo
cp /ruta/probakgo_linux_amd64_anterior /opt/probakgo/probakgo
chmod +x /opt/probakgo/probakgo
chown probakgo:probakgo /opt/probakgo/probakgo
systemctl start probakgo
/opt/probakgo/probakgo doctor
```

### Cliente Proxmox

```bash
cp /ruta/probakgo-client_linux_amd64_anterior /opt/probakgo/probakgo-client
chmod +x /opt/probakgo/probakgo-client
probakgo-client doctor
```

### Cliente Windows

PowerShell como administrador:

```powershell
schtasks /End /TN "Probakgo Windows Report"
schtasks /End /TN "Probakgo Windows Update"
Copy-Item .\probakgo-windows-client_windows_amd64.exe `
  C:\ProgramData\Probakgo\probakgo-windows-client.exe -Force
C:\ProgramData\Probakgo\probakgo-windows-client.exe doctor
```

Si una migración incompatible ya se aplicó, restaura la copia previa de SQLite y `.env` o la copia completa de la VM.
