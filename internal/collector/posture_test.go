package collector

import (
	"testing"

	"github.com/locktivity/epack-collector-sentry/internal/sentry"
)

func TestComputeMonitoringSummary_TrustOnlyEnabled(t *testing.T) {
	monitors := []sentry.Monitor{{ID: "1", Status: "active"}}
	alertRules := []sentry.AlertRule{{ID: "1", Type: "rule"}}

	s := computeMonitoringSummary(monitors, alertRules, LevelTrust)

	if !s.AlertsOrMonitorsEnabled {
		t.Error("AlertsOrMonitorsEnabled = false, want true")
	}
	if s.TotalMonitoringRules != nil {
		t.Errorf("TotalMonitoringRules = %v, want nil at trust", s.TotalMonitoringRules)
	}
	if s.CronMonitors != nil {
		t.Errorf("CronMonitors = %v, want nil at trust", s.CronMonitors)
	}
	if s.AlertRules != nil {
		t.Errorf("AlertRules = %v, want nil at trust", s.AlertRules)
	}
	if s.WithActionsPct != nil {
		t.Errorf("WithActionsPct = %v, want nil at trust", s.WithActionsPct)
	}
}

func TestComputeMonitoringSummary_TrustDisabledWhenEmpty(t *testing.T) {
	s := computeMonitoringSummary(nil, nil, LevelTrust)

	if s.AlertsOrMonitorsEnabled {
		t.Error("AlertsOrMonitorsEnabled = true, want false")
	}
}

func TestComputeMonitoringSummary_AuditIncludesCounts(t *testing.T) {
	monitors := []sentry.Monitor{
		{ID: "1", Status: "active", AlertRule: &sentry.MonitorAlert{Targets: []sentry.AlertTarget{{TargetType: 1}}}},
		{ID: "2", Status: "active"},
	}
	alertRules := []sentry.AlertRule{
		{ID: "1", Type: "rule", Actions: []sentry.AlertAction{{Type: "email"}}},
		{ID: "2", Type: "alert_rule"},
	}

	s := computeMonitoringSummary(monitors, alertRules, LevelAudit)

	if !s.AlertsOrMonitorsEnabled {
		t.Error("AlertsOrMonitorsEnabled = false, want true")
	}
	if s.TotalMonitoringRules == nil || *s.TotalMonitoringRules != 4 {
		t.Errorf("TotalMonitoringRules = %v, want 4", s.TotalMonitoringRules)
	}
	if s.CronMonitors == nil || *s.CronMonitors != 2 {
		t.Errorf("CronMonitors = %v, want 2", s.CronMonitors)
	}
	if s.AlertRules == nil || *s.AlertRules != 2 {
		t.Errorf("AlertRules = %v, want 2", s.AlertRules)
	}
	if s.WithActionsPct == nil || *s.WithActionsPct != 50 {
		t.Errorf("WithActionsPct = %v, want 50", s.WithActionsPct)
	}
}

func TestComputeMonitoringSummary_OnlyMonitorsEnabled(t *testing.T) {
	monitors := []sentry.Monitor{{ID: "1", Status: "active"}}

	s := computeMonitoringSummary(monitors, nil, LevelTrust)

	if !s.AlertsOrMonitorsEnabled {
		t.Error("AlertsOrMonitorsEnabled = false, want true with monitors only")
	}
}

func TestComputeMonitoringSummary_OnlyAlertRulesEnabled(t *testing.T) {
	alertRules := []sentry.AlertRule{{ID: "1", Type: "rule"}}

	s := computeMonitoringSummary(nil, alertRules, LevelTrust)

	if !s.AlertsOrMonitorsEnabled {
		t.Error("AlertsOrMonitorsEnabled = false, want true with alert rules only")
	}
}
