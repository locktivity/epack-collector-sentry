package collector

import (
	"testing"
	"time"

	"github.com/locktivity/epack-collector-sentry/internal/sentry"
)

func TestComputeMonitorMetrics_Empty(t *testing.T) {
	m, trunc := computeMonitorMetrics(nil, LevelTrust)
	if m.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", m.TotalCount)
	}
	if m.WithAlertTargetsPct != nil {
		t.Errorf("WithAlertTargetsPct = %v, want nil", m.WithAlertTargetsPct)
	}
	if trunc != nil {
		t.Errorf("truncation = %v, want nil", trunc)
	}
}

func TestComputeMonitorMetrics_Mixed(t *testing.T) {
	monitors := []sentry.Monitor{
		{Status: "active", AlertRule: &sentry.MonitorAlert{Targets: []sentry.AlertTarget{{TargetType: 1, TargetIdentifier: 42}}}},
		{Status: "active", AlertRule: &sentry.MonitorAlert{Targets: []sentry.AlertTarget{{TargetType: 1, TargetIdentifier: 43}}}},
		{Status: "disabled"},
		{Status: "active", IsMuted: true},
	}

	m, _ := computeMonitorMetrics(monitors, LevelTrust)

	if m.TotalCount != 4 {
		t.Errorf("TotalCount = %d, want 4", m.TotalCount)
	}
	if m.ActiveCount != 3 {
		t.Errorf("ActiveCount = %d, want 3", m.ActiveCount)
	}
	if m.DisabledCount != 1 {
		t.Errorf("DisabledCount = %d, want 1", m.DisabledCount)
	}
	if m.MutedCount != 1 {
		t.Errorf("MutedCount = %d, want 1", m.MutedCount)
	}
	if m.WithAlertTargetsCount != 2 {
		t.Errorf("WithAlertTargetsCount = %d, want 2", m.WithAlertTargetsCount)
	}
	if m.WithAlertTargetsPct == nil || *m.WithAlertTargetsPct != 50 {
		t.Errorf("WithAlertTargetsPct = %v, want 50", m.WithAlertTargetsPct)
	}
}

func TestComputeMonitorMetrics_AllActive(t *testing.T) {
	monitors := []sentry.Monitor{
		{Status: "ok", AlertRule: &sentry.MonitorAlert{Targets: []sentry.AlertTarget{{TargetType: 1, TargetIdentifier: 1}}}},
		{Status: "error", AlertRule: &sentry.MonitorAlert{Targets: []sentry.AlertTarget{{TargetType: 1, TargetIdentifier: 2}}}},
		{Status: "missed_checkin", AlertRule: &sentry.MonitorAlert{Targets: []sentry.AlertTarget{{TargetType: 1, TargetIdentifier: 3}}}},
	}

	m, _ := computeMonitorMetrics(monitors, LevelTrust)

	if m.ActiveCount != 3 {
		t.Errorf("ActiveCount = %d, want 3", m.ActiveCount)
	}
	if m.WithAlertTargetsPct == nil || *m.WithAlertTargetsPct != 100 {
		t.Errorf("WithAlertTargetsPct = %v, want 100", m.WithAlertTargetsPct)
	}
}

func TestComputeMonitorMetrics_NoAlertTargets(t *testing.T) {
	monitors := []sentry.Monitor{
		{Status: "active", AlertRule: nil},
		{Status: "active", AlertRule: &sentry.MonitorAlert{Targets: nil}},
	}

	m, _ := computeMonitorMetrics(monitors, LevelTrust)

	if m.WithAlertTargetsCount != 0 {
		t.Errorf("WithAlertTargetsCount = %d, want 0", m.WithAlertTargetsCount)
	}
	if m.WithAlertTargetsPct == nil || *m.WithAlertTargetsPct != 0 {
		t.Errorf("WithAlertTargetsPct = %v, want 0", m.WithAlertTargetsPct)
	}
}

func TestComputeMonitorMetrics_TrustOmitsInventory(t *testing.T) {
	monitors := []sentry.Monitor{{ID: "1", Status: "active"}}

	m, _ := computeMonitorMetrics(monitors, LevelTrust)

	if m.Inventory != nil {
		t.Errorf("expected nil inventory at trust, got %d items", len(m.Inventory))
	}
}

