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
