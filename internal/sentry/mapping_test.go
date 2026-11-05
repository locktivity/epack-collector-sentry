package sentry

import (
	"encoding/json"
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
						TimeWindow:  300,
						Environment: &env,
					},
				},
			}},
			ConditionGroup: &DetectorConditionGroup{
				Conditions: []DetectorCondition{{
					Comparison: json.RawMessage(`{"thresholdType":0}`),
				}},
			},
			Owner:       &OwnerField{MonitorOwner: &MonitorOwner{Type: "team", Name: "backend"}},
			WorkflowIds: []string{"w1"},
		},
	}

	workflows := []Workflow{{
		ID:   "w1",
		Name: "Notify Slack",
		ActionFilters: []WorkflowActionFilter{{
			Actions: []WorkflowAction{
				{Type: "slack", IntegrationID: &FlexibleID{Value: "123"}},
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
		t.Errorf("TimeWindow = %v, want 5 minutes for a timeWindow of 300 seconds", r.TimeWindow)
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

// Every project has an error monitor and an issue stream monitor by default.
// They are alert rules only when an alert is connected to them. An uptime
// monitor is not an alert rule either.
func TestMapDetectorsToAlertRules_SkipsMonitorsThatAreNotAlertRules(t *testing.T) {
	detectors := []Detector{
		{ID: "d3", Name: "Error Monitor", Type: "error", ProjectID: "11", Enabled: true, WorkflowIds: []string{}},
		{ID: "d4", Name: "Issue Stream Monitor", Type: "issue_stream", ProjectID: "11", Enabled: true},
		{ID: "d5", Name: "[us] Example", Type: "uptime_domain_failure", ProjectID: "11", Enabled: true},
	}

	if rules := mapDetectorsToAlertRules(detectors, nil, nil); len(rules) != 0 {
		t.Errorf("got %d rules, want 0 (%+v)", len(rules), rules)
	}
}

func TestMapDetectorsToAlertRules_ProjectFallbackToID(t *testing.T) {
	detectors := []Detector{{
		ID: "d4", Name: "Test", Type: "metric_issue", ProjectID: "999", Enabled: true,
	}}
	projects := []Project{{ID: "1", Slug: "other"}}

	rules := mapDetectorsToAlertRules(detectors, nil, projects)
	if rules[0].Projects[0] != "999" {
		t.Errorf("expected project ID fallback, got %q", rules[0].Projects[0])
	}
}

func TestFlexibleID_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"string", `"456"`, "456"},
		{"number", `789`, "789"},
		{"float", `12.5`, "12.5"},
		{"null", `null`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var id FlexibleID
			if err := id.UnmarshalJSON([]byte(tt.input)); err != nil {
				t.Fatalf("UnmarshalJSON(%s) error: %v", tt.input, err)
			}
			if id.Value != tt.want {
				t.Errorf("Value = %q, want %q", id.Value, tt.want)
			}
		})
	}
}

func TestMapDetectorsToAlertRules_ComparisonAsString(t *testing.T) {
	detectors := []Detector{{
		ID: "d5", Name: "String Comparison", Type: "metric_issue", Enabled: true,
		Config: DetectorConfig{DetectionType: "static"},
		DataSources: []DetectorDataSource{{
			QueryObj: &DetectorQueryObj{
				SnubaQuery: &DetectorSnubaQuery{Aggregate: "count()", TimeWindow: 10},
			},
		}},
		ConditionGroup: &DetectorConditionGroup{
			Conditions: []DetectorCondition{{
				Comparison: json.RawMessage(`"some_string_value"`),
			}},
		},
	}}

	rules := mapDetectorsToAlertRules(detectors, nil, nil)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].ThresholdType != 0 {
		t.Errorf("ThresholdType = %d, want 0 (default when comparison is a string)", rules[0].ThresholdType)
	}
}

