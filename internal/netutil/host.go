package netutil

import (
	"net"
	"net/http"
	"strings"
)

// HostFromRequest returns the externally visible request host, honoring common
// reverse proxy headers before falling back to r.Host.
func HostFromRequest(r *http.Request) string {
	host := r.Host
	if forwarded := r.Header.Get("X-Forwarded-Host"); forwarded != "" {
		host = strings.Split(forwarded, ",")[0]
	}
	return normalizeHost(host)
}

func RequestScheme(r *http.Request) string {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return strings.ToLower(strings.TrimSpace(strings.Split(proto, ",")[0]))
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func HostLooksPublic(host string) bool {
	host = normalizeHost(host)
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || isCGNAT(ip))
	}
	return strings.Contains(host, ".")
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(strings.Split(host, ",")[0])
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.Trim(host, "[]")
}

func isCGNAT(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127
}
