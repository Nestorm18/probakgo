package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	issuer = "Probakgo"
	digits = 6
	period = 30
)

var base32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

func GenerateSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32NoPad.EncodeToString(b), nil
}

func ProvisioningURI(username, secret string) string {
	label := issuer + ":" + username
	q := url.Values{}
	q.Set("secret", strings.ToUpper(strings.TrimSpace(secret)))
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", strconv.Itoa(digits))
	q.Set("period", strconv.Itoa(period))
	return "otpauth://totp/" + url.PathEscape(label) + "?" + q.Encode()
}

func Validate(code, secret string, now time.Time) bool {
	_, ok := ValidateStep(code, secret, now)
	return ok
}

// ValidateStep reports the time step the code belongs to, so callers can
// refuse a code whose step was already used (replay protection).
func ValidateStep(code, secret string, now time.Time) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != digits {
		return 0, false
	}
	key, err := base32NoPad.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return 0, false
	}
	step := now.Unix() / period
	for drift := int64(-1); drift <= 1; drift++ {
		if subtle.ConstantTimeCompare([]byte(generateCode(key, step+drift)), []byte(code)) == 1 {
			return step + drift, true
		}
	}
	return 0, false
}

func generateCode(key []byte, counter int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(counter))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	bin := (uint32(sum[offset])&0x7f)<<24 |
		(uint32(sum[offset+1])&0xff)<<16 |
		(uint32(sum[offset+2])&0xff)<<8 |
		(uint32(sum[offset+3]) & 0xff)
	otp := bin % 1000000
	return fmt.Sprintf("%06d", otp)
}
