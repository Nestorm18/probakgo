package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type discoveredVM struct {
	VMID string
	Name string
}

type backupDays struct {
	Monday    bool
	Tuesday   bool
	Wednesday bool
	Thursday  bool
	Friday    bool
	Saturday  bool
	Sunday    bool
}

type pveBackupSchedule struct {
	Days      backupDays
	StartMin  int
	Weekdays  bool
	HasAnyDay bool
}

type backupConfigResponse struct {
	Configs []backupConfig `json:"configs"`
}

type backupConfig struct {
	VMID      string `json:"vm_id"`
	VMName    string `json:"vm_name"`
	Monday    bool   `json:"monday"`
	Tuesday   bool   `json:"tuesday"`
	Wednesday bool   `json:"wednesday"`
	Thursday  bool   `json:"thursday"`
	Friday    bool   `json:"friday"`
	Saturday  bool   `json:"saturday"`
	Sunday    bool   `json:"sunday"`
}

func (c *pveClient) discoverPVEVMs() ([]discoveredVM, error) {
	var vms []discoveredVM
	seen := make(map[string]bool)
	successes := 0
	for _, ep := range []string{
		fmt.Sprintf("nodes/%s/qemu", c.si.Hostname),
		fmt.Sprintf("nodes/%s/lxc", c.si.Hostname),
	} {
		data, err := c.get(ep)
		if err != nil {
			continue
		}
		successes++
		raw, _ := data["data"].([]any)
		for _, item := range raw {
			appendDiscoveredVM(&vms, seen, item, "")
		}
	}

	// PVE 6 installations can return an empty per-node inventory even though
	// the cluster resource endpoint exposes the guests. It also handles a
	// partially unavailable QEMU/LXC endpoint without losing the other type.
	if len(vms) == 0 || successes < 2 {
		data, err := c.get("cluster/resources?type=vm")
		if err == nil {
			raw, _ := data["data"].([]any)
			for _, item := range raw {
				appendDiscoveredVM(&vms, seen, item, c.si.Hostname)
			}
		} else if successes == 0 {
			return nil, fmt.Errorf("could not query PVE guest inventory: %w", err)
		}
	}

	sort.Slice(vms, func(i, j int) bool {
		a, _ := strconv.Atoi(vms[i].VMID)
		b, _ := strconv.Atoi(vms[j].VMID)
		return a < b
	})
	return vms, nil
}

func appendDiscoveredVM(vms *[]discoveredVM, seen map[string]bool, item any, node string) {
	vm, ok := item.(map[string]any)
	if !ok {
		return
	}
	if itemNode := strings.TrimSpace(pveValueString(vm["node"])); node != "" && itemNode != "" && itemNode != node {
		return
	}
	if guestType := strings.ToLower(strings.TrimSpace(pveValueString(vm["type"]))); guestType != "" && guestType != "qemu" && guestType != "lxc" {
		return
	}
	vmid := pveValueString(vm["vmid"])
	if vmid == "" || seen[vmid] {
		return
	}
	seen[vmid] = true
	*vms = append(*vms, discoveredVM{
		VMID: vmid,
		Name: pveValueString(vm["name"]),
	})
}

func (c *pveClient) discoverPVEBackupSchedules(vms []discoveredVM) (map[string]pveBackupSchedule, error) {
	data, err := c.get("cluster/backup")
	if err != nil {
		return nil, err
	}
	jobs, _ := data["data"].([]any)
	schedules := make(map[string]pveBackupSchedule)
	allVMIDs := make([]string, 0, len(vms))
	for _, vm := range vms {
		allVMIDs = append(allVMIDs, vm.VMID)
	}

	for _, item := range jobs {
		job, ok := item.(map[string]any)
		if !ok || !backupJobEnabled(job["enabled"]) {
			continue
		}
		scheduleText := pveBackupScheduleText(job)
		schedule, ok := parsePVEBackupSchedule(scheduleText)
		if !ok {
			continue
		}
		for _, vmid := range backupJobVMIDs(job, allVMIDs) {
			current, exists := schedules[vmid]
			if !exists || betterPrimarySchedule(schedule, current) {
				schedule.Days = mergeWeekendDays(schedule.Days, current.Days)
				schedules[vmid] = schedule
				continue
			}
			if schedule.Days.Saturday || schedule.Days.Sunday {
				current.Days = mergeWeekendDays(current.Days, schedule.Days)
				schedules[vmid] = current
			}
		}
	}
	return schedules, nil
}

