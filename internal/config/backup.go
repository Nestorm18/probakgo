package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// BackupEnv exports the server's dotenv configuration with the same precedence
// as loadEnv: process environment, working directory, executable directory.
// Only known server variables are copied from the process environment.
func BackupEnv() ([]byte, error) {
	paths := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		paths = append(paths, filepath.Join(filepath.Dir(exe), ".env"))
	}
	return backupEnvFrom(paths)
}

func backupEnvFrom(paths []string) ([]byte, error) {
	values := make(map[string]string)
	for _, name := range paths {
		contents, err := godotenv.Read(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, errors.New("no se pudo leer la configuración .env para la copia")
		}
		for key, value := range contents {
			if _, exists := values[key]; !exists {
				values[key] = value
			}
		}
	}
	for _, key := range []string{"DATABASE_PATH", "API_HOST", "API_PORT", "SESSION_KEY", "DATA_ENCRYPTION_KEY", "TIMEZONE", "SESSION_SECURE", "CSRF_TRUSTED_ORIGINS", "TRUSTED_PROXY_CIDRS", "DEV", "GITHUB_TOKEN"} {
		if value, exists := os.LookupEnv(key); exists {
			values[key] = value
		}
	}
	if values["DATA_ENCRYPTION_KEY"] == "" || values["SESSION_KEY"] == "" {
		return nil, errors.New("no se pudo completar el .env de la copia: faltan las claves del servidor")
	}
	data, err := godotenv.Marshal(values)
	if err != nil {
		return nil, errors.New("no se pudo preparar el .env de la copia")
	}
	return []byte(data + "\n"), nil
}
