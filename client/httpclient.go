package main

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"net/http"
	"os"
	"time"
)

func newHTTPClient(cfg *Config) *http.Client {
	tlsCfg := &tls.Config{}
	if cfg.CABundle != "" {
		caCert, err := os.ReadFile(cfg.CABundle)
		pool := x509.NewCertPool()
		if err != nil || !pool.AppendCertsFromPEM(caCert) {
			// Fall back to the system roots, but say why the bundle was ignored.
			log.Printf("WARN: PROXMOX_CA_BUNDLE %s ignored: no PEM certificates could be read (%v); using system roots", cfg.CABundle, err)
		} else {
			tlsCfg.RootCAs = pool
		}
	} else if !cfg.VerifyTLS {
		tlsCfg.InsecureSkipVerify = true //nolint:gosec
	}
	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
		Timeout:   60 * time.Second,
	}
}
