package handlers

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const clientBinaryName = "client.py"

func (h *H) DownloadMetadata(w http.ResponseWriter, r *http.Request) {
	info, err := os.Stat(clientBinaryName)
	if err != nil {
		errJSON(w, http.StatusNotFound, "client binary not available")
		return
	}
	f, err := os.Open(clientBinaryName)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "cannot read client binary")
		return
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		errJSON(w, http.StatusInternalServerError, "hash error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"filename": filepath.Base(clientBinaryName),
		"sha256":   fmt.Sprintf("%x", hash.Sum(nil)),
		"mtime":    info.ModTime().Format(time.RFC3339),
		"size":     info.Size(),
	})
}

func (h *H) DownloadLatest(w http.ResponseWriter, r *http.Request) {
	f, err := os.Open(clientBinaryName)
	if err != nil {
		errJSON(w, http.StatusNotFound, "client binary not available")
		return
	}
	defer f.Close()
	info, _ := f.Stat()
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(clientBinaryName)))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	_, _ = io.Copy(w, f)
}
