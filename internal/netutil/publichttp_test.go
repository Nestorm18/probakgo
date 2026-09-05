package netutil

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

func TestPublicDialRejectsMixedDNSAndPinsAddress(t *testing.T) {
	for _, blocked := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "10.0.0.1", "192.168.1.1", "172.16.1.1", "169.254.169.254", "100.100.100.200", "168.63.129.16", "fc00::1", "fe80::1", "64:ff9b::7f00:1", "2002:7f00:1::1"} {
		t.Run(blocked, func(t *testing.T) {
			called := false
			_, err := dialPublicIPs(context.Background(), "tcp", "443", []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr(blocked)}, func(context.Context, string, string) (net.Conn, error) { called = true; return nil, nil })
			if err == nil || called {
				t.Fatal("connected despite private DNS answer")
			}
		})
	}
	var target string
	_, _ = dialPublicIPs(context.Background(), "tcp", "443", []netip.Addr{netip.MustParseAddr("2606:4700:4700::1111")}, func(_ context.Context, _, address string) (net.Conn, error) {
		target = address
		return nil, errors.New("test dial")
	})
	if target != "[2606:4700:4700::1111]:443" {
		t.Fatalf("did not pin numeric address: %s", target)
	}
}

type redirectTransport func(*http.Request) (*http.Response, error)

func (f redirectTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPublicHTTPSClientDoesNotFollowRedirect(t *testing.T) {
	client := PublicHTTPSClient()
	if client.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("untrusted destination can use environment proxy")
	}
	requests := 0
	client.Transport = redirectTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"http://127.0.0.1/private"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})
	resp, err := client.Get("https://example.com/push")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if requests != 1 || resp.StatusCode != http.StatusFound {
		t.Fatal("followed untrusted redirect")
	}
}
