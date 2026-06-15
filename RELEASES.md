# Releases

Checklist para publicar una version de Probakgo.

## Antes de publicar

1. Actualiza la version en `main.go` y `client/main.go`.
2. Ejecuta tests completos:

```bash
go test ./...
```

3. Compila binarios Linux sin CGO para evitar problemas de GLIBC en Proxmox antiguos:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=<version>" -o probakgo_linux_amd64 .
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=<version>" -o probakgo-client_linux_amd64 ./client
```

4. Prueba el servidor y el cliente:

```bash
./probakgo_linux_amd64 version
./probakgo-client_linux_amd64 version
./probakgo-client_linux_amd64 doctor
```

5. Revisa migraciones nuevas en una copia de `probakgo_data.db`.

## Publicacion

1. Crea el tag `v<version>`.
2. Publica una GitHub Release con estos assets:
   - `probakgo_linux_amd64`
   - `probakgo-client_linux_amd64`
3. Verifica que `/about/update` y `probakgo-client update` descargan la nueva version.
4. Actualiza primero un nodo no critico y revisa:
   - `probakgo-client doctor`
   - heartbeat en la web
   - envio de reporte tras backup o `--vzdump-hook`

## Rollback

1. Descarga el asset de la release anterior.
2. Sustituye el binario afectado:

```bash
systemctl stop probakgo
cp probakgo_linux_amd64 /opt/probakgo/probakgo
systemctl start probakgo
```

Para cliente:

```bash
cp probakgo-client_linux_amd64 /opt/probakgo/probakgo-client
/opt/probakgo/probakgo-client doctor
```

3. Si una migracion nueva ya se aplico, restaura una copia previa de `probakgo_data.db` o una copia completa de la VM.
