package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type SysInfo struct {
	Hostname string
	cfg      *Config
}

type SwapInfo struct {
	Total   int64 `json:"swap_total"`
	Used    int64 `json:"swap_used"`
	Enabled bool  `json:"swap_enabled"`
}

func newSysInfo(cfg *Config) *SysInfo {
	hostname := ""
	if data, err := os.ReadFile("/etc/hostname"); err == nil {
		hostname = strings.TrimSpace(string(data))
	}
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	return &SysInfo{Hostname: hostname, cfg: cfg}
}

func (si *SysInfo) localIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func (si *SysInfo) publicIP() string {
	client := &http.Client{Timeout: 5 * time.Second}
	for _, svc := range []string{"https://api.ipify.org", "https://ifconfig.me/ip", "https://icanhazip.com"} {
		resp, err := client.Get(svc)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return strings.TrimSpace(string(body))
		}
	}
	return ""
}

func (si *SysInfo) machineID() string {
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if data, err := os.ReadFile(p); err == nil {
			if id := strings.TrimSpace(string(data)); id != "" {
				return id
			}
		}
	}
	log.Println("WARN: could not read machine-id")
	return ""
}

func (si *SysInfo) swapInfo() SwapInfo {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return SwapInfo{}
	}
	return parseSwapInfo(string(data))
}

func parseSwapInfo(meminfo string) SwapInfo {
	var totalKB, freeKB int64
	for _, line := range strings.Split(meminfo, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "SwapTotal":
			totalKB = value
		case "SwapFree":
			freeKB = value
		}
	}
	if totalKB <= 0 {
		return SwapInfo{}
	}
	usedKB := totalKB - freeKB
	if usedKB < 0 {
		usedKB = 0
	}
	return SwapInfo{
		Total:   totalKB * 1024,
		Used:    usedKB * 1024,
		Enabled: true,
	}
}

func serverTypeFromContent(content string) string {
	c := strings.ToLower(content)
	switch {
	case strings.Contains(c, "proxmox backup server"):
		return "pbs"
	case strings.Contains(c, "proxmox virtual environment"):
		return "pve"
	default:
		return "unknown"
	}
}

func (si *SysInfo) detectServerType() string {
	data, err := os.ReadFile("/etc/issue")
	if err != nil {
		return "unknown"
	}
	return serverTypeFromContent(string(data))
}
