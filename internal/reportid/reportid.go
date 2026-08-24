package reportid

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// New returns a random identifier suitable for idempotent report submission.
func New() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate report id: %w", err)
	}
	return hex.EncodeToString(raw), nil
}
