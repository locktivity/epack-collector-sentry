package collector

import (
	"testing"

	"github.com/locktivity/epack-collector-sentry/internal/sentry"
)

func ptrFloat64(v float64) *float64 { return &v }

func TestComputeAlertRuleMetrics_Empty(t *testing.T) {
	m, trunc := computeAlertRuleMetrics(nil, LevelTrust)
	if m.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", m.TotalCount)
	}
	if m.WithActionsPct != nil {
		t.Errorf("WithActionsPct = %v, want nil", m.WithActionsPct)
	}
	if trunc != nil {
		t.Errorf("truncation = %v, want nil", trunc)
	}
}

func TestComputeAlertRuleMetrics_Mixed(t *testing.T) {
	rules := []sentry.AlertRule{
		{
			Type: "alert_rule",
			Triggers: []sentry.AlertTrigger{
				{Label: "critical", Actions: []sentry.AlertAction{{Type: "slack"}}},
			},
		},
		{
			Type: "alert_rule",
			Triggers: []sentry.AlertTrigger{
				{Label: "warning", Actions: nil},
			},
		},
		{
			Type:    "rule",
			Actions: []sentry.AlertAction{{Type: "email"}},
		},
		{
			Type: "rule",
		},
	}

	m, _ := computeAlertRuleMetrics(rules, LevelTrust)

	if m.TotalCount != 4 {
		t.Errorf("TotalCount = %d, want 4", m.TotalCount)
	}
	if m.MetricAlertCount != 2 {
		t.Errorf("MetricAlertCount = %d, want 2", m.MetricAlertCount)
	}
	if m.IssueAlertCount != 2 {
		t.Errorf("IssueAlertCount = %d, want 2", m.IssueAlertCount)
	}
	if m.WithActionsCount != 2 {
		t.Errorf("WithActionsCount = %d, want 2", m.WithActionsCount)
	}
	if m.WithActionsPct == nil || *m.WithActionsPct != 50 {
		t.Errorf("WithActionsPct = %v, want 50", m.WithActionsPct)
	}
	if m.DenominatorAlertRules != 4 {
		t.Errorf("DenominatorAlertRules = %d, want 4", m.DenominatorAlertRules)
	}
}

func TestComputeAlertRuleMetrics_AllWithActions(t *testing.T) {
	rules := []sentry.AlertRule{
		{Type: "alert_rule", Actions: []sentry.AlertAction{{Type: "slack"}}},
		{Type: "rule", Triggers: []sentry.AlertTrigger{{Actions: []sentry.AlertAction{{Type: "email"}}}}},
	}

	m, _ := computeAlertRuleMetrics(rules, LevelTrust)

	if m.WithActionsCount != 2 {
		t.Errorf("WithActionsCount = %d, want 2", m.WithActionsCount)
	}
	if m.WithActionsPct == nil || *m.WithActionsPct != 100 {
		t.Errorf("WithActionsPct = %v, want 100", m.WithActionsPct)
	}
}

func TestComputeAlertRuleMetrics_TrustOmitsInventory(t *testing.T) {
	rules := []sentry.AlertRule{{ID: "1", Type: "rule"}}

	m, _ := computeAlertRuleMetrics(rules, LevelTrust)

	if m.Inventory != nil {
		t.Errorf("expected nil inventory at trust, got %d items", len(m.Inventory))
	}
}

