package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type pbsClient struct {
	cfg     *Config
	si      *SysInfo
	http    *http.Client
	baseURL string
}

func newPBSClient(cfg *Config, si *SysInfo) *pbsClient {
	return &pbsClient{
		cfg:     cfg,
		si:      si,
		http:    newHTTPClient(cfg),
		baseURL: fmt.Sprintf("https://%s:8007/api2/json", si.localIP()),
	}
}

func (c *pbsClient) get(endpoint string) (map[string]any, error) {
	url := c.baseURL + "/" + endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// PBS uses colon separator: PBSAPIToken=user:tokenid:secret
	req.Header.Set("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s", c.cfg.ProxmoxToken, c.cfg.ProxmoxSecret))

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("WARN: PBS %s attempt %d/3: %v", endpoint, attempt, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 401 {
			return nil, fmt.Errorf("PBS auth error (401): check PROXMOX_TOKEN and PROXMOX_SECRET")
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("PBS API %s returned HTTP %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(body)))
		}
		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = fmt.Errorf("invalid JSON: %w", err)
			log.Printf("WARN: PBS %s attempt %d/3: %v", endpoint, attempt, lastErr)
			continue
		}
		return result, nil
	}
	return nil, fmt.Errorf("PBS API %s failed after 3 attempts: %w", endpoint, lastErr)
}

func (c *pbsClient) validateConnection() bool {
	_, err := c.get("version")
	if err != nil {
		log.Printf("ERROR: %v", err)
	}
	return err == nil
}

func (c *pbsClient) generateReport() (map[string]any, error) {
	data, err := c.get("status/datastore-usage")
	if err != nil {
		return nil, fmt.Errorf("datastore-usage: %w", err)
	}
	swap := c.si.swapInfo()
	if list, ok := data["data"].([]any); ok {
		for _, item := range list {
			ds, ok := item.(map[string]any)
			if !ok {
				continue
			}
			storeName, _ := ds["store"].(string)
			if storeName == "" {
				continue
			}
			groups, err := c.get(fmt.Sprintf("admin/datastore/%s/groups", storeName))
			if err != nil {
				log.Printf("WARN: PBS groups for %q: %v", storeName, err)
				continue
			}
			groupList, _ := groups["data"].([]any)

			// Fetch snapshot sizes and verification states (groups endpoint omits both).
			// Build map (backup-type/backup-id/backup-time) → {size, verification-state}.
			type snapInfo struct {
				size       int64
				verifState string
			}
			snapData := map[string]snapInfo{}
			if snaps, err := c.get(fmt.Sprintf("admin/datastore/%s/snapshots", storeName)); err == nil {
				for _, s := range snaps["data"].([]any) {
					snap, ok := s.(map[string]any)
					if !ok {
						continue
					}
					bt, _ := snap["backup-type"].(string)
					bi, _ := snap["backup-id"].(string)
					ts, _ := snap["backup-time"].(float64)
					sz, _ := snap["size"].(float64)
					var verifState string
					if v, ok := snap["verification"].(map[string]any); ok {
						verifState, _ = v["state"].(string)
					}
					key := fmt.Sprintf("%s/%s/%d", bt, bi, int64(ts))
					snapData[key] = snapInfo{size: int64(sz), verifState: verifState}
				}
			} else {
				log.Printf("WARN: PBS snapshots for %q: %v", storeName, err)
			}

			// Inject size and verification-state of the latest snapshot into each group.
			for _, g := range groupList {
				grp, ok := g.(map[string]any)
				if !ok {
					continue
				}
				bt, _ := grp["backup-type"].(string)
				bi, _ := grp["backup-id"].(string)
				lastBackup, _ := grp["last-backup"].(float64)
				key := fmt.Sprintf("%s/%s/%d", bt, bi, int64(lastBackup))
				if info, found := snapData[key]; found {
					grp["size"] = info.size
					grp["verification-state"] = info.verifState
				}
			}
			ds["groups"] = groupList
		}
	}
	tasks := c.maintenanceTasks()
	return map[string]any{
		"hostname":       c.si.Hostname,
		"ip_address":     c.si.localIP(),
		"public_ip":      c.si.publicIP(),
		"client_version": version,
		"machine_id":     c.si.machineID(),
		"swap_total":     swap.Total,
		"swap_used":      swap.Used,
		"swap_enabled":   swap.Enabled,
		"pbs_information": map[string]any{
			"data":  data["data"],
			"tasks": tasks,
		},
	}, nil
}