func TestAlertRuleType(t *testing.T) {
	tests := []struct {
		detectorType string
		workflowIds  []string
		want         string
		wantOK       bool
	}{
		{"metric_issue", nil, "alert_rule", true},
		{"metric_issue", []string{"w1"}, "alert_rule", true},
		{"error", []string{"w1"}, "rule", true},
		{"issue_stream", []string{"w1"}, "rule", true},
		{"error", nil, "", false},
		{"issue_stream", nil, "", false},
		{"uptime_domain_failure", []string{"w1"}, "", false},
		{"unknown", []string{"w1"}, "", false},
	}
	for _, tt := range tests {
		got, ok := alertRuleType(Detector{Type: tt.detectorType, WorkflowIds: tt.workflowIds})
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("alertRuleType(%s with workflows %v) = %q, %v, want %q, %v", tt.detectorType, tt.workflowIds, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestMapDetectorsToAlertRules_ThresholdDirection(t *testing.T) {
	tests := []struct {
		name       string
		conditions []DetectorCondition
		want       int
	}{
		{"static above", []DetectorCondition{
			{Type: "gt", Comparison: json.RawMessage(`1600`), ConditionResult: float64(75)},
			{Type: "lte", Comparison: json.RawMessage(`1600`), ConditionResult: float64(0)},
		}, 0},
		{"static below", []DetectorCondition{
			{Type: "lt", Comparison: json.RawMessage(`50`), ConditionResult: float64(75)},
			{Type: "gte", Comparison: json.RawMessage(`50`), ConditionResult: float64(0)},
		}, 1},
		{"static below or equal", []DetectorCondition{
			{Type: "lte", Comparison: json.RawMessage(`50`), ConditionResult: float64(50)},
		}, 1},
		{"resolve condition listed first", []DetectorCondition{
			{Type: "gte", Comparison: json.RawMessage(`50`), ConditionResult: float64(0)},
			{Type: "lt", Comparison: json.RawMessage(`50`), ConditionResult: float64(75)},
		}, 1},
		{"dynamic below", []DetectorCondition{
			{Type: "anomaly_detection", Comparison: json.RawMessage(`{"seasonality":"auto","sensitivity":"low","thresholdType":1}`), ConditionResult: float64(75)},
		}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detectors := []Detector{{
				ID: "d6", Name: "Latency", Type: "metric_issue", Enabled: true,
				ConditionGroup: &DetectorConditionGroup{LogicType: "any", Conditions: tt.conditions},
			}}
			rules := mapDetectorsToAlertRules(detectors, nil, nil)
			if len(rules) != 1 {
				t.Fatalf("got %d rules, want 1", len(rules))
			}
			if rules[0].ThresholdType != tt.want {
				t.Errorf("ThresholdType = %d, want %d", rules[0].ThresholdType, tt.want)
			}
		})
	}
}

func TestMapWorkflowActionToAlertAction_TargetType(t *testing.T) {
	tests := []struct {
		actionType string
		config     string
		want       string
	}{
		{"email", `{"targetType":"team","targetDisplay":null,"targetIdentifier":"42"}`, "team"},
		{"email", `{"targetType":"user","targetDisplay":null,"targetIdentifier":"9001"}`, "user"},
		{"email", `{"targetType":"issue_owners","targetDisplay":null,"targetIdentifier":null}`, "issue_owners"},
		{"slack", `{"targetType":"specific","targetDisplay":"#alerts","targetIdentifier":"C0123"}`, "specific"},
		{"email", ``, ""},
	}
	for _, tt := range tests {
		got := mapWorkflowActionToAlertAction(WorkflowAction{Type: tt.actionType, Config: json.RawMessage(tt.config)})
		if got.Type != tt.actionType || got.TargetType != tt.want {
			t.Errorf("action with config %s = %+v, want type %q and target type %q", tt.config, got, tt.actionType, tt.want)
		}
	}
}

func TestMapDetectorsToAlertRules_IssueAlertEnvironment(t *testing.T) {
	production, staging := "production", "staging"
	detectors := []Detector{
		{ID: "d7", Name: "Issue Stream Monitor", Type: "issue_stream", ProjectID: "11", Enabled: true, WorkflowIds: []string{"w1"}},
		{ID: "d8", Name: "Error Monitor", Type: "error", ProjectID: "11", Enabled: true, WorkflowIds: []string{"w1", "w2"}},
	}
	workflows := []Workflow{
		{ID: "w1", Name: "Page on-call", Enabled: true, Environment: &production},
		{ID: "w2", Name: "Email staging", Enabled: true, Environment: &staging},
	}

	rules := mapDetectorsToAlertRules(detectors, workflows, nil)
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(rules))
	}
	if rules[0].Environment == nil || *rules[0].Environment != "production" {
		t.Errorf("Environment = %v, want production from the connected alert", rules[0].Environment)
	}
	if rules[1].Environment != nil {
		t.Errorf("Environment = %q, want nil when the connected alerts name different environments", *rules[1].Environment)
	}
}
