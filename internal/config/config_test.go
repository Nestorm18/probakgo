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

func TestValidateRejectsShortDataEncryptionKey(t *testing.T) {
	cfg := &Config{
		APIPort:    "36748",
		Timezone:   "Europe/Madrid",
		SessionKey: "test-session-key-32-bytes-long!!",
		DataKey:    "short",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted a short DATA_ENCRYPTION_KEY")
	}
}

func TestValidateChecksLoginTrustedCIDRs(t *testing.T) {
	cfg := &Config{
		APIPort:           "36748",
		Timezone:          "Europe/Madrid",
		SessionKey:        "test-session-key-32-bytes-long!!",
		LoginTrustedCIDRs: []string{"192.0.2.0/24", "2001:db8::/48"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	cfg.LoginTrustedCIDRs = []string{"192.0.2.10"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted an address without a prefix length")
	}
}
