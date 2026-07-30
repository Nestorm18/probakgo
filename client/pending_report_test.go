package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRetryPendingReportRetainsMarkerOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".report_pending")
	if err := os.WriteFile(path, []byte("pending"), 0600); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	wantErr := errors.New("send failed")

	err := retryPendingReport(path, func() error { return wantErr })

	if !errors.Is(err, wantErr) {
		t.Fatalf("retry error: got %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("pending marker was removed after failure: %v", err)
	}
}

func TestRetryPendingReportClearsMarkerOnSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".report_pending")
	if err := os.WriteFile(path, []byte("pending"), 0600); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	called := false

	if err := retryPendingReport(path, func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("retry pending report: %v", err)
	}
	if !called {
		t.Fatal("pending report sender was not called")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("pending marker still exists: %v", err)
	}
}
