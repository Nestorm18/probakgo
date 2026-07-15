package webhandlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"probakgo/internal/netutil"
)

func normalizePublicAPIURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.ContainsAny(raw, "\r\n\t") {
		return "", fmt.Errorf("URL publica no valida")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", fmt.Errorf("URL publica no valida")
	}
	if u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("la URL publica debe contener solo esquema, host y puerto")
	}
	if err := validateURLPort(u); err != nil {
		return "", err
	}
	return u.Scheme + "://" + u.Host, nil
}

func installerAPIURL(r *http.Request, configured string) (string, error) {
	if strings.TrimSpace(configured) != "" {
		return normalizePublicAPIURL(configured)
	}

	scheme := netutil.RequestScheme(r)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("esquema de acceso no valido")
	}
	host := strings.TrimSpace(r.Host)
	if host == "" || strings.ContainsAny(host, "\r\n\t/\\") {
		return "", fmt.Errorf("host de acceso no valido")
	}
	u, err := url.Parse("http://" + host)
	if err != nil || u.User != nil || u.Hostname() == "" || u.Path != "" {
		return "", fmt.Errorf("host de acceso no valido")
	}
	if err := validateURLPort(u); err != nil {
		return "", err
	}
	if netutil.HostLooksPublic(u.Hostname()) {
		return "", fmt.Errorf("configura la URL publica antes de generar instaladores desde un dominio publico")
	}
	return scheme + "://" + u.Host, nil
}

func validateURLPort(u *url.URL) error {
	port := u.Port()
	if port == "" {
		return nil
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("puerto de URL no valido")
	}
	return nil
}
