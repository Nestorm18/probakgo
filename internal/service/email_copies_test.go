package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestBuildEmailDataCountsRetainedCopies(t *testing.T) {
	_, st := openTestStore(t)
	ctx := t.Context()
	serverID, err := st.UpsertPVEServer(ctx, "pve-copies", "10.0.0.1", "", "test", "copies-machine")
	if err != nil {
		t.Fatal(err)
	}
	reportID, err := st.InsertPVEReport(ctx, serverID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.InsertPVEBackupTask(ctx, reportID, domain.BackupTaskPayload{VMID: 100, VMName: "vm-test", Status: "OK", StartTime: time.Now().Unix()}); err != nil {
		t.Fatal(err)
	}
	for _, source := range []struct {
		kind  string
		count int
	}{{"pbs", 20}, {"dir", 3}} {
		storageID, err := st.InsertPVEStorage(ctx, reportID, domain.StoragePayload{Storage: source.kind, Type: source.kind, Content: "backup"})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < source.count; i++ {
			if err := st.InsertPVEStorageContent(ctx, storageID, domain.ContentDataPayload{VMID: 100, Content: "backup", VolID: fmt.Sprintf("%s:backup/%d", source.kind, i)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	cfg, err := st.GetEmailConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data, err := buildEmailData(ctx, st, NewReport(st, time.UTC), cfg)
	if err != nil {
		t.Fatal(err)
	}
	servers := append(data.PVEOk, data.PVEIssues...)
	if len(servers) != 1 || len(servers[0].VMTasks) != 1 || servers[0].VMTasks[0].Copies != 23 {
		t.Fatalf("expected 23 retained copies: %+v", servers)
	}
	body, err := renderEmailTemplate(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, ">23</td>") || strings.Index(body, ">Copias</td>") <= strings.Index(body, ">Tamaño</td>") {
		t.Fatal("email must show the copy count after size")
	}
}

func TestBuildEmailDataSkipsSilencedStaleServers(t *testing.T) {
	_, st := openTestStore(t)
	ctx := t.Context()
	pveID, err := st.UpsertPVEServer(ctx, "pve-silenced", "10.0.0.2", "", "test", "pve-silenced-machine")
	if err != nil {
		t.Fatal(err)
	}
	pbsID, err := st.UpsertPBSServer(ctx, "pbs-silenced", "10.0.0.3", "", "test", "pbs-silenced-machine")
	if err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(24 * time.Hour)
	for _, id := range []string{fmt.Sprintf("pve_stale:pve:%d", pveID), fmt.Sprintf("pbs_report_stale:pbs:%d", pbsID)} {
		if err := st.UpsertAlertSuppression(ctx, id, until, ""); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := st.GetEmailConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data, err := buildEmailData(ctx, st, NewReport(st, time.UTC), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.PVEIssues) != 0 || len(data.PBSIssues) != 0 || data.TotalIssues != 0 {
		t.Fatalf("silenced stale servers must not be reported: pve=%+v pbs=%+v total=%d", data.PVEIssues, data.PBSIssues, data.TotalIssues)
	}
}
