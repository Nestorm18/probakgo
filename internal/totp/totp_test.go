package totp

import (
	"testing"
	"time"
)

func TestValidateRFC6238SHA1VectorLastSixDigits(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	if !Validate("287082", secret, time.Unix(59, 0)) {
		t.Fatal("expected RFC 6238 test vector to validate")
	}
}

func TestGenerateSecret(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}
	if secret == "" {
		t.Fatal("secret is empty")
	}
}
