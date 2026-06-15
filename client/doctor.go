package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type doctor struct {
	ok       int
	warnings int
	failures int
}

func runDoctor() {
	d := &doctor{}
	fmt.Printf("probakgo-client doctor v%s\n\n", version)

	cfg := loadConfig()
	si := newSysInfo(cfg)
	if cfg.ServerType == "" || cfg.ServerType == "unknown" {
		cfg.ServerType = si.detectServerType()
	}

	ok, msg := doctorConfigFile()
	d.check("configuration file", ok, msg)
	d.check("API_URL configured", cfg.APIURL != "", "API_URL is empty")
	d.check("API_KEY configured", cfg.APIKey != "", "API_KEY is empty")
	d.check("API_KEY format", strings.HasPrefix(cfg.APIKey, "pbk-"), "API key must start with pbk-")
	d.check("machine-id", si.machineID() != "", "could not read /etc/machine-id")
	d.check("server type", cfg.ServerType == "pve" || cfg.ServerType == "pbs", "could not detect PVE/PBS; set SERVER_TYPE")

	if cfg.APIURL != "" {
		ok, msg := doctorAPIHealth(cfg.APIURL)
		d.check("Probakgo API health", ok, msg)
	}

	if cfg.ProxmoxToken == "" || cfg.ProxmoxSecret == "" {
		d.fail("Proxmox credentials", "PROXMOX_TOKEN or PROXMOX_SECRET is empty")
	} else {
		switch cfg.ServerType {
		case "pve":
			d.check("Proxmox VE API", newPVEClient(cfg, si).validateConnection(), "PVE connection failed")
			d.warnCheck("vzdump hook", doctorFileContains("/etc/vzdump.conf", hookPath), "hook not registered in /etc/vzdump.conf")
		case "pbs":
			d.check("Proxmox Backup Server API", newPBSClient(cfg, si).validateConnection(), "PBS connection failed")
		}
	}

	if cfg.APIURL != "" && strings.HasPrefix(cfg.APIKey, "pbk-") && (cfg.ServerType == "pve" || cfg.ServerType == "pbs") {
		d.check("heartbeat send", sendHeartbeat(cfg, si) == nil, "heartbeat failed")
	}

	d.warnCheck("installed binary", doctorPathExists(binaryPath), binaryPath+" not found")
	d.warnCheck("PATH link", doctorPathExists(binaryLinkPath), binaryLinkPath+" not found")
	d.warnCheck("heartbeat service unit", doctorPathExists(heartbeatServicePath), heartbeatServicePath+" not found")
	d.warnCheck("heartbeat timer unit", doctorPathExists(heartbeatTimerPath), heartbeatTimerPath+" not found")
	if doctorHasSystemctl() {
		d.warnCheck("heartbeat timer enabled", doctorSystemctlOK("is-enabled", "probakgo-client-heartbeat.timer"), "timer is not enabled")
		d.warnCheck("heartbeat timer active", doctorSystemctlOK("is-active", "probakgo-client-heartbeat.timer"), "timer is not active")
		d.warnCheck("heartbeat timer scheduled", doctorTimerHasNextRun(), "timer has no next run scheduled")
	}

	fmt.Printf("\nSummary: %d OK, %d warning, %d failed\n", d.ok, d.warnings, d.failures)
	if d.failures > 0 {
		os.Exit(1)
	}
}

func (d *doctor) check(name string, ok bool, failMsg ...string) {
	if ok {
		d.pass(name)
		return
	}
	msg := "failed"
	if len(failMsg) > 0 && failMsg[0] != "" {
		msg = failMsg[0]
	}
	d.fail(name, msg)
}

func (d *doctor) warnCheck(name string, ok bool, warnMsg string) {
	if ok {
		d.pass(name)
		return
	}
	d.warn(name, warnMsg)
}

func (d *doctor) pass(name string) {
	d.ok++
	fmt.Printf("[OK]   %s\n", name)
}

func (d *doctor) warn(name, msg string) {
	d.warnings++
	fmt.Printf("[WARN] %s: %s\n", name, msg)
}

func (d *doctor) fail(name, msg string) {
	d.failures++
	fmt.Printf("[FAIL] %s: %s\n", name, msg)
}

func doctorConfigFile() (bool, string) {
	for _, p := range []string{"/opt/probakgo/.env", ".env"} {
		if _, err := os.Stat(p); err == nil {
			return true, ""
		}
	}
	return false, "no .env found in /opt/probakgo/.env or current directory"
}

func doctorAPIHealth(apiURL string) (bool, string) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(strings.TrimRight(apiURL, "/") + "/api/health")
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return true, ""
}

func doctorPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func doctorFileContains(path, needle string) bool {
	data, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(data), needle)
}

func doctorHasSystemctl() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func doctorSystemctlOK(args ...string) bool {
	cmd := exec.Command("systemctl", args...)
	return cmd.Run() == nil
}

func doctorTimerHasNextRun() bool {
	out, err := exec.Command("systemctl", "list-timers", "--all", "--no-pager", "probakgo-client-heartbeat.timer").CombinedOutput()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "probakgo-client-heartbeat.timer") {
			return !strings.HasPrefix(line, "-")
		}
	}
	return false
}
