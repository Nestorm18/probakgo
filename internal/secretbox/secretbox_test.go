package secretbox

import "testing"

func TestEncryptDecryptAndLookupHash(t *testing.T) {
	box, err := New("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	encrypted, err := box.Encrypt("pbk-secret")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if encrypted == "pbk-secret" || !IsEncrypted(encrypted) {
		t.Fatalf("value was not encrypted: %q", encrypted)
	}
	plain, err := box.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if plain != "pbk-secret" {
		t.Fatalf("plain: got %q", plain)
	}
	if box.LookupHash("pbk-secret") == box.LookupHash("pbk-other") {
		t.Fatal("different values produced the same lookup hash")
	}
	if got, err := box.Encrypt(encrypted); err != nil || got != encrypted {
		t.Fatalf("encrypting ciphertext: got %q, err %v", got, err)
	}
}

func TestDecryptRejectsWrongKey(t *testing.T) {
	first, _ := New("0123456789abcdef0123456789abcdef")
	second, _ := New("abcdef0123456789abcdef0123456789")
	encrypted, _ := first.Encrypt("secret")
	if _, err := second.Decrypt(encrypted); err == nil {
		t.Fatal("Decrypt succeeded with the wrong key")
	}
}

func TestNewRejectsShortKey(t *testing.T) {
	if _, err := New("short"); err == nil {
		t.Fatal("New accepted a short key")
	}
}
