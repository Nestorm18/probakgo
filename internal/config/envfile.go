package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// SetEnvFileValue sets KEY=value in a dotenv file, creating it when missing.
// The file is replaced atomically: a crash or a full disk cannot truncate the
// existing keys, including DATA_ENCRYPTION_KEY.
func SetEnvFileValue(path, key, value string) error {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeFileAtomic(path, []byte(SetDotEnvValue(string(data), key, value)), 0600)
}

// SetDotEnvValue returns content with KEY=value replaced or appended on its
// own line, even when the content does not end with a newline.
func SetDotEnvValue(content, key, value string) string {
	line := fmt.Sprintf("%s=%s", key, value)
	if strings.TrimSpace(content) == "" {
		return line + "\n"
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	found := false
	for i, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, key+"=") {
			lines[i] = line
			found = true
		}
	}
	if !found {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines[len(lines)-1] = line
			lines = append(lines, "")
		} else {
			lines = append(lines, line, "")
		}
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()
	if err = tmp.Chmod(perm); err != nil {
		return err
	}
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = preserveOwner(path, tmpName); err != nil {
		return err
	}
	if err = os.Rename(tmpName, path); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if d, dirErr := os.Open(dir); dirErr == nil {
			_ = d.Sync()
			_ = d.Close()
		}
	}
	return nil
}
