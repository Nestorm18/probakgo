package csp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithNonceStoresUniqueRequestNonce(t *testing.T) {
	base := httptest.NewRequest(http.MethodGet, "/", nil)
	first, firstNonce, err := WithNonce(base)
	if err != nil {
		t.Fatalf("WithNonce: %v", err)
	}
	second, secondNonce, err := WithNonce(base)
	if err != nil {
		t.Fatalf("WithNonce second request: %v", err)
	}
	if firstNonce == "" || Nonce(first) != firstNonce {
		t.Fatal("nonce was not stored in request context")
	}
	if secondNonce == firstNonce || Nonce(second) != secondNonce {
		t.Fatal("request nonces are not unique")
	}
	if Nonce(base) != "" {
		t.Fatal("base request unexpectedly contains a nonce")
	}
}
