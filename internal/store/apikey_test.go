package store

import (
	"context"
	"testing"
)

func TestListAPIKeysPageSearchAndPaging(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	if _, err := st.CreateAPIKey(ctx, "alpha", "pve-alpha", "https://10.0.0.1:8006"); err != nil {
		t.Fatalf("create alpha key: %v", err)
	}
	if _, err := st.CreateAPIKey(ctx, "beta", "pbs-beta", "https://10.0.0.2:8007"); err != nil {
		t.Fatalf("create beta key: %v", err)
	}
	if _, err := st.CreateAPIKey(ctx, "gamma", "win-gamma", ""); err != nil {
		t.Fatalf("create gamma key: %v", err)
	}

	page1, err := st.ListAPIKeysPage(ctx, 2, 0, "")
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("first page rows: got %d, want 2", len(page1))
	}
	if page1[0].Name != "gamma" || page1[1].Name != "beta" {
		t.Fatalf("first page names: got %q, %q", page1[0].Name, page1[1].Name)
	}

	page2, err := st.ListAPIKeysPage(ctx, 2, 2, "")
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(page2) != 1 || page2[0].Name != "alpha" {
		t.Fatalf("second page: got %+v", page2)
	}

	found, err := st.ListAPIKeysPage(ctx, 10, 0, "10.0.0.2")
	if err != nil {
		t.Fatalf("search keys: %v", err)
	}
	if len(found) != 1 || found[0].Name != "beta" {
		t.Fatalf("search result: got %+v", found)
	}
}
