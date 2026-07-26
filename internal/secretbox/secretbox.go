package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

const prefix = "enc:v1:"

type Box struct {
	aead      cipher.AEAD
	lookupKey [sha256.Size]byte
}

func New(masterKey string) (*Box, error) {
	if len(masterKey) < 32 {
		return nil, fmt.Errorf("data encryption key must contain at least 32 bytes")
	}
	encryptionKey := sha256.Sum256([]byte("probakgo:data-encryption:v1:" + masterKey))
	lookupKey := sha256.Sum256([]byte("probakgo:api-key-lookup:v1:" + masterKey))
	block, err := aes.NewCipher(encryptionKey[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead, lookupKey: lookupKey}, nil
}

func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, prefix)
}

func (b *Box) Encrypt(value string) (string, error) {
	if value == "" || IsEncrypted(value) {
		return value, nil
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate encryption nonce: %w", err)
	}
	sealed := b.aead.Seal(nil, nonce, []byte(value), []byte(prefix))
	payload := append(nonce, sealed...)
	return prefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

func (b *Box) Decrypt(value string) (string, error) {
	if value == "" || !IsEncrypted(value) {
		return value, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return "", fmt.Errorf("decode encrypted value: %w", err)
	}
	nonceSize := b.aead.NonceSize()
	if len(payload) <= nonceSize {
		return "", fmt.Errorf("encrypted value is truncated")
	}
	plain, err := b.aead.Open(nil, payload[:nonceSize], payload[nonceSize:], []byte(prefix))
	if err != nil {
		return "", fmt.Errorf("decrypt value: %w", err)
	}
	return string(plain), nil
}

func (b *Box) LookupHash(value string) string {
	mac := hmac.New(sha256.New, b.lookupKey[:])
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}
