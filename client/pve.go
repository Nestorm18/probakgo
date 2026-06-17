package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type pveClient struct {
	cfg     *Config
	si      *SysInfo
	http    *http.Client
	baseURL string
}

func newPVEClient(cfg *Config, si *SysInfo) *pveClient {
	return &pveClient{
		cfg:     cfg,
		si:      si,
		http:    newHTTPClient(cfg),
		baseURL: fmt.Sprintf("https://%s:8006/api2/json", si.localIP()),
	}
}

func (c *pveClient) get(endpoint string) (map[string]any, error) {
	url := c.baseURL + "/" + endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.cfg.ProxmoxToken, c.cfg.ProxmoxSecret))

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("WARN: PVE %s attempt %d/3: %v", endpoint, attempt, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		switch resp.StatusCode {
		case 401:
			return nil, fmt.Errorf("PVE auth error (401): check PROXMOX_TOKEN and PROXMOX_SECRET")
		case 403:
			return nil, fmt.Errorf("PVE permission error (403): token lacks required privileges")
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("PVE API %s returned HTTP %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(body)))
		}
		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = fmt.Errorf("invalid JSON: %w", err)
			log.Printf("WARN: PVE %s attempt %d/3: %v", endpoint, attempt, lastErr)
			continue
		}
		return result, nil
	}
	return nil, fmt.Errorf("PVE API %s failed after 3 attempts: %w", endpoint, lastErr)
}

func (c *pveClient) validateConnection() bool {
	_, err := c.get("version")
	if err != nil {
		log.Printf("ERROR: %v", err)
	}
	return err == nil
}

type backupStatus struct {
	OK        bool  `json:"status"`
	StartTime int64 `json:"starttime"`
	EndTime   int64 `json:"endtime"`
	Duration  int64 `json:"duration"`
}

type backupFile struct {
	ctime int64
	size  int64
	volid string
}

func (c *pveClient) vmNames() map[int64]string {
	names := make(map[int64]string)
	for _, ep := range []string{
		fmt.Sprintf("nodes/%s/qemu", c.si.Hostname),
		fmt.Sprintf("nodes/%s/lxc", c.si.Hostname),
	} {
		data, err := c.get(ep)
		if err != nil {
			continue
		}
		vms, _ := data["data"].([]any)
		for _, v := range vms {
			vm, ok := v.(map[string]any)
			if !ok {
				continue
			}
			vmidF, _ := vm["vmid"].(float64)
			name, _ := vm["name"].(string)
			names[int64(vmidF)] = name
		}
	}
	return names
}

