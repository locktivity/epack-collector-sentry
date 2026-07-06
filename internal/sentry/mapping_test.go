package sentry

import (
	"testing"
)

func TestMapDetectorsToAlertRules_Empty(t *testing.T) {
	rules := mapDetectorsToAlertRules(nil, nil, nil)
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}
}

func TestMapDetectorsToAlertRules_MetricAlert(t *testing.T) {
	env := "production"
	detectors := []Detector{
		{
			ID: "d1", Name: "High Error Rate", Type: "metric_issue",
			ProjectID: "42", Enabled: true,
			Config: DetectorConfig{DetectionType: "static"},
			DataSources: []DetectorDataSource{{
				QueryObj: &DetectorQueryObj{
					SnubaQuery: &DetectorSnubaQuery{
						Aggregate:   "count()",
						TimeWindow:  5,
						Environment: &env,
					},
				},
			}},
			ConditionGroup: &DetectorConditionGroup{
				Conditions: []DetectorCondition{{
					Comparison: DetectorConditionComparison{ThresholdType: 0},
				}},
			},
			Owner:       &OwnerField{MonitorOwner: &MonitorOwner{Type: "team", Name: "backend"}},
			WorkflowIds: []string{"w1"},
		},
	}

	intID := "123"
	workflows := []Workflow{{
		ID:   "w1",
		Name: "Notify Slack",
		ActionFilters: []WorkflowActionFilter{{
			Actions: []WorkflowAction{
				{Type: "slack", IntegrationID: &intID},
			},
		}},
	}}

	projects := []Project{{ID: "42", Slug: "billing", Name: "Billing"}}

	rules := mapDetectorsToAlertRules(detectors, workflows, projects)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	r := rules[0]
	if r.ID != "d1" {
		t.Errorf("ID = %q, want %q", r.ID, "d1")
	}
	if r.Type != "alert_rule" {
		t.Errorf("Type = %q, want %q", r.Type, "alert_rule")
	}
	if r.Status != "active" {
		t.Errorf("Status = %v, want %q", r.Status, "active")
	}
	if r.Aggregate != "count()" {
		t.Errorf("Aggregate = %q, want %q", r.Aggregate, "count()")
	}
	if r.TimeWindow != 5 {
		t.Errorf("TimeWindow = %v, want 5", r.TimeWindow)
	}
	if r.ThresholdType != 0 {
		t.Errorf("ThresholdType = %d, want 0", r.ThresholdType)
	}
	if r.DetectionType != "static" {
		t.Errorf("DetectionType = %q, want %q", r.DetectionType, "static")
	}
	if len(r.Projects) != 1 || r.Projects[0] != "billing" {
		t.Errorf("Projects = %v, want [billing]", r.Projects)
	}
	if r.Environment == nil || *r.Environment != "production" {
		t.Errorf("Environment = %v, want production", r.Environment)
	}
	if r.Owner == nil || r.Owner.Type != "team" || r.Owner.Name != "backend" {
		t.Errorf("Owner = %v, want team:backend", r.Owner)
	}

	if len(r.Triggers) != 1 {
		t.Fatalf("Triggers len = %d, want 1", len(r.Triggers))
	}
	if r.Triggers[0].Label != "Notify Slack" {
		t.Errorf("Trigger label = %q, want %q", r.Triggers[0].Label, "Notify Slack")
	}
	if len(r.Triggers[0].Actions) != 1 || r.Triggers[0].Actions[0].Type != "slack" {
		t.Errorf("Trigger actions = %v, want [{slack}]", r.Triggers[0].Actions)
	}
	if r.Triggers[0].Actions[0].IntegrationID != "123" {
		t.Errorf("IntegrationID = %v, want %q", r.Triggers[0].Actions[0].IntegrationID, "123")
	}
}

func TestMapDetectorsToAlertRules_IssueAlert(t *testing.T) {
	detectors := []Detector{
		{
			ID: "d2", Name: "New Issue", Type: "error",
			ProjectID: "10", Enabled: false,
			WorkflowIds: []string{"w2"},
		},
	}

	workflows := []Workflow{{
		ID:   "w2",
		Name: "Email Team",
		Triggers: &WorkflowTriggers{
			Actions: []WorkflowAction{
				{Type: "email"},
			},
		},
	}}

	rules := mapDetectorsToAlertRules(detectors, workflows, nil)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	r := rules[0]
	if r.Type != "rule" {
		t.Errorf("Type = %q, want %q", r.Type, "rule")
	}
	if r.Status != "disabled" {
		t.Errorf("Status = %v, want %q", r.Status, "disabled")
	}
	if len(r.Actions) != 1 || r.Actions[0].Type != "email" {
		t.Errorf("Actions = %v, want [{email}]", r.Actions)
	}
	if r.Projects[0] != "10" {
		t.Errorf("Projects = %v, want [10] (fallback to ID)", r.Projects)
	}
}

func TestMapDetectorsToAlertRules_NoWorkflows(t *testing.T) {
	detectors := []Detector{{
		ID: "d3", Name: "Orphan", Type: "issue_stream", Enabled: true,
	}}

	rules := mapDetectorsToAlertRules(detectors, nil, nil)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Type != "rule" {
		t.Errorf("Type = %q, want %q", rules[0].Type, "rule")
	}
	if len(rules[0].Triggers) != 0 && len(rules[0].Actions) != 0 {
		t.Error("expected no triggers/actions for orphan detector")
	}
}

func TestMapDetectorsToAlertRules_ProjectFallbackToID(t *testing.T) {
	detectors := []Detector{{
		ID: "d4", Name: "Test", Type: "error", ProjectID: "999", Enabled: true,
	}}
	projects := []Project{{ID: "1", Slug: "other"}}

	rules := mapDetectorsToAlertRules(detectors, nil, projects)
	if rules[0].Projects[0] != "999" {
		t.Errorf("expected project ID fallback, got %q", rules[0].Projects[0])
	}
}

func TestMapDetectorType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"metric_issue", "alert_rule"},
		{"error", "rule"},
		{"issue_stream", "rule"},
		{"unknown", "rule"},
	}
	for _, tt := range tests {
		got := mapDetectorType(tt.input)
		if got != tt.want {
			t.Errorf("mapDetectorType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
