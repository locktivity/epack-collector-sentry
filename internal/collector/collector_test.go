package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/locktivity/epack-collector-sentry/internal/sentry"
)

type FixedClock struct {
	Time time.Time
}

func (c FixedClock) Now() time.Time { return c.Time }

type fakeSentryAPI struct {
	monitors       []sentry.Monitor
	monitorsErr    error
	alertRules     []sentry.AlertRule
	alertRulesErr  error
	members        []sentry.Member
	membersErr     error
	teams          []sentry.Team
	teamsErr       error
	teamMembers    map[string][]sentry.TeamMember
	teamMembersErr error
}

func (f *fakeSentryAPI) ListMonitors(_ context.Context, _ []string, _ []string) ([]sentry.Monitor, error) {
	if f.monitorsErr != nil {
		return nil, f.monitorsErr
	}
	return f.monitors, nil
}

func (f *fakeSentryAPI) ListAlertRules(_ context.Context) ([]sentry.AlertRule, error) {
	if f.alertRulesErr != nil {
		return nil, f.alertRulesErr
	}
	return f.alertRules, nil
}

func (f *fakeSentryAPI) ListMembers(_ context.Context) ([]sentry.Member, error) {
	if f.membersErr != nil {
		return nil, f.membersErr
	}
	return f.members, nil
}

func (f *fakeSentryAPI) ListTeams(_ context.Context) ([]sentry.Team, error) {
	if f.teamsErr != nil {
		return nil, f.teamsErr
	}
	return f.teams, nil
}

func (f *fakeSentryAPI) ListTeamMembers(_ context.Context, teamSlug string) ([]sentry.TeamMember, error) {
	if f.teamMembersErr != nil {
		return nil, f.teamMembersErr
	}
	if f.teamMembers != nil {
		return f.teamMembers[teamSlug], nil
	}
	return nil, nil
}

var goldenTime = time.Date(2026, 6, 26, 14, 0, 0, 0, time.UTC)

func richFake() *fakeSentryAPI {
	env := "production"
	owner := "team:backend"
	return &fakeSentryAPI{
		monitors: []sentry.Monitor{
			{
				ID: "mon1", Name: "daily-sync", Status: "active", Type: "cron_job",
				Config:  sentry.MonitorConfig{ScheduleType: "crontab", Schedule: "0 2 * * *"},
				Project: sentry.MonitorProject{Slug: "billing"},
				Owner:   &sentry.MonitorOwner{Type: "team", Name: "backend"},
				AlertRule: &sentry.MonitorAlert{
					Targets: []sentry.AlertTarget{{TargetType: 2, TargetIdentifier: 42}},
				},
				Environments: []sentry.MonitorEnv{
					{Name: "production", Status: "ok"},
				},
			},
			{
				ID: "mon2", Name: "weekly-report", Status: "disabled", Type: "cron_job",
				Config:  sentry.MonitorConfig{ScheduleType: "interval", Schedule: []any{7, "day"}},
				Project: sentry.MonitorProject{Slug: "reporting"},
			},
		},
		alertRules: []sentry.AlertRule{
			{
				ID: "ar1", Name: "High Error Rate", Type: "alert_rule",
				Aggregate: "count()", TimeWindow: 5, ThresholdType: 0,
				Projects: []string{"billing"}, Environment: &env, Owner: &owner,
				Triggers: []sentry.AlertTrigger{
					{Label: "critical", AlertThreshold: 100, Actions: []sentry.AlertAction{
						{Type: "slack", TargetType: "specific", IntegrationID: float64(123)},
					}},
				},
				DateCreated: "2025-11-01T10:00:00Z",
			},
			{
				ID: "ar2", Name: "New Issue", Type: "rule",
				Actions: []sentry.AlertAction{{Type: "email", TargetType: "user"}},
			},
		},
		members: []sentry.Member{
			{
				ID: "u1", Email: "admin@acme.com", Name: "Admin", OrgRole: "admin",
				User:  &sentry.MemberUser{IsActive: true, Has2fa: true},
				Flags: sentry.MemberFlags{SSOLinked: true},
			},
			{
				ID: "u2", Email: "dev@acme.com", Name: "Dev", OrgRole: "member",
				User:  &sentry.MemberUser{IsActive: true, Has2fa: false},
				Flags: sentry.MemberFlags{SSOLinked: false},
			},
		},
		teams: []sentry.Team{
			{ID: "t1", Slug: "backend", Name: "Backend", MemberCount: 2, ProjectCount: 3},
		},
		teamMembers: map[string][]sentry.TeamMember{
			"backend": {
				{ID: "u1", Email: "admin@acme.com", Name: "Admin", TeamRole: "admin"},
				{ID: "u2", Email: "dev@acme.com", Name: "Dev", TeamRole: "contributor"},
			},
		},
	}
}

