//go:build !windows

package config

import (
	"errors"
	"os"
	"syscall"
)

// preserveOwner keeps the owner of an existing file when root replaces it, so
// the dedicated service account can still read its .env.
func preserveOwner(existing, replacement string) error {
	info, err := os.Stat(existing)
	if errors.Is(err, os.ErrNotExist) || os.Geteuid() != 0 {
		return nil
	}
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	return os.Chown(replacement, int(stat.Uid), int(stat.Gid))
}
