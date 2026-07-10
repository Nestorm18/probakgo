# Releases

Checklist para publicar una version de Probakgo.

## Antes de publicar

1. Actualiza la version en `main.go`, `client/main.go` y `client-windows/main.go`.
2. Ejecuta tests completos:

```bash
go test ./...
go vet ./...
```

3. Compila binarios sin CGO:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=<version>" -o probakgo_linux_amd64 .
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=<version>" -o probakgo-client_linux_amd64 ./client
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=<version>" -o probakgo-windows-client_windows_amd64.exe ./client-windows
```

4. Prueba version y diagnostico:

```bash
./probakgo_linux_amd64 version
./probakgo-client_linux_amd64 version
./probakgo-client_linux_amd64 doctor
```

En Windows:

```powershell
.\probakgo-windows-client_windows_amd64.exe version
.\probakgo-windows-client_windows_amd64.exe doctor
```

5. Revisa migraciones nuevas en una copia de `probakgo_data.db`.

## Publicacion

1. Crea y sube el tag `v<version>` para publicar automáticamente:

```bash
git tag v<version>
git push origin v<version>
```

También puedes ejecutar `Release` manualmente desde GitHub Actions e introducir ese mismo tag en el campo `tag`.
2. El workflow `Release` publica la GitHub Release. Debe incluir estos assets:
   - `probakgo_linux_amd64`
   - `probakgo-client_linux_amd64`
   - `probakgo-windows-client_windows_amd64.exe`
   - `SHA256SUMS`
3. Verifica que `/about/update`, `probakgo-client update` y la descarga de Windows desde la pantalla de API key usan la nueva version.
4. Actualiza primero nodos no criticos y revisa:
   - `probakgo-client doctor`
   - heartbeat en la web
   - envio de reporte tras backup o `--vzdump-hook`
   - Windows: `C:\ProgramData\Probakgo\probakgo-windows-client.log`

## Rollback

1. Descarga el asset de la release anterior.
2. Sustituye el binario afectado:

```bash
systemctl stop probakgo
cp probakgo_linux_amd64 /opt/probakgo/probakgo
systemctl start probakgo
```

Cliente Proxmox:

```bash
cp probakgo-client_linux_amd64 /opt/probakgo/probakgo-client
/opt/probakgo/probakgo-client doctor
```

Cliente Windows, desde PowerShell como administrador:

```powershell
Copy-Item .\probakgo-windows-client_windows_amd64.exe C:\ProgramData\Probakgo\probakgo-windows-client.exe -Force
C:\ProgramData\Probakgo\probakgo-windows-client.exe doctor
```

3. Si una migracion nueva ya se aplico, restaura una copia previa de `probakgo_data.db` o una copia completa de la VM.