func testCollector(fake *fakeSentryAPI, level Level) *Collector {
	c := New(Config{Organization: "acme-corp"}, fake, level)
	c.clock = FixedClock{Time: goldenTime}
	return c
}

func TestCollect_TrustLevel(t *testing.T) {
	c := testCollector(richFake(), LevelTrust)
	out, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if out.SchemaVersion != "1.0.0" {
		t.Errorf("SchemaVersion = %q, want %q", out.SchemaVersion, "1.0.0")
	}
	if out.CollectedAt != "2026-06-26T14:00:00Z" {
		t.Errorf("CollectedAt = %q, want deterministic time", out.CollectedAt)
	}
	if out.CollectedAtLevel != "trust" {
		t.Errorf("CollectedAtLevel = %q, want %q", out.CollectedAtLevel, "trust")
	}
	if out.Organization != "acme-corp" {
		t.Errorf("Organization = %q, want %q", out.Organization, "acme-corp")
	}
	if !out.MonitoringSummary.AlertsOrMonitorsEnabled {
		t.Error("AlertsOrMonitorsEnabled = false, want true")
	}
	if out.MonitoringSummary.TotalMonitoringRules != nil {
		t.Errorf("TotalMonitoringRules should be nil at trust, got %v", out.MonitoringSummary.TotalMonitoringRules)
	}
	if out.Monitors != nil {
		t.Error("Monitors should be nil at trust")
	}
	if out.AlertRules != nil {
		t.Error("AlertRules should be nil at trust")
	}
	if out.Users != nil {
		t.Error("Users should be nil at trust")
	}
	if out.Teams != nil {
		t.Error("Teams should be nil at trust")
	}
}

func TestCollect_AuditLevel(t *testing.T) {
	c := testCollector(richFake(), LevelAudit)
	out, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if !out.MonitoringSummary.AlertsOrMonitorsEnabled {
		t.Error("AlertsOrMonitorsEnabled = false, want true")
	}
	if out.MonitoringSummary.TotalMonitoringRules == nil || *out.MonitoringSummary.TotalMonitoringRules != 4 {
		t.Errorf("TotalMonitoringRules = %v, want 4", out.MonitoringSummary.TotalMonitoringRules)
	}

	if out.Monitors == nil {
		t.Fatal("Monitors should be populated at audit")
	}
	if out.Monitors.TotalCount != 2 {
		t.Errorf("Monitors.TotalCount = %d, want 2", out.Monitors.TotalCount)
	}
	if len(out.Monitors.Inventory) != 2 {
		t.Errorf("Monitors.Inventory len = %d, want 2", len(out.Monitors.Inventory))
	}

	if out.AlertRules == nil {
		t.Fatal("AlertRules should be populated at audit")
	}
	if out.AlertRules.TotalCount != 2 {
		t.Errorf("AlertRules.TotalCount = %d, want 2", out.AlertRules.TotalCount)
	}

	if out.Users == nil {
		t.Fatal("Users should be populated at audit")
	}
	if out.Users.TotalCount != 2 {
		t.Errorf("Users.TotalCount = %d, want 2", out.Users.TotalCount)
	}
	if out.Users.Inventory[0].TeamMemberships != nil {
		t.Error("TeamMemberships should be nil at audit level")
	}

	if out.Teams == nil {
		t.Fatal("Teams should be populated at audit")
	}
	if out.Teams.TotalCount != 1 {
		t.Errorf("Teams.TotalCount = %d, want 1", out.Teams.TotalCount)
	}
	if out.Teams.Inventory[0].Members != nil {
		t.Error("Team Members should be nil at audit level")
	}
}

