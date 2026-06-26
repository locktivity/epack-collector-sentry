package collector

import "github.com/locktivity/epack-collector-sentry/internal/sentry"

func computeMonitoringSummary(monitors []sentry.Monitor, alertRules []sentry.AlertRule, level Level) MonitoringSummary {
	enabled := len(monitors) > 0 || len(alertRules) > 0

	s := MonitoringSummary{
		AlertsOrMonitorsEnabled: enabled,
	}

	if level.AtLeast(LevelAudit) {
		totalMonitors := len(monitors)
		totalAlertRules := len(alertRules)
		total := totalMonitors + totalAlertRules
		s.CronMonitors = &totalMonitors
		s.AlertRules = &totalAlertRules
		s.TotalMonitoringRules = &total

		if total > 0 {
			withActions := countMonitorsWithTargets(monitors) + countAlertRulesWithActions(alertRules)
			pct := (withActions * 100) / total
			s.WithActionsPct = &pct
		}
	}

	return s
}

func countMonitorsWithTargets(monitors []sentry.Monitor) int {
	count := 0
	for _, mon := range monitors {
		if mon.AlertRule != nil && len(mon.AlertRule.Targets) > 0 {
			count++
		}
	}
	return count
}

func countAlertRulesWithActions(rules []sentry.AlertRule) int {
	count := 0
	for _, r := range rules {
		if hasActions(r) {
			count++
		}
	}
	return count
}