func pveBackupScheduleText(job map[string]any) string {
	if schedule := strings.TrimSpace(pveValueString(job["schedule"])); schedule != "" {
		return schedule
	}
	days := strings.TrimSpace(pveValueString(job["dow"]))
	start := strings.TrimSpace(pveValueString(job["starttime"]))
	return strings.TrimSpace(days + " " + start)
}

func pveValueString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	default:
		return ""
	}
}

func syncBackupConfig(cfg *Config, serverName, machineID string, vms []discoveredVM, schedules ...map[string]pveBackupSchedule) (created, updated, skipped int, err error) {
	return syncBackupConfigWithOverwrite(cfg, serverName, machineID, vms, false, schedules...)
}

func syncBackupConfigWithOverwrite(cfg *Config, serverName, machineID string, vms []discoveredVM, overwriteExisting bool, schedules ...map[string]pveBackupSchedule) (created, updated, skipped int, err error) {
	if len(vms) == 0 {
		return 0, 0, 0, nil
	}
	apiURL := strings.TrimRight(cfg.APIURL, "/")
	if apiURL == "" || cfg.APIKey == "" {
		return 0, 0, 0, fmt.Errorf("API_URL and API_KEY are required")
	}

	base := apiURL + "/api/backup-config/pve/" + url.PathEscape(serverName)
	existing, err := fetchExistingVMConfigs(base, cfg.APIKey, machineID)
	if err != nil {
		return 0, 0, 0, err
	}

	scheduleByVMID := map[string]pveBackupSchedule{}
	if len(schedules) > 0 && schedules[0] != nil {
		scheduleByVMID = schedules[0]
	}

	for _, vm := range vms {
		days := scheduleByVMID[vm.VMID].Days
		payload := vmConfigPayload(vm, days)
		if existingCfg, ok := existing[vm.VMID]; ok {
			if overwriteExisting {
				if backupConfigMatches(existingCfg, vm, days) {
					skipped++
					continue
				}
				if err := updateVMConfig(base, cfg.APIKey, machineID, vm.VMID, payload); err != nil {
					return created, updated, skipped, err
				}
				updated++
				continue
			}
			if !hasAnyScheduledDay(existingCfg) {
				if err := updateVMConfig(base, cfg.APIKey, machineID, vm.VMID, payload); err != nil {
					return created, updated, skipped, err
				}
				updated++
				continue
			}
			skipped++
			continue
		}
		if err := createVMConfig(base, cfg.APIKey, machineID, vm.VMID, payload); err != nil {
			return created, updated, skipped, err
		}
		created++
	}
	return created, updated, skipped, nil
}

func syncInstalledBackupConfig(overwriteExisting bool) error {
	return syncInstalledBackupConfigForMode(overwriteExisting, true)
}

func autoSyncInstalledBackupConfig() error {
	return syncInstalledBackupConfigForMode(false, false)
}

func syncInstalledBackupConfigForMode(overwriteExisting, requirePVE bool) error {
	cfg := loadConfig()
	si := newSysInfo(cfg)
	if cfg.ServerType == "" || cfg.ServerType == "unknown" {
		cfg.ServerType = si.detectServerType()
	}
	if cfg.ServerType != "pve" {
		if !requirePVE {
			return nil
		}
		return fmt.Errorf("sync-backups is only available on Proxmox VE")
	}
	if cfg.APIURL == "" || cfg.APIKey == "" {
		return fmt.Errorf("API_URL and API_KEY are required")
	}
	if cfg.ProxmoxToken == "" || cfg.ProxmoxSecret == "" {
		return fmt.Errorf("PROXMOX_TOKEN and PROXMOX_SECRET are required")
	}

	pve := newPVEClient(cfg, si)
	vms, err := pve.discoverPVEVMs()
	if err != nil {
		return fmt.Errorf("list PVE VMs: %w", err)
	}
	schedules, scheduleErr := pve.discoverPVEBackupSchedules(vms)
	if scheduleErr != nil {
		if overwriteExisting {
			return fmt.Errorf("read PVE backup jobs: %w", scheduleErr)
		}
		fmt.Fprintf(os.Stderr, "WARN: could not read PVE backup jobs, using Monday-Friday defaults: %v\n", scheduleErr)
	}
	created, updated, skipped, err := syncBackupConfigWithOverwrite(
		cfg, si.Hostname, si.machineID(), vms, overwriteExisting, schedules,
	)
	if err != nil {
		return fmt.Errorf("sync backup config: %w", err)
	}
	fmt.Printf("Backup config sync: %d created, %d updated, %d unchanged\n", created, updated, skipped)
	return nil
}

