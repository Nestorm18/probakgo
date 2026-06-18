package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"probakgo/internal/domain"
)

type psDisk struct {
	Name       string `json:"Name"`
	Label      string `json:"Label"`
	FileSystem string `json:"FileSystem"`
	DriveType  string `json:"DriveType"`
	Total      int64  `json:"Total"`
	Used       int64  `json:"Used"`
	Free       int64  `json:"Free"`
	Health     string `json:"Health"`
}

func buildReportRequest(ctx context.Context, cfg Config) (domain.WindowsReportRequest, error) {
	hostname, _ := os.Hostname()
	mid, err := machineID(ctx)
	if err != nil {
		return domain.WindowsReportRequest{}, err
	}
	disks, err := collectDisks(ctx)
	if err != nil {
		return domain.WindowsReportRequest{}, err
	}
	return domain.WindowsReportRequest{
		Hostname:      hostname,
		IPAddress:     localIP(),
		PublicIP:      publicIP(ctx),
		ClientVersion: version,
		MachineID:     mid,
		Disks:         disks,
	}, nil
}

func buildHeartbeatRequest(ctx context.Context, cfg Config) (domain.HeartbeatRequest, error) {
	hostname, _ := os.Hostname()
	mid, err := machineID(ctx)
	if err != nil {
		return domain.HeartbeatRequest{}, err
	}
	return domain.HeartbeatRequest{
		ServerType:    "windows",
		Hostname:      hostname,
		IPAddress:     localIP(),
		PublicIP:      publicIP(ctx),
		ClientVersion: version,
		MachineID:     mid,
	}, nil
}

func collectDisks(ctx context.Context) ([]domain.WindowsDiskPayload, error) {
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("disk collection is only available on windows")
	}
	ps := `
$logical = Get-CimInstance Win32_LogicalDisk -Filter "DriveType=3" | ForEach-Object {
  [PSCustomObject]@{
    Name=$_.DeviceID; Label=$_.VolumeName; FileSystem=$_.FileSystem; DriveType="Fixed";
    Total=[int64]$_.Size; Used=[int64]($_.Size-$_.FreeSpace); Free=[int64]$_.FreeSpace; Health=""
  }
}
@($logical | Sort-Object Name) | ConvertTo-Json -Compress
`
	out, err := runPowerShell(ctx, ps)
	if err != nil {
		return nil, err
	}
	return parseDisksJSON(out)
}

func parseDisksJSON(data []byte) ([]domain.WindowsDiskPayload, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil, nil
	}
	var rows []psDisk
	if data[0] == '[' {
		if err := json.Unmarshal(data, &rows); err != nil {
			return nil, err
		}
	} else {
		var row psDisk
		if err := json.Unmarshal(data, &row); err != nil {
			return nil, err
		}
		rows = []psDisk{row}
	}
	out := make([]domain.WindowsDiskPayload, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Name) == "" {
			continue
		}
		if !isWindowsLogicalVolume(row.Name, row.DriveType) {
			continue
		}
		out = append(out, domain.WindowsDiskPayload{
			Name:       row.Name,
			Label:      row.Label,
			FileSystem: row.FileSystem,
			DriveType:  row.DriveType,
			Total:      row.Total,
			Used:       row.Used,
			Free:       row.Free,
			Health:     row.Health,
		})
	}
	return out, nil
}

func isWindowsLogicalVolume(name, driveType string) bool {
	name = strings.TrimSpace(name)
	driveType = strings.TrimSpace(strings.ToLower(driveType))
	if driveType != "" && driveType != "fixed" {
		return false
	}
	return len(name) == 2 && name[1] == ':' && ((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z'))
}

func machineID(ctx context.Context) (string, error) {
	if runtime.GOOS != "windows" {
		hostname, _ := os.Hostname()
		return hostname, nil
	}
	out, err := runPowerShell(ctx, `(Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Cryptography').MachineGuid`)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		return "", fmt.Errorf("empty MachineGuid")
	}
	return id, nil
}

func runPowerShell(ctx context.Context, script string) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("powershell failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func localIP() string {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", time.Second)
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			return addr.IP.String()
		}
	}
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ip, _, _ := net.ParseCIDR(addr.String())
			if ip == nil || ip.To4() == nil {
				continue
			}
			return ip.String()
		}
	}
	return ""
}

func publicIP(ctx context.Context) string {
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := httpRequest(cctx, "GET", "https://api.ipify.org", nil)
	if err != nil {
		return ""
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	var b bytes.Buffer
	_, _ = b.ReadFrom(resp.Body)
	return strings.TrimSpace(b.String())
}