func (c *pveClient) backupJobTasks(names map[int64]string, filesByVMID map[int64][]backupFile) []map[string]any {
	data, err := c.get(fmt.Sprintf("nodes/%s/tasks?typefilter=vzdump&limit=100", c.si.Hostname))
	if err != nil {
		return nil
	}
	raw, _ := data["data"].([]any)

	type rawTask struct {
		start, end float64
		status, id string
		upid       string
	}
	var finished []rawTask
	for _, t := range raw {
		m, ok := t.(map[string]any)
		if !ok {
			continue
		}
		end, hasEnd := m["endtime"].(float64)
		start, hasStart := m["starttime"].(float64)
		id, _ := m["id"].(string)
		upid, _ := m["upid"].(string)
		status, hasStatus := c.taskStatus(m)
		if !hasEnd || !hasStart || !hasStatus {
			continue
		}
		finished = append(finished, rawTask{start, end, status, id, upid})
	}
	if len(finished) == 0 {
		return nil
	}

	// Sort by endtime DESC so we process most-recent first.
	sort.Slice(finished, func(i, j int) bool { return finished[i].end > finished[j].end })

	// Group tasks into the last job: two consecutive tasks belong to the same job if
	// the gap between one task ending and the next one starting is < 2 hours.
	// The gap is: prevTask.start - currentTask.end (going backwards in time).
	const maxGap = 2 * 3600.0
	var jobTasks []rawTask
	prevStart := finished[0].start
	for _, t := range finished {
		if len(jobTasks) > 0 && prevStart-t.end > maxGap {
			break
		}
		jobTasks = append(jobTasks, t)
		if t.start < prevStart {
			prevStart = t.start
		}
	}

	// Build results; deduplicate by VMID (first = most recent within the job).
	// Skip tasks with non-numeric id (job orchestration tasks, not per-VM tasks).
	// PBS pull-mode tasks use id format "vm/101" or "ct/101" instead of just "101".
	var jobStart int64 = 1 << 62
	var jobEnd int64
	var perVMTaskCount int
	for _, t := range jobTasks {
		if parseVMID(t.id) == 0 {
			continue
		}
		perVMTaskCount++
		if int64(t.start) < jobStart {
			jobStart = int64(t.start)
		}
		if int64(t.end) > jobEnd {
			jobEnd = int64(t.end)
		}
	}
	jobDuration := jobEnd - jobStart

	logDurations := make(map[int64]int64)
	var aggregateDuration int64
	for _, t := range jobTasks {
		if parseVMID(t.id) == 0 && aggregateDuration == 0 {
			aggregateDuration = int64(t.end - t.start)
		}
		for vmid, duration := range c.aggregateTaskDurations(t.upid) {
			logDurations[vmid] = duration
		}
	}

	seen := make(map[int64]bool)
	var result []map[string]any
	for _, t := range jobTasks {
		vmid := parseVMID(t.id)
		if vmid == 0 {
			continue
		}
		if seen[vmid] {
			continue
		}
		seen[vmid] = true

		start := int64(t.start)
		end := int64(t.end)
		duration := int64(t.end - t.start)
		var matchedFile backupFile
		var hasFile bool
		if d := logDurations[vmid]; d > 0 {
			duration = d
		} else if aggregateDuration > 0 && duration == aggregateDuration {
			duration = 0
		} else if perVMTaskCount > 1 && jobDuration > 0 && duration == jobDuration {
			duration = 0
		}
		task := map[string]any{
			"vmid":      vmid,
			"vm_name":   names[vmid],
			"status":    t.status,
			"starttime": start,
			"endtime":   end,
			"duration":  duration,
			"size":      int64(0),
			"filename":  "",
		}
		for _, f := range filesByVMID[vmid] {
			// File must have been created during this task's execution window.
			// ±300s of starttime caused cross-matching between consecutive jobs.
			if f.ctime >= int64(t.start)-60 && f.ctime <= int64(t.end)+60 {
				matchedFile = f
				hasFile = true
				break
			}
		}
		if hasFile {
			task["size"] = matchedFile.size
			task["filename"] = matchedFile.volid
			if logDurations[vmid] > 0 {
				task["starttime"] = matchedFile.ctime
				task["endtime"] = matchedFile.ctime + duration
			}
		}
		result = append(result, task)
	}
	if len(result) > 0 {
		return result
	}

	// Some scheduled multi-VM jobs are reported by PVE only as an aggregate task
	// with an empty id (vzdump::root@pam:). In that case, reconstruct per-VM
	// rows from backup files created during the aggregate job window.
	aggregate := jobTasks[0]
	durations := c.aggregateTaskDurations(aggregate.upid)
	var vmids []int64
	for vmid, files := range filesByVMID {
		for _, f := range files {
			if f.ctime >= int64(aggregate.start)-60 && f.ctime <= int64(aggregate.end)+60 {
				vmids = append(vmids, vmid)
				break
			}
		}
	}
	sort.Slice(vmids, func(i, j int) bool { return vmids[i] < vmids[j] })
	for _, vmid := range vmids {
		start := int64(aggregate.start)
		end := int64(aggregate.end)
		duration := int64(0)
		var matchedFile backupFile
		var hasFile bool
		for _, f := range filesByVMID[vmid] {
			if f.ctime >= int64(aggregate.start)-60 && f.ctime <= int64(aggregate.end)+60 {
				matchedFile = f
				hasFile = true
				break
			}
		}
		if d := durations[vmid]; d > 0 {
			duration = d
			if hasFile {
				start = matchedFile.ctime
				end = matchedFile.ctime + d
			}
		}
		task := map[string]any{
			"vmid":      vmid,
			"vm_name":   names[vmid],
			"status":    aggregate.status,
			"starttime": start,
			"endtime":   end,
			"duration":  duration,
			"size":      int64(0),
			"filename":  "",
		}
		if hasFile {
			task["size"] = matchedFile.size
			task["filename"] = matchedFile.volid
		}
		result = append(result, task)
	}
	return result
}

func (c *pveClient) taskStatus(task map[string]any) (string, bool) {
	if exitstatus := str(task["exitstatus"]); exitstatus != "" {
		return exitstatus, true
	}
	status := str(task["status"])
	if status == "stopped" {
		if upid := str(task["upid"]); upid != "" {
			if exitstatus := c.taskExitStatus(upid); exitstatus != "" {
				return exitstatus, true
			}
		}
	}
	if status != "" {
		return status, true
	}
	return "", false
}

func (c *pveClient) taskExitStatus(upid string) string {
	data, err := c.get(fmt.Sprintf("nodes/%s/tasks/%s/status", c.si.Hostname, url.PathEscape(upid)))
	if err != nil {
		log.Printf("WARN: could not read vzdump task status %s: %v", upid, err)
		return ""
	}
	payload, _ := data["data"].(map[string]any)
	return str(payload["exitstatus"])
}

