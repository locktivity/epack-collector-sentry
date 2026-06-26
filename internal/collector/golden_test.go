package collector

import (
	"context"
	"encoding/json"
	"testing"
)

func collectGolden(t *testing.T, level Level) map[string]any {
	t.Helper()
	c := testCollector(richFake(), level)
	out, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	return m
}

func requireField(t *testing.T, m map[string]any, key string) any {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("missing required field %q", key)
	}
	return v
}

func requireAbsent(t *testing.T, m map[string]any, key string) {
	t.Helper()
	if _, ok := m[key]; ok {
		t.Errorf("field %q should be absent", key)
	}
}

func TestGolden_TrustLevel(t *testing.T) {
	m := collectGolden(t, LevelTrust)

	if m["schema_version"] != "1.0.0" {
		t.Errorf("schema_version = %v", m["schema_version"])
	}
	requireField(t, m, "collected_at")
	if m["collected_at_level"] != "trust" {
		t.Errorf("collected_at_level = %v", m["collected_at_level"])
	}
	requireField(t, m, "organization")
	requireField(t, m, "diagnostics")

	summary, ok := m["monitoring_summary"].(map[string]any)
	if !ok {
		t.Fatal("monitoring_summary missing or wrong type")
	}
	if _, ok := summary["alerts_or_monitors_enabled"]; !ok {
		t.Error("monitoring_summary missing alerts_or_monitors_enabled")
	}

	// Trust must NOT include counts or inventories
	requireAbsent(t, summary, "total_monitoring_rules")
	requireAbsent(t, summary, "cron_monitors")
	requireAbsent(t, summary, "alert_rules")
	requireAbsent(t, summary, "with_actions_pct")
	requireAbsent(t, m, "monitors")
	requireAbsent(t, m, "alert_rules")
	requireAbsent(t, m, "users")
	requireAbsent(t, m, "teams")

	// Diagnostics structure
	diag := m["diagnostics"].(map[string]any)
	requireField(t, diag, "errors")
	requireField(t, diag, "warnings")
	requireField(t, diag, "truncation")
}

func TestGolden_AuditLevel(t *testing.T) {
	m := collectGolden(t, LevelAudit)

	if m["collected_at_level"] != "audit" {
		t.Errorf("collected_at_level = %v", m["collected_at_level"])
	}

	// Monitoring summary has counts at audit
	summary := m["monitoring_summary"].(map[string]any)
	requireField(t, summary, "alerts_or_monitors_enabled")
	requireField(t, summary, "total_monitoring_rules")
	requireField(t, summary, "cron_monitors")
	requireField(t, summary, "alert_rules")
	requireField(t, summary, "with_actions_pct")

	// Monitors section present with inventory
	monitors := requireField(t, m, "monitors").(map[string]any)
	requireField(t, monitors, "total_count")
	requireField(t, monitors, "active_count")
	requireField(t, monitors, "inventory")
	inv := monitors["inventory"].([]any)
	if len(inv) == 0 {
		t.Fatal("monitors inventory should not be empty")
	}
	firstMon := inv[0].(map[string]any)
	requireField(t, firstMon, "id")
	requireField(t, firstMon, "name")
	requireField(t, firstMon, "status")
	requireField(t, firstMon, "connected_workflows")
	requireAbsent(t, firstMon, "open_alert_count") // internal only

	// Alert rules section present with inventory
	alertRules := requireField(t, m, "alert_rules").(map[string]any)
	requireField(t, alertRules, "total_count")
	requireField(t, alertRules, "inventory")
	arInv := alertRules["inventory"].([]any)
	if len(arInv) > 0 {
		firstAR := arInv[0].(map[string]any)
		requireField(t, firstAR, "id")
		requireField(t, firstAR, "name")
		requireField(t, firstAR, "type")
		if triggers, ok := firstAR["triggers"].([]any); ok && len(triggers) > 0 {
			trig := triggers[0].(map[string]any)
			actions := trig["actions"].([]any)
			if len(actions) > 0 {
				requireAbsent(t, actions[0].(map[string]any), "integration_id") // internal only
			}
		}
	}

	// Users and teams present at audit
	users := requireField(t, m, "users").(map[string]any)
	requireField(t, users, "total_count")
	requireField(t, users, "inventory")
	userInv := users["inventory"].([]any)
	if len(userInv) > 0 {
		requireAbsent(t, userInv[0].(map[string]any), "team_memberships") // internal only
	}

	teams := requireField(t, m, "teams").(map[string]any)
	requireField(t, teams, "total_count")
	requireField(t, teams, "inventory")
	teamInv := teams["inventory"].([]any)
	if len(teamInv) > 0 {
		requireAbsent(t, teamInv[0].(map[string]any), "members") // internal only
	}
}