func TestComputeMonitorMetrics_AuditIncludesInventory(t *testing.T) {
	now := time.Now()
	monitors := []sentry.Monitor{
		{
			ID:     "abc123",
			Name:   "daily-sync",
			Type:   "cron_job",
			Status: "active",
			Config: sentry.MonitorConfig{ScheduleType: "crontab", Schedule: "0 2 * * *"},
			Project: sentry.MonitorProject{Slug: "billing"},
			Owner:  &sentry.MonitorOwner{Type: "team", Name: "backend"},
			AlertRule: &sentry.MonitorAlert{
				Targets: []sentry.AlertTarget{{TargetType: 2, TargetIdentifier: 42}},
			},
			Environments: []sentry.MonitorEnv{
				{Name: "production", Status: "ok", LastCheckIn: &now},
				{Name: "staging", Status: "ok"},
			},
		},
	}

	m, _ := computeMonitorMetrics(monitors, LevelAudit)

	if len(m.Inventory) != 1 {
		t.Fatalf("expected 1 inventory item, got %d", len(m.Inventory))
	}

	inv := m.Inventory[0]
	if inv.ID != "abc123" {
		t.Errorf("ID = %q, want %q", inv.ID, "abc123")
	}
	if inv.Owner != "team:backend" {
		t.Errorf("Owner = %q, want %q", inv.Owner, "team:backend")
	}
	if len(inv.ConnectedWorkflows) != 1 || inv.ConnectedWorkflows[0].TargetType != "team" {
		t.Errorf("ConnectedWorkflows = %v, want [{team 42}]", inv.ConnectedWorkflows)
	}
	if inv.LatestGroup == nil || inv.LatestGroup.Environment != "production" {
		t.Errorf("LatestGroup = %v, want production env", inv.LatestGroup)
	}
	if len(inv.Environments) != 2 {
		t.Errorf("Environments = %v, want [production staging]", inv.Environments)
	}
}

func TestComputeMonitorMetrics_InternalIncludesOpenAlertCount(t *testing.T) {
	now := time.Now()
	monitors := []sentry.Monitor{
		{
			ID: "1", Status: "active",
			Environments: []sentry.MonitorEnv{
				{Name: "production", Status: "error", ActiveIncident: &sentry.ActiveIncident{StartingTimestamp: now, ResolvingTimestamp: nil}},
				{Name: "staging", Status: "ok", ActiveIncident: nil},
			},
		},
	}

	m, _ := computeMonitorMetrics(monitors, LevelInternal)
	if m.Inventory[0].OpenAlertCount == nil {
		t.Fatal("expected OpenAlertCount to be set at internal level")
	}
	if *m.Inventory[0].OpenAlertCount != 1 {
		t.Errorf("OpenAlertCount = %d, want 1", *m.Inventory[0].OpenAlertCount)
	}
}

func TestComputeMonitorMetrics_AuditOmitsOpenAlertCount(t *testing.T) {
	now := time.Now()
	monitors := []sentry.Monitor{
		{
			ID: "1", Status: "active",
			Environments: []sentry.MonitorEnv{
				{Name: "production", Status: "error", ActiveIncident: &sentry.ActiveIncident{StartingTimestamp: now}},
			},
		},
	}

	m, _ := computeMonitorMetrics(monitors, LevelAudit)
	if m.Inventory[0].OpenAlertCount != nil {
		t.Errorf("expected nil OpenAlertCount at audit level, got %d", *m.Inventory[0].OpenAlertCount)
	}
}

func TestComputeMonitorMetrics_ConnectedWorkflowTypes(t *testing.T) {
	monitors := []sentry.Monitor{
		{
			ID: "1", Status: "active",
			AlertRule: &sentry.MonitorAlert{
				Targets: []sentry.AlertTarget{
					{TargetType: 1, TargetIdentifier: 10},
					{TargetType: 2, TargetIdentifier: 20},
					{TargetType: 99, TargetIdentifier: 30},
				},
			},
		},
	}

	m, _ := computeMonitorMetrics(monitors, LevelAudit)

	wf := m.Inventory[0].ConnectedWorkflows
	if len(wf) != 3 {
		t.Fatalf("expected 3 workflows, got %d", len(wf))
	}
	if wf[0].TargetType != "user" {
		t.Errorf("wf[0].TargetType = %q, want user", wf[0].TargetType)
	}
	if wf[1].TargetType != "team" {
		t.Errorf("wf[1].TargetType = %q, want team", wf[1].TargetType)
	}
	if wf[2].TargetType != "unknown" {
		t.Errorf("wf[2].TargetType = %q, want unknown", wf[2].TargetType)
	}
}