func fetchExistingVMConfigs(baseURL, apiKey, machineID string) (map[string]backupConfig, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if machineID != "" {
		req.Header.Set("X-Machine-ID", machineID)
	}

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("get backup config: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get backup config returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed backupConfigResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse backup config: %w", err)
	}
	existing := make(map[string]backupConfig, len(parsed.Configs))
	for _, cfg := range parsed.Configs {
		existing[cfg.VMID] = cfg
	}
	return existing, nil
}

func createVMConfig(baseURL, apiKey, machineID, vmid string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/vms", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	if machineID != "" {
		req.Header.Set("X-Machine-ID", machineID)
	}

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("create backup config for VM %s: %w", vmid, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("create backup config for VM %s returned %d: %s", vmid, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func updateVMConfig(baseURL, apiKey, machineID, vmid string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, baseURL+"/vms/"+url.PathEscape(vmid), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	if machineID != "" {
		req.Header.Set("X-Machine-ID", machineID)
	}

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("update backup config for VM %s: %w", vmid, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update backup config for VM %s returned %d: %s", vmid, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func vmConfigPayload(vm discoveredVM, days backupDays) map[string]any {
	if !days.any() {
		days = backupDays{Monday: true, Tuesday: true, Wednesday: true, Thursday: true, Friday: true}
	}
	return map[string]any{
		"vm_id":     vm.VMID,
		"vm_name":   vm.Name,
		"monday":    days.Monday,
		"tuesday":   days.Tuesday,
		"wednesday": days.Wednesday,
		"thursday":  days.Thursday,
		"friday":    days.Friday,
		"saturday":  days.Saturday,
		"sunday":    days.Sunday,
	}
}

func hasAnyScheduledDay(cfg backupConfig) bool {
	return cfg.Monday || cfg.Tuesday || cfg.Wednesday || cfg.Thursday || cfg.Friday || cfg.Saturday || cfg.Sunday
}

func backupConfigMatches(cfg backupConfig, vm discoveredVM, days backupDays) bool {
	if !days.any() {
		days = backupDays{Monday: true, Tuesday: true, Wednesday: true, Thursday: true, Friday: true}
	}
	return cfg.VMID == vm.VMID &&
		cfg.VMName == vm.Name &&
		cfg.Monday == days.Monday &&
		cfg.Tuesday == days.Tuesday &&
		cfg.Wednesday == days.Wednesday &&
		cfg.Thursday == days.Thursday &&
		cfg.Friday == days.Friday &&
		cfg.Saturday == days.Saturday &&
		cfg.Sunday == days.Sunday
}

func backupJobEnabled(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != "" && x != "0" && strings.ToLower(x) != "false"
	default:
		return true
	}
}

func backupJobVMIDs(job map[string]any, allVMIDs []string) []string {
	excluded := parseVMIDList(fmt.Sprint(job["exclude"]))
	if isAllBackupJob(job["all"]) {
		var out []string
		for _, vmid := range allVMIDs {
			if !excluded[vmid] {
				out = append(out, vmid)
			}
		}
		return out
	}
	vmids := parseVMIDList(fmt.Sprint(job["vmid"]))
	out := make([]string, 0, len(vmids))
	for vmid := range vmids {
		if !excluded[vmid] {
			out = append(out, vmid)
		}
	}
	sort.Strings(out)
	return out
}

func isAllBackupJob(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		x = strings.ToLower(strings.TrimSpace(x))
		return x == "1" || x == "true" || x == "all"
	default:
		return false
	}
}

func parseVMIDList(s string) map[string]bool {
	out := make(map[string]bool)
	s = strings.TrimSpace(s)
	if s == "" || s == "<nil>" {
		return out
	}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out[part] = true
	}
	return out
}