func TestGolden_InternalLevel(t *testing.T) {
	m := collectGolden(t, LevelInternal)

	if m["collected_at_level"] != "internal" {
		t.Errorf("collected_at_level = %v", m["collected_at_level"])
	}

	// Internal is a superset of audit - monitors have open_alert_count
	monitors := m["monitors"].(map[string]any)
	inv := monitors["inventory"].([]any)
	firstMon := inv[0].(map[string]any)
	requireField(t, firstMon, "open_alert_count")

	// Alert rules have integration_id at internal
	alertRules := m["alert_rules"].(map[string]any)
	arInv := alertRules["inventory"].([]any)
	firstAR := arInv[0].(map[string]any)
	if triggers, ok := firstAR["triggers"].([]any); ok && len(triggers) > 0 {
		trig := triggers[0].(map[string]any)
		actions := trig["actions"].([]any)
		if len(actions) > 0 {
			requireField(t, actions[0].(map[string]any), "integration_id")
		}
	}

	// Users have team_memberships at internal
	users := m["users"].(map[string]any)
	userInv := users["inventory"].([]any)
	firstUser := userInv[0].(map[string]any)
	requireField(t, firstUser, "team_memberships")
	memberships := firstUser["team_memberships"].([]any)
	if len(memberships) == 0 {
		t.Error("team_memberships should not be empty at internal")
	}

	// Teams have members at internal
	teams := m["teams"].(map[string]any)
	teamInv := teams["inventory"].([]any)
	firstTeam := teamInv[0].(map[string]any)
	requireField(t, firstTeam, "members")
	members := firstTeam["members"].([]any)
	if len(members) == 0 {
		t.Error("team members should not be empty at internal")
	}
	firstMember := members[0].(map[string]any)
	requireField(t, firstMember, "user_id")
	requireField(t, firstMember, "email")
	requireField(t, firstMember, "team_role")
}

func TestGolden_NoExtraTopLevelFields(t *testing.T) {
	allowed := map[string]bool{
		"schema_version":     true,
		"collected_at":       true,
		"collected_at_level": true,
		"organization":       true,
		"monitoring_summary": true,
		"monitors":           true,
		"alert_rules":        true,
		"users":              true,
		"teams":              true,
		"diagnostics":        true,
	}

	for _, level := range []Level{LevelTrust, LevelAudit, LevelInternal} {
		m := collectGolden(t, level)
		for k := range m {
			if !allowed[k] {
				t.Errorf("unexpected top-level field %q at %s level", k, level)
			}
		}
	}
}

func TestGolden_EmptyOrg_Trust(t *testing.T) {
	c := testCollector(&fakeSentryAPI{}, LevelTrust)
	out, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	data, _ := json.Marshal(out)
	var m map[string]any
	json.Unmarshal(data, &m)

	summary := m["monitoring_summary"].(map[string]any)
	if summary["alerts_or_monitors_enabled"] != false {
		t.Error("empty org trust should have alerts_or_monitors_enabled=false")
	}
	requireAbsent(t, m, "monitors")
	requireAbsent(t, m, "alert_rules")
}
