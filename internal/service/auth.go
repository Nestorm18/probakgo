package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"probakgo/internal/domain"
	"probakgo/internal/store"
)

var (
	ErrInvalidKey = errors.New("invalid or inactive API key")
	ErrMachineID  = errors.New("machine ID mismatch")
	ErrKeyType    = errors.New("key type not allowed for this endpoint")
)

type AuthService struct {
	store *store.Store
}

func NewAuth(st *store.Store) *AuthService {
	return &AuthService{store: st}
}

// ValidateServerKey validates a pbk- key and handles machine ID binding.
// Returns the API key record if valid.
func (a *AuthService) ValidateServerKey(rawKey, machineID string) (*domain.APIKey, error) {
	ctx := context.Background()
	k, err := a.store.GetAPIKeyByValue(ctx, rawKey)
	if err != nil || !k.IsActive {
		return nil, ErrInvalidKey
	}
	if k.KeyType != "server" {
		return nil, ErrKeyType
	}
	if machineID == "" {
		return nil, fmt.Errorf("%w: missing X-Machine-ID", ErrMachineID)
	}
	if k.MachineID == "" {
		if err := a.store.BindAPIKeyMachineID(ctx, k.ID, machineID); err != nil {
			slog.Error("bind machine id", "err", err)
			return nil, fmt.Errorf("bind machine id: %w", err)
		}
		k.MachineID = machineID
	} else if k.MachineID != machineID {
		return nil, fmt.Errorf("%w: key bound to different machine", ErrMachineID)
	}
	_ = a.store.UpdateAPIKeyLastUsed(ctx, k.ID)
	return k, nil
}

// ExtractBearer extracts the token from "Bearer <token>" or returns it as-is.
func ExtractBearer(header string) string {
	if after, ok := strings.CutPrefix(header, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return strings.TrimSpace(header)
}
