package webhandlers

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dbpkg "probakgo/internal/db"
	"probakgo/internal/domain"
	"probakgo/internal/service"
	"probakgo/internal/store"
)

func newWorkbookTestHandler(t *testing.T) (*WebH, *store.Store, *sql.DB) {
	t.Helper()
	database, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	st := store.New(database)
	return New(st, nil, service.NewReport(st, time.UTC)), st, database
}

func TestPVEWorkbookExportsLatestBackupPerVM(t *testing.T) {
	h, st, database := newWorkbookTestHandler(t)
	ctx := context.Background()
	serverID, _ := st.UpsertPVEServer(ctx, "pve-1", "10.0.0.1", "", "0.0.228", "machine-pve")
	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)
	for _, task := range []domain.BackupTaskPayload{
		{VMID: 607, VMName: "ABALwin-antigua", Status: "OK", StartTime: 100, EndTime: 120, Duration: 20, Size: 10, Filename: "old"},
		{VMID: 607, VMName: "ABALwin", Status: "OK", StartTime: time.Now().Add(-time.Hour).Unix(), EndTime: time.Now().Unix(), Duration: 2098, Size: 1_870_000_000_000, Filename: "PBS:backup/vm/607/2026-08-19T18:15:00Z"},
		{VMID: 608, VMName: "ABALvpn", Status: "OK", StartTime: time.Now().Add(-time.Minute).Unix(), EndTime: time.Now().Unix(), Duration: 25, Size: 32_210_000_000, Filename: "PBS:backup/vm/608/2026-08-19T18:49:59Z"},
	} {
		if err := st.InsertPVEBackupTask(ctx, reportID, task); err != nil {
			t.Fatalf("insert PVE task: %v", err)
		}
	}
	oldServerID, _ := st.UpsertPVEServer(ctx, "pve-sin-reporte-hoy", "10.0.0.2", "", "0.0.228", "machine-pve-old")
	oldReportID, _ := st.InsertPVEReport(ctx, oldServerID, nil)
	if err := st.InsertPVEBackupTask(ctx, oldReportID, domain.BackupTaskPayload{VMID: 609, VMName: "ABALUbiquity", Status: "OK", StartTime: time.Now().Add(-25 * time.Hour).Unix(), Duration: 43, Size: 42_950_000_000, Filename: "PBS:backup/vm/609/last"}); err != nil {
		t.Fatalf("insert old PVE task: %v", err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE pve_reports SET reported_at = ? WHERE id = ?`, time.Now().Add(-25*time.Hour), oldReportID); err != nil {
		t.Fatalf("age PVE report: %v", err)
	}

	rr := httptest.NewRecorder()
	h.PVEServersXLSX(rr, httptest.NewRequest(http.MethodGet, "/servers/pve.xlsx", nil))
	assertWorkbookResponse(t, rr, []string{"Últimas copias", "ABALwin", "ABALvpn", "ABALUbiquity", "pve-sin-reporte-hoy", "Actual", "Último disponible", "PBS:backup/vm/607"}, []string{"ABALwin-antigua"})
	writeWorkbookQA(t, "pve.xlsx", rr.Body.Bytes())
}

func TestPBSWorkbookExportsDatastoresAndTasks(t *testing.T) {
	h, st, database := newWorkbookTestHandler(t)
	ctx := context.Background()
	serverID, _ := st.UpsertPBSServer(ctx, "pbs-1", "10.0.1.1", "", "0.0.228", "machine-pbs")
	reportID, _ := st.InsertPBSReport(ctx, serverID)
	if _, err := st.InsertPBSStore(ctx, reportID, domain.PBSDatastorePayload{Store: "backup", Total: 2_000_000_000, Used: 1_000_000_000, Avail: 1_000_000_000, MountStatus: "mounted"}); err != nil {
		t.Fatalf("insert PBS store: %v", err)
	}
	if err := st.InsertPBSTask(ctx, reportID, domain.PBSTaskPayload{TaskType: "sync", JobID: "sync-job", Remote: "remote-1", Store: "backup", Status: "OK", StartTime: 100, EndTime: 160, UPID: "UPID:test"}); err != nil {
		t.Fatalf("insert PBS task: %v", err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE pbs_reports SET reported_at = ? WHERE id = ?`, time.Now().Add(-25*time.Hour), reportID); err != nil {
		t.Fatalf("age PBS report: %v", err)
	}

	rr := httptest.NewRecorder()
	h.PBSServersXLSX(rr, httptest.NewRequest(http.MethodGet, "/servers/pbs.xlsx", nil))
	assertWorkbookResponse(t, rr, []string{"Datastores", "Tareas", "backup", "sync-job", "UPID:test", "Último disponible"}, nil)
	writeWorkbookQA(t, "pbs.xlsx", rr.Body.Bytes())
}

