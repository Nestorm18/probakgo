package webhandlers

import (
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestPBSFillBadge_IgnoresPastEstimateAndUsesPercent(t *testing.T) {
	label, class := pbsFillBadge([]domain.PBSStore{{
		Total:             2948636082176,
		Used:              2620000000000,
		EstimatedFullDate: time.Now().Add(-24 * time.Hour).Unix(),
	}})

	if label != "88% · Sin riesgo" || class != "ok" {
		t.Fatalf("want 88%% · Sin riesgo ok, got %s %s", label, class)
	}
}

func TestPBSFillBadge_FutureEstimateWins(t *testing.T) {
	label, class := pbsFillBadge([]domain.PBSStore{{
		Total:             2948636082176,
		Used:              2620000000000,
		EstimatedFullDate: time.Now().Add(10 * 24 * time.Hour).Unix(),
	}})

	if label != "Lleno en 10d" || class != "bad" {
		t.Fatalf("want Lleno en 10d bad, got %s %s", label, class)
	}
}

func TestPBSStoreDisplays_UsesSameFillStatus(t *testing.T) {
	rows := pbsStoreDisplays([]domain.PBSStore{{
		Store:             "synology",
		Total:             2948636082176,
		Used:              2620000000000,
		EstimatedFullDate: time.Now().Add(-24 * time.Hour).Unix(),
	}})

	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].StoreName != "synology" || rows[0].BadgeLabel != "88%" || rows[0].BadgeClass != "ok" || !rows[0].NoFillRisk {
		t.Fatalf("unexpected row: %+v", rows[0])
	}
}
