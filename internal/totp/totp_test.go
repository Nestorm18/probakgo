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

func TestValidateStepReportsTheMatchedStep(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	// 287082 belongs to step 1 (t=30..59); t=89 is step 2, within the drift.
	step, ok := ValidateStep("287082", secret, time.Unix(89, 0))
	if !ok || step != 1 {
		t.Fatalf("got step %d ok=%v, want step 1", step, ok)
	}
	if _, ok := ValidateStep("287082", secret, time.Unix(150, 0)); ok {
		t.Fatal("code accepted outside the drift window")
	}
}
