package store

import (
	"context"
	"fmt"

	"probakgo/internal/domain"
)

func (s *Store) GetWindowsServerByAPIKey(ctx context.Context, keyID int64) (*domain.WindowsServer, error) {
	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM windows_servers WHERE api_key_id=? AND is_deleted=0`, keyID).Scan(&id); err != nil {
		return nil, err
	}
	return s.GetWindowsServer(ctx, id)
}

func (s *Store) ServerPublicIPForAPIKey(ctx context.Context, serverType string, keyID int64) (string, error) {
	var table string
	switch serverType {
	case "pve":
		table = "pve_servers"
	case "pbs":
		table = "pbs_servers"
	case "windows":
		table = "windows_servers"
	default:
		return "", fmt.Errorf("invalid server type")
	}
	var ip string
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(public_ip, '') FROM `+table+` WHERE api_key_id=? AND is_deleted=0`, keyID).Scan(&ip)
	return ip, err
}
