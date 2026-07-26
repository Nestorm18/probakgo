package csp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
)

type nonceContextKey struct{}

func WithNonce(r *http.Request) (*http.Request, string, error) {
	raw := make([]byte, 18)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", fmt.Errorf("generate CSP nonce: %w", err)
	}
	nonce := base64.RawStdEncoding.EncodeToString(raw)
	return r.WithContext(context.WithValue(r.Context(), nonceContextKey{}, nonce)), nonce, nil
}

func Nonce(r *http.Request) string {
	nonce, _ := r.Context().Value(nonceContextKey{}).(string)
	return nonce
}
