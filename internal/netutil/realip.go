package netutil

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// TrustedProxyRealIP uses forwarding headers only when the direct peer belongs
// to a configured trusted proxy network. Direct clients cannot spoof their IP.
func TrustedProxyRealIP(rawCIDRs []string) func(http.Handler) http.Handler {
	trusted := parseTrustedProxyCIDRs(rawCIDRs)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer, ok := remoteAddrIP(r.RemoteAddr)
			if !ok || !isTrustedProxy(peer, trusted) {
				clearForwardedHeaders(r)
				next.ServeHTTP(w, r)
				return
			}

			if client, ok := forwardedClientIP(r, trusted); ok {
				r.RemoteAddr = net.JoinHostPort(client.String(), "0")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func parseTrustedProxyCIDRs(rawCIDRs []string) []netip.Prefix {
	trusted := make([]netip.Prefix, 0, len(rawCIDRs))
	for _, raw := range rawCIDRs {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err == nil {
			trusted = append(trusted, prefix.Masked())
		}
	}
	return trusted
}

func remoteAddrIP(remoteAddr string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	addr, err := netip.ParseAddr(strings.Trim(host, "[]"))
	return addr, err == nil
}

func isTrustedProxy(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func forwardedClientIP(r *http.Request, trusted []netip.Prefix) (netip.Addr, bool) {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			addr, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
			if err == nil && !isTrustedProxy(addr, trusted) {
				return addr, true
			}
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		addr, err := netip.ParseAddr(realIP)
		if err == nil {
			return addr, true
		}
	}
	return netip.Addr{}, false
}

func clearForwardedHeaders(r *http.Request) {
	r.Header.Del("X-Forwarded-For")
	r.Header.Del("X-Real-IP")
	r.Header.Del("X-Forwarded-Host")
	r.Header.Del("X-Forwarded-Proto")
}