func TestWindowsWorkbookExportsOnlineServerDisks(t *testing.T) {
	h, st, database := newWorkbookTestHandler(t)
	ctx := context.Background()
	serverID, _ := st.UpsertWindowsServer(ctx, "win-1", "10.0.2.1", "", "0.0.228", "machine-win")
	reportID, _ := st.InsertWindowsReport(ctx, serverID)
	if err := st.InsertWindowsDisk(ctx, reportID, domain.WindowsDiskPayload{Name: "C:", Label: "System", FileSystem: "NTFS", DriveType: "Fixed", Total: 500_000_000_000, Used: 250_000_000_000, Free: 250_000_000_000, Health: "OK"}); err != nil {
		t.Fatalf("insert Windows disk: %v", err)
	}
	if err := st.UpsertServerHeartbeat(ctx, domain.ServerHeartbeat{ServerType: "windows", ServerID: serverID, Hostname: "win-1", LastSeenAt: time.Now()}); err != nil {
		t.Fatalf("insert Windows heartbeat: %v", err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE windows_reports SET reported_at = ? WHERE id = ?`, time.Now().Add(-25*time.Hour), reportID); err != nil {
		t.Fatalf("age Windows report: %v", err)
	}

	rr := httptest.NewRecorder()
	h.WindowsServersXLSX(rr, httptest.NewRequest(http.MethodGet, "/servers/windows.xlsx", nil))
	assertWorkbookResponse(t, rr, []string{"Discos", "win-1", "System", "NTFS", "Último disponible"}, nil)
	writeWorkbookQA(t, "windows.xlsx", rr.Body.Bytes())
}

func writeWorkbookQA(t *testing.T, filename string, data []byte) {
	t.Helper()
	directory := os.Getenv("PROBAKGO_XLSX_QA_DIR")
	if directory == "" {
		return
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create XLSX QA directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directory, filename), data, 0o600); err != nil {
		t.Fatalf("write XLSX QA workbook: %v", err)
	}
}

func assertWorkbookResponse(t *testing.T, rr *httptest.ResponseRecorder, wants, rejects []string) {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != xlsxContentType {
		t.Fatalf("Content-Type = %q", got)
	}
	reader, err := zip.NewReader(bytes.NewReader(rr.Body.Bytes()), int64(rr.Body.Len()))
	if err != nil {
		t.Fatalf("open XLSX response: %v", err)
	}
	var content strings.Builder
	for _, file := range reader.File {
		if !strings.HasSuffix(file.Name, ".xml") {
			continue
		}
		stream, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}
		raw, _ := io.ReadAll(stream)
		_ = stream.Close()
		content.Write(raw)
	}
	for _, want := range wants {
		if !strings.Contains(content.String(), want) {
			t.Fatalf("workbook is missing %q", want)
		}
	}
	for _, reject := range rejects {
		if strings.Contains(content.String(), reject) {
			t.Fatalf("workbook unexpectedly contains %q", reject)
		}
	}
}