func (c *pveClient) aggregateTaskDurations(upid string) map[int64]int64 {
	if upid == "" {
		return nil
	}
	var lines []string
	const pageLimit = 500
	for start := 0; ; start += pageLimit {
		data, err := c.get(fmt.Sprintf("nodes/%s/tasks/%s/log?start=%d&limit=%d", c.si.Hostname, url.PathEscape(upid), start, pageLimit))
		if err != nil {
			log.Printf("WARN: could not read vzdump task log %s: %v", upid, err)
			return nil
		}
		raw, _ := data["data"].([]any)
		for _, item := range raw {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := m["t"].(string); ok {
				lines = append(lines, text)
			}
		}
		if len(raw) < pageLimit {
			break
		}
	}
	durations := parseBackupDurations(lines)
	if len(durations) == 0 {
		log.Printf("WARN: no per-VM durations found in vzdump task log %s (%d lines)", upid, len(lines))
	}
	return durations
}

var finishedBackupREs = []*regexp.Regexp{
	regexp.MustCompile(`(?i)Finished Backup of (?:VM|CT)\s+(\d+)\s+\((\d{1,2}:\d{2}(?::\d{2})?)\)`),
	regexp.MustCompile(`(?i)Finished Backup of (?:VM|CT)\s+(\d+).*?\b(?:duration|in|took)\s+(\d{1,2}:\d{2}(?::\d{2})?)`),
	regexp.MustCompile(`(?i)Backup of (?:VM|CT)\s+(\d+).*?finished.*?\((\d{1,2}:\d{2}(?::\d{2})?)\)`),
}

func parseBackupDurations(lines []string) map[int64]int64 {
	durations := make(map[int64]int64)
	for _, line := range lines {
		for _, re := range finishedBackupREs {
			matches := re.FindStringSubmatch(line)
			if len(matches) != 3 {
				continue
			}
			vmid, err := strconv.ParseInt(matches[1], 10, 64)
			if err != nil {
				continue
			}
			duration, ok := parseClockDuration(matches[2])
			if ok {
				durations[vmid] = duration
			}
			break
		}
	}
	return durations
}

func parseClockDuration(s string) (int64, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, false
	}
	var nums []int64
	for _, part := range parts {
		n, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return 0, false
		}
		nums = append(nums, n)
	}
	if len(nums) == 2 {
		return nums[0]*60 + nums[1], true
	}
	return nums[0]*3600 + nums[1]*60 + nums[2], true
}

func (c *pveClient) lastBackupStatus() backupStatus {
	empty := backupStatus{OK: false, StartTime: -1, EndTime: -1, Duration: -1}
	data, err := c.get(fmt.Sprintf("nodes/%s/tasks?typefilter=vzdump&limit=50", c.si.Hostname))
	if err != nil {
		log.Printf("WARN: could not get backup tasks: %v", err)
		return empty
	}
	tasks, _ := data["data"].([]any)

	type task struct {
		end, start float64
		status     string
	}
	var finished []task
	for _, t := range tasks {
		m, ok := t.(map[string]any)
		if !ok {
			continue
		}
		end, hasEnd := m["endtime"].(float64)
		start, hasStart := m["starttime"].(float64)
		status, hasStatus := c.taskStatus(m)
		if hasEnd && hasStart && hasStatus {
			finished = append(finished, task{end, start, status})
		}
	}
	if len(finished) == 0 {
		return empty
	}
	sort.Slice(finished, func(i, j int) bool { return finished[i].end > finished[j].end })
	last := finished[0]
	return backupStatus{
		OK:        last.status == "OK",
		StartTime: int64(last.start),
		EndTime:   int64(last.end),
		Duration:  int64(last.end - last.start),
	}
}

