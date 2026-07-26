package schedule

import (
	"crypto/sha256"
	"encoding/binary"
	"os"
	"strings"
)

// DailyMinute returns a stable minute in the range 0..59. Different hosts and
// purposes spread scheduled work across the hour without changing on restart.
func DailyMinute(seed string) int {
	sum := sha256.Sum256([]byte(seed))
	return int(binary.BigEndian.Uint16(sum[:2]) % 60)
}

func HostSeed() string {
	for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if data, err := os.ReadFile(path); err == nil {
			if id := strings.TrimSpace(string(data)); id != "" {
				return id
			}
		}
	}
	hostname, _ := os.Hostname()
	return hostname
}
