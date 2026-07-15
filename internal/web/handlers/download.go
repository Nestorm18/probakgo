package webhandlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const githubRepo = "Nestorm18/probakgo"

const (
	maxGitHubMetadataBytes = 2 << 20
	maxReleaseAssetBytes   = 256 << 20
)

type releaseAsset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type githubLatestRelease struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

func (h *WebH) DownloadClientLinuxAMD64(w http.ResponseWriter, r *http.Request) {
	h.downloadReleaseAsset(w, r, "probakgo-client_linux_amd64", "probakgo-client")
}

func (h *WebH) DownloadClientWindowsAMD64(w http.ResponseWriter, r *http.Request) {
	h.downloadReleaseAsset(w, r, "probakgo-windows-client_windows_amd64.exe", "probakgo-windows-client.exe")
}

func (h *WebH) downloadReleaseAsset(w http.ResponseWriter, r *http.Request, assetName, filename string) {
	token := githubDownloadToken(r)
	if token == "" {
		http.Error(w, "GitHub token requerido", http.StatusUnauthorized)
		return
	}

	assetID, err := latestAssetID(r, token, assetName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, fmt.Sprintf("https://api.github.com/repos/%s/releases/assets/%d", githubRepo, assetID), nil)
	if err != nil {
		http.Error(w, "no se pudo crear la peticion", http.StatusInternalServerError)
		return
	}
	setGitHubHeaders(req, token)
	req.Header.Set("Accept", "application/octet-stream")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "error descargando desde GitHub", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("GitHub devolvio HTTP %d", resp.StatusCode), http.StatusBadGateway)
		return
	}
	if resp.ContentLength > maxReleaseAssetBytes {
		http.Error(w, "el asset de GitHub supera el tamano permitido", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	if _, err := copyWithLimit(w, resp.Body, maxReleaseAssetBytes); err != nil {
		return
	}
}

func latestAssetID(r *http.Request, token, assetName string) (int64, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://api.github.com/repos/"+githubRepo+"/releases/latest", nil)
	if err != nil {
		return 0, fmt.Errorf("no se pudo crear la peticion")
	}
	setGitHubHeaders(req, token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error consultando GitHub")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("GitHub devolvio HTTP %d", resp.StatusCode)
	}

	var rel githubLatestRelease
	if err := decodeJSONWithLimit(resp.Body, &rel, maxGitHubMetadataBytes); err != nil {
		return 0, fmt.Errorf("error leyendo respuesta de GitHub")
	}
	for _, asset := range rel.Assets {
		if asset.Name == assetName {
			return asset.ID, nil
		}
	}
	return 0, fmt.Errorf("asset %q no encontrado en la ultima release", assetName)
}

func decodeJSONWithLimit(r io.Reader, dst any, max int64) error {
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > max {
		return fmt.Errorf("respuesta demasiado grande")
	}
	return json.Unmarshal(data, dst)
}

func copyWithLimit(dst io.Writer, src io.Reader, max int64) (int64, error) {
	n, err := io.Copy(dst, io.LimitReader(src, max+1))
	if err != nil {
		return n, err
	}
	if n > max {
		return n, fmt.Errorf("respuesta demasiado grande")
	}
	return n, nil
}

func setGitHubHeaders(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "probakgo")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

func githubDownloadToken(r *http.Request) string {
	if token := strings.TrimSpace(r.Header.Get("X-GitHub-Token")); token != "" {
		return token
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("bearer "):])
	}
	return ""
}
