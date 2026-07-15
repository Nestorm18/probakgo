package webhandlers

import (
	"strings"
	"testing"
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
