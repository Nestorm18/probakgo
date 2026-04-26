package service

import (
	"errors"
	"testing"
)

func TestExtractBearer_WithPrefix(t *testing.T) {
	got := ExtractBearer("Bearer mytoken123")
	if got != "mytoken123" {
		t.Errorf("want mytoken123, got %q", got)
	}
}

func TestExtractBearer_WithoutPrefix(t *testing.T) {
	got := ExtractBearer("mytoken123")
	if got != "mytoken123" {
		t.Errorf("want mytoken123, got %q", got)
	}
}

func TestValidateServerKey_HappyPath(t *testing.T) {
	_, st := openTestStore(t)
	auth := NewAuth(st)

	k, _ := st.CreateAPIKey("client", "server", "")
	result, err := auth.ValidateServerKey(k.Key, "")
	if err != nil {
		t.Fatalf("ValidateServerKey: %v", err)
	}
	if result.Key != k.Key {
		t.Errorf("want key %q, got %q", k.Key, result.Key)
	}
}

func TestValidateServerKey_WrongType(t *testing.T) {
	_, st := openTestStore(t)
	auth := NewAuth(st)

	k, _ := st.CreateAPIKey("admin", "admin", "")
	_, err := auth.ValidateServerKey(k.Key, "")
	if !errors.Is(err, ErrKeyType) {
		t.Errorf("want ErrKeyType, got %v", err)
	}
}

func TestValidateServerKey_MachineBinding_First(t *testing.T) {
	_, st := openTestStore(t)
	auth := NewAuth(st)

	k, _ := st.CreateAPIKey("client", "server", "")

	// First call with a machine ID binds it
	_, err := auth.ValidateServerKey(k.Key, "machine-abc")
	if err != nil {
		t.Fatalf("first bind: %v", err)
	}

	// Same machine is accepted on subsequent calls
	_, err = auth.ValidateServerKey(k.Key, "machine-abc")
	if err != nil {
		t.Fatalf("same machine second call: %v", err)
	}

	// MachineID was persisted
	updated, _ := st.GetAPIKeyByValue(k.Key)
	if updated.MachineID != "machine-abc" {
		t.Errorf("MachineID: want machine-abc, got %q", updated.MachineID)
	}
}

func TestValidateServerKey_MachineBinding_Mismatch(t *testing.T) {
	_, st := openTestStore(t)
	auth := NewAuth(st)

	k, _ := st.CreateAPIKey("client", "server", "")
	_, _ = auth.ValidateServerKey(k.Key, "machine-aaa")

	_, err := auth.ValidateServerKey(k.Key, "machine-bbb")
	if !errors.Is(err, ErrMachineID) {
		t.Errorf("want ErrMachineID, got %v", err)
	}
}

func TestValidateAdminKey_HappyPath(t *testing.T) {
	_, st := openTestStore(t)
	auth := NewAuth(st)

	k, _ := st.CreateAPIKey("myadmin", "admin", "")
	result, err := auth.ValidateAdminKey(k.Key)
	if err != nil {
		t.Fatalf("ValidateAdminKey: %v", err)
	}
	if result.KeyType != "admin" {
		t.Errorf("KeyType: want admin, got %q", result.KeyType)
	}
}

func TestValidateAdminKey_WrongType(t *testing.T) {
	_, st := openTestStore(t)
	auth := NewAuth(st)

	k, _ := st.CreateAPIKey("client", "server", "")
	_, err := auth.ValidateAdminKey(k.Key)
	if !errors.Is(err, ErrKeyType) {
		t.Errorf("want ErrKeyType, got %v", err)
	}
}
