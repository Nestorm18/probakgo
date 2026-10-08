package service

import (
	"context"
	"errors"
	"testing"

	"probakgo/internal/domain"
	"probakgo/internal/store"
)

func TestPartialEvaluationKeepsAlertingWithoutFalseResolutions(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)

	previous := domain.Alert{ID: "pve_heartbeat:pve:1", Type: domain.AlertTypePVEHeartbeat, Severity: domain.AlertSeverityCritical, Title: "Servidor offline"}
	if err := st.SyncAlertStates(ctx, []domain.Alert{previous}); err != nil {
		t.Fatalf("seed alert state: %v", err)
	}

	detected := domain.Alert{ID: "disk:pve:2:local", Type: domain.AlertTypeDisk, Severity: domain.AlertSeverityCritical, Title: "Disco lleno"}
	saved := evaluators
	t.Cleanup(func() { evaluators = saved })
	evaluators = []AlertEvaluator{
		func(*store.Store, AlertConfigs) ([]domain.Alert, error) { return []domain.Alert{detected}, nil },
		func(*store.Store, AlertConfigs) ([]domain.Alert, error) { return nil, errors.New("corrupt data") },
	}

	err := sendImmediateCriticalAlerts(ctx, st, nil)
	if !IsPartialAlertsError(err) {
		t.Fatalf("expected a partial evaluation error, got %v", err)
	}
	present, err := st.ListPresentAlerts(ctx)
	if err != nil {
		t.Fatalf("list present alerts: %v", err)
	}
	ids := map[string]bool{}
	for _, alert := range present {
		ids[alert.ID] = true
	}
	if !ids[detected.ID] {
		t.Fatalf("alert from a healthy evaluator was not recorded: %+v", present)
	}
	if !ids[previous.ID] {
		t.Fatalf("alert missing from a partial evaluation was resolved: %+v", present)
	}
}