func TestComputeAlertRuleMetrics_AuditIncludesInventory(t *testing.T) {
	env := "production"
	owner := sentry.OwnerField{MonitorOwner: &sentry.MonitorOwner{Type: "team", Name: "backend"}}
	rules := []sentry.AlertRule{
		{
			ID:            "789",
			Name:          "High Error Rate",
			Type:          "alert_rule",
			Aggregate:     "count()",
			TimeWindow:    5,
			ThresholdType: 0,
			DetectionType: "static",
			Projects:      []string{"billing"},
			Environment:   &env,
			Owner:         &owner,
			Triggers: []sentry.AlertTrigger{
				{
					Label:          "critical",
					AlertThreshold: ptrFloat64(100),
					Actions:        []sentry.AlertAction{{Type: "slack", TargetType: "specific"}},
				},
			},
			DateCreated: "2025-11-01T10:00:00Z",
		},
	}

	m, _ := computeAlertRuleMetrics(rules, LevelAudit)

	if len(m.Inventory) != 1 {
		t.Fatalf("expected 1 inventory item, got %d", len(m.Inventory))
	}

	inv := m.Inventory[0]
	if inv.ID != "789" {
		t.Errorf("ID = %q, want %q", inv.ID, "789")
	}
	if inv.Type != "metric" {
		t.Errorf("Type = %q, want %q", inv.Type, "metric")
	}
	if inv.ThresholdType != "above" {
		t.Errorf("ThresholdType = %q, want %q", inv.ThresholdType, "above")
	}
	if inv.Project != "billing" {
		t.Errorf("Project = %q, want %q", inv.Project, "billing")
	}
	if inv.Owner != "team:backend" {
		t.Errorf("Owner = %q, want %q", inv.Owner, "team:backend")
	}
	if len(inv.Triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(inv.Triggers))
	}
	if inv.Triggers[0].Label != "critical" {
		t.Errorf("Trigger label = %q, want %q", inv.Triggers[0].Label, "critical")
	}
	if len(inv.Triggers[0].Actions) != 1 || inv.Triggers[0].Actions[0].Type != "slack" {
		t.Errorf("Trigger actions = %v, want [{slack specific}]", inv.Triggers[0].Actions)
	}
}

func TestComputeAlertRuleMetrics_IssueRuleType(t *testing.T) {
	rules := []sentry.AlertRule{{ID: "1", Type: "rule", Actions: []sentry.AlertAction{{Type: "email"}}}}

	m, _ := computeAlertRuleMetrics(rules, LevelAudit)

	if m.Inventory[0].Type != "issue" {
		t.Errorf("Type = %q, want %q", m.Inventory[0].Type, "issue")
	}
}

func TestHasActions_TopLevelActions(t *testing.T) {
	r := sentry.AlertRule{Actions: []sentry.AlertAction{{Type: "email"}}}
	if !hasActions(r) {
		t.Error("expected hasActions = true for top-level actions")
	}
}

func TestHasActions_TriggerActions(t *testing.T) {
	r := sentry.AlertRule{
		Triggers: []sentry.AlertTrigger{
			{Actions: nil},
			{Actions: []sentry.AlertAction{{Type: "slack"}}},
		},
	}
	if !hasActions(r) {
		t.Error("expected hasActions = true for trigger actions")
	}
}

func TestBuildAlertRuleInventory_InternalIncludesIntegrationID(t *testing.T) {
	r := sentry.AlertRule{
		ID: "1", Type: "alert_rule", Name: "Test",
		Triggers: []sentry.AlertTrigger{
			{
				Label:          "critical",
				AlertThreshold: ptrFloat64(100),
				Actions: []sentry.AlertAction{
					{Type: "slack", TargetType: "specific", IntegrationID: float64(12345)},
					{Type: "email", TargetType: "user", IntegrationID: nil},
				},
			},
		},
	}

	inv := buildAlertRuleInventory(r, LevelInternal)
	if len(inv.Triggers) != 1 || len(inv.Triggers[0].Actions) != 2 {
		t.Fatalf("expected 1 trigger with 2 actions, got %v", inv.Triggers)
	}
	if inv.Triggers[0].Actions[0].IntegrationID != "12345" {
		t.Errorf("IntegrationID = %q, want %q", inv.Triggers[0].Actions[0].IntegrationID, "12345")
	}
	if inv.Triggers[0].Actions[1].IntegrationID != "" {
		t.Errorf("IntegrationID = %q, want empty", inv.Triggers[0].Actions[1].IntegrationID)
	}
}

func TestBuildAlertRuleInventory_AuditOmitsIntegrationID(t *testing.T) {
	r := sentry.AlertRule{
		ID: "1", Type: "alert_rule", Name: "Test",
		Triggers: []sentry.AlertTrigger{
			{
				Label: "critical",
				Actions: []sentry.AlertAction{
					{Type: "slack", TargetType: "specific", IntegrationID: float64(12345)},
				},
			},
		},
	}

	inv := buildAlertRuleInventory(r, LevelAudit)
	if inv.Triggers[0].Actions[0].IntegrationID != "" {
		t.Errorf("IntegrationID = %q, want empty at audit level", inv.Triggers[0].Actions[0].IntegrationID)
	}
}

func TestHasActions_NoActions(t *testing.T) {
	r := sentry.AlertRule{
		Triggers: []sentry.AlertTrigger{{Actions: nil}},
	}
	if hasActions(r) {
		t.Error("expected hasActions = false when no actions anywhere")
	}
}
