package config

import "testing"

func TestValidateRejectsPublicExampleSessionKey(t *testing.T) {
	cfg := &Config{APIPort: "36748", Timezone: "Europe/Madrid", SessionKey: exampleSessionKey}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted the public example session key")
	}
}

func TestValidateAcceptsTrustedProxyCIDRs(t *testing.T) {
	cfg := &Config{
		APIPort:        "36748",
		Timezone:       "Europe/Madrid",
		SessionKey:     "test-session-key-32-bytes-long!!",
		TrustedProxies: []string{"127.0.0.1/32", "::1/128"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