func (c *pveClient) generateReport() (map[string]any, error) {
	storagesData, err := c.get("storage")
	if err != nil {
		return nil, fmt.Errorf("list storages: %w", err)
	}
	nodeStoragesData, _ := c.get(fmt.Sprintf("nodes/%s/storage", c.si.Hostname))

	storageList, _ := storagesData["data"].([]any)
	var nodeStorages []any
	if nodeStoragesData != nil {
		nodeStorages, _ = nodeStoragesData["data"].([]any)
	}

	now := time.Now().Format(time.RFC3339)
	var result []map[string]any
	filesByVMID := make(map[int64][]backupFile)

	for _, s := range storageList {
		sm, ok := s.(map[string]any)
		if !ok {
			continue
		}
		storageName := str(sm["storage"])

		sr := map[string]any{
			"digest":        str(sm["digest"]),
			"prune_backups": sm["prune-backups"],
			"shared":        boolVal(sm["shared"]),
			"server":        str(sm["server"]),
			"storage":       storageName,
			"export":        str(sm["export"]),
			"path":          str(sm["path"]),
			"content":       str(sm["content"]),
			"type":          str(sm["type"]),
			"created_at":    now,
			"status":        "online",
			"storage_info":  []any{},
			"content_data":  []any{},
		}

		for _, ns := range nodeStorages {
			nsm, ok := ns.(map[string]any)
			if !ok || str(nsm["storage"]) != storageName {
				continue
			}
			sr["storage_info"] = []any{map[string]any{
				"total":        nsm["total"],
				"used":         nsm["used"],
				"avail":        nsm["avail"],
				"used_percent": nsm["used-percent"],
				"enabled":      boolVal(nsm["enabled"]),
			}}
			break
		}

		contentData, err := c.get(fmt.Sprintf("nodes/%s/storage/%s/content", c.si.Hostname, storageName))
		if err == nil {
			items, _ := contentData["data"].([]any)
			var contents []any
			for _, item := range items {
				im, ok := item.(map[string]any)
				if !ok {
					continue
				}
				verif := ""
				if v, ok := im["verification"].(map[string]any); ok {
					verif, _ = v["state"].(string)
				}
				var ctime int64
				switch v := im["ctime"].(type) {
				case float64:
					ctime = int64(v)
				case string:
					ctime, _ = strconv.ParseInt(v, 10, 64)
				}
				contents = append(contents, map[string]any{
					"vmid":         im["vmid"],
					"format":       im["format"],
					"size":         im["size"],
					"content":      im["content"],
					"volid":        im["volid"],
					"verification": verif,
					"ctime":        ctime,
					"notes":        im["notes"],
					"subtype":      im["subtype"],
					"parent":       im["parent"],
					"created_at":   now,
				})
				if str(im["content"]) == "backup" {
					vmidF, _ := im["vmid"].(float64)
					sizeF, _ := im["size"].(float64)
					filesByVMID[int64(vmidF)] = append(filesByVMID[int64(vmidF)], backupFile{
						ctime: ctime,
						size:  int64(sizeF),
						volid: str(im["volid"]),
					})
				}
			}
			if contents != nil {
				sr["content_data"] = contents
			}
		} else if storageName != "local" {
			log.Printf("WARN: could not read storage %s content: %v", storageName, err)
			sr["status"] = "offline"
		}

		result = append(result, sr)
	}

	names := c.vmNames()
	tasks := c.backupJobTasks(names, filesByVMID)
	swap := c.si.swapInfo()

	return map[string]any{
		"hostname":           c.si.Hostname,
		"ip_address":         c.si.localIP(),
		"public_ip":          c.si.publicIP(),
		"client_version":     version,
		"machine_id":         c.si.machineID(),
		"swap_total":         swap.Total,
		"swap_used":          swap.Used,
		"swap_enabled":       swap.Enabled,
		"last_backup_status": c.reportBackupStatus(tasks),
		"storages":           result,
		"backup_tasks":       tasks,
	}, nil
}

func (c *pveClient) reportBackupStatus(tasks []map[string]any) backupStatus {
	if len(tasks) > 0 {
		return jobBackupStatus(tasks)
	}
	return c.lastBackupStatus()
}

// jobBackupStatus derives the overall backup status from all tasks in the job.
// If any VM failed, the job is considered failed - even if later VMs succeeded.
func jobBackupStatus(tasks []map[string]any) backupStatus {
	if len(tasks) == 0 {
		return backupStatus{OK: false, StartTime: -1, EndTime: -1, Duration: -1}
	}
	allOK := true
	var minStart int64 = 1 << 62
	var maxEnd int64
	for _, t := range tasks {
		if str(t["status"]) != "OK" {
			allOK = false
		}
		if s, _ := t["starttime"].(int64); s < minStart {
			minStart = s
		}
		if e, _ := t["endtime"].(int64); e > maxEnd {
			maxEnd = e
		}
	}
	return backupStatus{
		OK:        allOK,
		StartTime: minStart,
		EndTime:   maxEnd,
		Duration:  maxEnd - minStart,
	}
}

// parseVMID extracts a numeric VMID from a task id field.
// Handles both plain "101" and PBS pull-mode format "vm/101" / "ct/101".
func parseVMID(id string) int64 {
	// Try plain numeric first.
	if v, err := strconv.ParseInt(id, 10, 64); err == nil {
		return v
	}
	// Try "type/vmid" format (vm/101, ct/101, etc.).
	if i := strings.LastIndex(id, "/"); i >= 0 {
		if v, err := strconv.ParseInt(id[i+1:], 10, 64); err == nil {
			return v
		}
	}
	return 0
}

func str(v any) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

// boolVal converts Proxmox API values (true/false or 0/1) to bool.
func boolVal(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	}
	return false
}
