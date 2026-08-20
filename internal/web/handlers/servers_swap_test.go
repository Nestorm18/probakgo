package webhandlers

import (
	"strings"
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestBuildPVESwapListView(t *testing.T) {
	base := buildSwapView(true, 0, 2_000_000_000)

	active := buildPVESwapListView(base, true, false)
	if active.CSSClass != "bad" {
		t.Fatalf("active swap alert class = %q, want bad", active.CSSClass)
	}

	disabled := buildPVESwapListView(base, false, false)
	if disabled.CSSClass != "muted" || !strings.Contains(disabled.Title, "desactivada") {
		t.Fatalf("disabled swap alert = %+v, want muted disabled state", disabled)
	}

	suppressed := buildPVESwapListView(base, true, true)
	if suppressed.CSSClass != "muted" || !strings.Contains(suppressed.Title, "suprimida") {
		t.Fatalf("suppressed swap alert = %+v, want muted suppressed state", suppressed)
	}
}

func TestCurrentPVESwapViewUsesNewerHeartbeat(t *testing.T) {
	reportedAt := time.Now().Add(-time.Minute)
	report := &domain.PVEReport{
		ReportedAt: reportedAt, SwapTotal: 2_000_000_000, SwapUsed: 128_000_000, SwapEnabled: true,
	}
	heartbeat := &domain.ServerHeartbeat{
		SwapReported: true, SwapEnabled: false, LastSeenAt: reportedAt.Add(time.Second),
	}

	view := currentPVESwapView(report, heartbeat)
	if view.Enabled || view.CSSClass != "ok" {
		t.Fatalf("current swap view = %+v, want heartbeat state without swap", view)
	}
}

func TestCurrentPVESwapViewKeepsNewerReport(t *testing.T) {
	reportedAt := time.Now()
	report := &domain.PVEReport{
		ReportedAt: reportedAt, SwapTotal: 2_000_000_000, SwapUsed: 128_000_000, SwapEnabled: true,
	}
	heartbeat := &domain.ServerHeartbeat{
		SwapReported: true, SwapEnabled: false, LastSeenAt: reportedAt.Add(-time.Second),
	}

	view := currentPVESwapView(report, heartbeat)
	if !view.Enabled || view.Total != report.SwapTotal || view.Used != report.SwapUsed {
		t.Fatalf("current swap view = %+v, want newer report state", view)
	}
}
