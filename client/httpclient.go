package main

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"time"
)

func newHTTPClient(cfg *Config) *http.Client {
	tlsCfg := &tls.Config{}
	if cfg.CABundle != "" {
		if caCert, err := os.ReadFile(cfg.CABundle); err == nil {
			pool := x509.NewCertPool()
			pool.AppendCertsFromPEM(caCert)
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