func TestCollect_InternalLevel(t *testing.T) {
	c := testCollector(richFake(), LevelInternal)
	out, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if out.Users.Inventory[0].TeamMemberships == nil {
		t.Fatal("TeamMemberships should be populated at internal")
	}
	if len(out.Users.Inventory[0].TeamMemberships) != 1 {
		t.Errorf("TeamMemberships len = %d, want 1", len(out.Users.Inventory[0].TeamMemberships))
	}
	if out.Users.Inventory[0].TeamMemberships[0].TeamSlug != "backend" {
		t.Errorf("TeamSlug = %q, want %q", out.Users.Inventory[0].TeamMemberships[0].TeamSlug, "backend")
	}

	if out.Teams.Inventory[0].Members == nil {
		t.Fatal("Team Members should be populated at internal")
	}
	if len(out.Teams.Inventory[0].Members) != 2 {
		t.Errorf("Team Members len = %d, want 2", len(out.Teams.Inventory[0].Members))
	}

	if out.Monitors.Inventory[0].OpenAlertCount == nil {
		t.Error("OpenAlertCount should be populated at internal")
	}

	if out.AlertRules.Inventory[0].Triggers[0].Actions[0].IntegrationID != "123" {
		t.Errorf("IntegrationID = %q, want %q", out.AlertRules.Inventory[0].Triggers[0].Actions[0].IntegrationID, "123")
	}
}

func TestCollect_EmptyOrg(t *testing.T) {
	fake := &fakeSentryAPI{}
	c := testCollector(fake, LevelAudit)
	out, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if out.MonitoringSummary.AlertsOrMonitorsEnabled {
		t.Error("AlertsOrMonitorsEnabled = true, want false for empty org")
	}
	if out.Monitors.TotalCount != 0 {
		t.Errorf("Monitors.TotalCount = %d, want 0", out.Monitors.TotalCount)
	}
	if out.AlertRules.TotalCount != 0 {
		t.Errorf("AlertRules.TotalCount = %d, want 0", out.AlertRules.TotalCount)
	}
}

func TestCollect_MemberPermissionDenied(t *testing.T) {
	fake := richFake()
	fake.membersErr = &sentry.APIError{StatusCode: http.StatusForbidden, Body: "forbidden"}
	c := testCollector(fake, LevelAudit)
	out, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if out.Users != nil {
		t.Error("Users should be nil when member:read is denied")
	}
	if len(out.Diagnostics.Warnings) == 0 {
		t.Error("expected a warning about missing member:read scope")
	}
}

func TestCollect_TeamPermissionDenied(t *testing.T) {
	fake := richFake()
	fake.teamsErr = &sentry.APIError{StatusCode: http.StatusForbidden, Body: "forbidden"}
	c := testCollector(fake, LevelAudit)
	out, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if out.Teams != nil {
		t.Error("Teams should be nil when team:read is denied")
	}
	if len(out.Diagnostics.Warnings) == 0 {
		t.Error("expected a warning about missing team:read scope")
	}
}

func TestCollect_MonitorAPIError(t *testing.T) {
	fake := richFake()
	fake.monitorsErr = &sentry.APIError{StatusCode: http.StatusUnauthorized, Body: "bad token"}
	c := testCollector(fake, LevelTrust)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error for unauthorized monitors call")
	}
}

func TestCollect_AlertRuleAPIError(t *testing.T) {
	fake := richFake()
	fake.alertRulesErr = &sentry.APIError{StatusCode: 500, Body: "server error"}
	c := testCollector(fake, LevelTrust)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error for failed alert rules call")
	}
}

func TestCollect_OutputIsValidJSON(t *testing.T) {
	c := testCollector(richFake(), LevelInternal)
	out, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("failed to marshal output: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	for _, field := range []string{"schema_version", "collected_at", "collected_at_level", "organization", "monitoring_summary", "diagnostics"} {
		if _, ok := decoded[field]; !ok {
			t.Errorf("missing required top-level field %q", field)
		}
	}
}

func TestCollect_TeamMemberPermissionDenied(t *testing.T) {
	fake := richFake()
	fake.teamMembersErr = &sentry.APIError{StatusCode: http.StatusForbidden, Body: "forbidden"}
	c := testCollector(fake, LevelInternal)
	out, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if out.Users.Inventory[0].TeamMemberships != nil {
		t.Error("TeamMemberships should be nil when team member detail is denied")
	}
	if out.Teams.Inventory[0].Members != nil {
		t.Error("Team Members should be nil when team member detail is denied")
	}
	found := false
	for _, w := range out.Diagnostics.Warnings {
		if w == "team:read scope missing; team member details unavailable" {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about team member detail unavailable")
	}
}