type pbsSyncJob struct {
	remote      string
	remoteStore string
	store       string
}

// maintenanceTasks returns the latest completed sync and garbage collection task
// for each PBS job/datastore. Task status is fetched separately because older PBS
// releases do not always include exitstatus in the list endpoint.
func (c *pbsClient) maintenanceTasks() []map[string]any {
	result, err := c.get("nodes/localhost/tasks?limit=200")
	if err != nil {
		log.Printf("WARN: PBS maintenance tasks: %v", err)
		return nil
	}
	rawTasks, _ := result["data"].([]any)
	if len(rawTasks) == 0 {
		return nil
	}

	jobs := c.syncJobs()
	latest := make(map[string]map[string]any)
	for _, raw := range rawTasks {
		task, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		kind, jobID := classifyPBSTask(task)
		if kind == "" {
			continue
		}
		key := kind + ":" + jobID
		if existing, found := latest[key]; !found || taskTime(task) > taskTime(existing) {
			latest[key] = task
		}
	}

	out := make([]map[string]any, 0, len(latest))
	for _, task := range latest {
		kind, jobID := classifyPBSTask(task)
		status := taskString(task, "exitstatus")
		if status == "" || strings.EqualFold(status, "stopped") {
			upid := taskString(task, "upid")
			if upid != "" {
				if detail, err := c.get("nodes/localhost/tasks/" + url.PathEscape(upid) + "/status"); err == nil {
					if data, ok := detail["data"].(map[string]any); ok {
						status = taskString(data, "exitstatus")
					}
				}
			}
		}
		if status == "" || strings.EqualFold(status, "running") || strings.EqualFold(status, "stopped") {
			continue
		}

		remote := taskString(task, "remote")
		remoteStore := taskString(task, "remote-store")
		store := taskString(task, "store")
		if kind == "sync" {
			if job, ok := jobs[jobID]; ok {
				if remote == "" {
					remote = job.remote
				}
				if remoteStore == "" {
					remoteStore = job.remoteStore
				}
				if store == "" {
					store = job.store
				}
			}
		}
		if kind == "gc" && store == "" {
			store = jobID
		}
		out = append(out, map[string]any{
			"task_type":    kind,
			"job_id":       jobID,
			"remote":       remote,
			"remote_store": remoteStore,
			"store":        store,
			"status":       status,
			"start_time":   taskNumber(task, "starttime"),
			"end_time":     taskNumber(task, "endtime"),
			"upid":         taskString(task, "upid"),
		})
	}
	return out
}

func (c *pbsClient) syncJobs() map[string]pbsSyncJob {
	result, err := c.get("config/sync")
	if err != nil {
		log.Printf("WARN: PBS sync configuration: %v", err)
		return nil
	}
	entries, _ := result["data"].([]any)
	jobs := make(map[string]pbsSyncJob, len(entries))
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := taskString(entry, "id")
		if id == "" {
			continue
		}
		jobs[id] = pbsSyncJob{
			remote:      taskString(entry, "remote"),
			remoteStore: taskString(entry, "remote-store"),
			store:       taskString(entry, "store"),
		}
	}
	return jobs
}

func classifyPBSTask(task map[string]any) (kind, jobID string) {
	workerID := taskString(task, "worker_id")
	if workerID == "" {
		workerID = taskString(task, "id")
	}
	upidType, upidID := pbsUPIDTask(taskString(task, "upid"))
	if workerID == "" {
		workerID = upidID
	}
	text := strings.ToLower(strings.Join([]string{
		taskString(task, "worker_type"), taskString(task, "type"), upidType, taskString(task, "upid"), workerID,
	}, " "))
	switch {
	case strings.Contains(text, "sync"):
		return "sync", workerID
	case strings.Contains(text, "garbage"), strings.Contains(text, ":gc"):
		return "gc", workerID
	default:
		return "", ""
	}
}

func pbsUPIDTask(upid string) (taskType, workerID string) {
	parts := strings.Split(upid, ":")
	if len(parts) < 7 || parts[0] != "UPID" {
		return "", ""
	}
	return parts[5], parts[6]
}

func taskString(task map[string]any, key string) string {
	if value, ok := task[key]; ok {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return ""
}

func taskNumber(task map[string]any, key string) int64 {
	switch value := task[key].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	default:
		return 0
	}
}

func taskTime(task map[string]any) int64 {
	if end := taskNumber(task, "endtime"); end > 0 {
		return end
	}
	return taskNumber(task, "starttime")
}