func parsePVEBackupSchedule(schedule string) (pveBackupSchedule, bool) {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(schedule)))
	if len(fields) == 0 {
		return pveBackupSchedule{}, false
	}
	daysToken := fields[0]
	timeToken := ""
	if strings.Contains(daysToken, ":") {
		timeToken = daysToken
		daysToken = "daily"
	} else if len(fields) > 1 {
		timeToken = fields[1]
	}
	days, ok := parsePVEDays(daysToken)
	if !ok {
		return pveBackupSchedule{}, false
	}
	startMin := parseScheduleStartMinute(timeToken)
	return pveBackupSchedule{
		Days:      days,
		StartMin:  startMin,
		Weekdays:  days.weekdays(),
		HasAnyDay: days.any(),
	}, true
}

func parsePVEDays(token string) (backupDays, bool) {
	token = strings.TrimSpace(strings.ToLower(token))
	if token == "" || token == "daily" || token == "*" {
		return backupDays{Monday: true, Tuesday: true, Wednesday: true, Thursday: true, Friday: true, Saturday: true, Sunday: true}, true
	}
	if token == "weekdays" {
		return backupDays{Monday: true, Tuesday: true, Wednesday: true, Thursday: true, Friday: true}, true
	}
	if token == "weekend" {
		return backupDays{Saturday: true, Sunday: true}, true
	}
	var days backupDays
	for _, part := range strings.Split(token, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		rangeSeparator := ""
		if strings.Contains(part, "..") {
			rangeSeparator = ".."
		} else if strings.Contains(part, "-") {
			rangeSeparator = "-"
		}
		if rangeSeparator != "" {
			bounds := strings.SplitN(part, rangeSeparator, 2)
			start, ok1 := pveDayIndex(bounds[0])
			end, ok2 := pveDayIndex(bounds[1])
			if !ok1 || !ok2 {
				return backupDays{}, false
			}
			for i := start; ; i = (i + 1) % 7 {
				days = days.withDay(i)
				if i == end {
					break
				}
			}
			continue
		}
		idx, ok := pveDayIndex(part)
		if !ok {
			return backupDays{}, false
		}
		days = days.withDay(idx)
	}
	return days, days.any()
}

func pveDayIndex(day string) (int, bool) {
	switch strings.TrimSpace(day) {
	case "mon", "monday":
		return 0, true
	case "tue", "tuesday":
		return 1, true
	case "wed", "wednesday":
		return 2, true
	case "thu", "thursday":
		return 3, true
	case "fri", "friday":
		return 4, true
	case "sat", "saturday":
		return 5, true
	case "sun", "sunday":
		return 6, true
	default:
		return 0, false
	}
}

func parseScheduleStartMinute(token string) int {
	parts := strings.Split(strings.TrimSpace(token), ":")
	if len(parts) < 2 {
		return -1
	}
	hour, errH := strconv.Atoi(parts[0])
	minute, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return -1
	}
	return hour*60 + minute
}

func betterPrimarySchedule(candidate, current pveBackupSchedule) bool {
	if candidate.Weekdays != current.Weekdays {
		return candidate.Weekdays
	}
	if candidate.StartMin != current.StartMin {
		return candidate.StartMin > current.StartMin
	}
	return candidate.Days.dayCount() > current.Days.dayCount()
}

func mergeWeekendDays(base, extra backupDays) backupDays {
	if extra.Saturday {
		base.Saturday = true
	}
	if extra.Sunday {
		base.Sunday = true
	}
	return base
}

func (d backupDays) any() bool {
	return d.Monday || d.Tuesday || d.Wednesday || d.Thursday || d.Friday || d.Saturday || d.Sunday
}

func (d backupDays) weekdays() bool {
	return d.Monday && d.Tuesday && d.Wednesday && d.Thursday && d.Friday
}

func (d backupDays) dayCount() int {
	count := 0
	for _, ok := range []bool{d.Monday, d.Tuesday, d.Wednesday, d.Thursday, d.Friday, d.Saturday, d.Sunday} {
		if ok {
			count++
		}
	}
	return count
}

func (d backupDays) withDay(idx int) backupDays {
	switch idx {
	case 0:
		d.Monday = true
	case 1:
		d.Tuesday = true
	case 2:
		d.Wednesday = true
	case 3:
		d.Thursday = true
	case 4:
		d.Friday = true
	case 5:
		d.Saturday = true
	case 6:
		d.Sunday = true
	}
	return d
}
