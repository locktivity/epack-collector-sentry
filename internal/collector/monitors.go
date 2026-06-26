package collector

import (
	"fmt"
	"time"

	"github.com/locktivity/epack-collector-sentry/internal/sentry"
)

type MonitorMetrics struct {
	TotalCount            int                `json:"total_count"`
	ActiveCount           int                `json:"active_count"`
	DisabledCount         int                `json:"disabled_count"`
	MutedCount            int                `json:"muted_count"`
	WithAlertTargetsCount int                `json:"with_alert_targets_count"`
	WithAlertTargetsPct   *int               `json:"with_alert_targets_pct"`
	DenominatorMonitors   int                `json:"denominator_monitors"`
	Inventory             []MonitorInventory `json:"inventory,omitempty"`
}

type MonitorInventory struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	Type               string              `json:"type"`
	Status             string              `json:"status"`
	IsMuted            bool                `json:"is_muted"`
	ScheduleType       string              `json:"schedule_type"`
	Schedule           any                 `json:"schedule"`
	Project            string              `json:"project"`
	Owner              string              `json:"owner,omitempty"`
	ConnectedWorkflows []ConnectedWorkflow `json:"connected_workflows"`
	LatestGroup        *LatestGroup        `json:"latest_group,omitempty"`
	Environments       []string            `json:"environments"`
	OpenAlertCount     *int                `json:"open_alert_count,omitempty"`
}

type ConnectedWorkflow struct {
	TargetType       string `json:"target_type"`
	TargetIdentifier int    `json:"target_identifier"`
}

type LatestGroup struct {
	Environment    string          `json:"environment"`
	Status         string          `json:"status"`
	LastCheckIn    *time.Time      `json:"last_check_in"`
	NextCheckIn    *time.Time      `json:"next_check_in"`
	ActiveIncident *IncidentEntry  `json:"active_incident"`
}

type IncidentEntry struct {
	StartingTimestamp  time.Time  `json:"starting_timestamp"`
	ResolvingTimestamp *time.Time `json:"resolving_timestamp"`
}

func computeMonitorMetrics(monitors []sentry.Monitor, level Level) (MonitorMetrics, *string) {
	m := MonitorMetrics{
		TotalCount:          len(monitors),
		DenominatorMonitors: len(monitors),
	}

	for _, mon := range monitors {
		switch mon.Status {
		case "active", "ok", "error", "missed_checkin":
			m.ActiveCount++
		case "disabled":
			m.DisabledCount++
		}

		if mon.IsMuted {
			m.MutedCount++
		}

		if mon.AlertRule != nil && len(mon.AlertRule.Targets) > 0 {
			m.WithAlertTargetsCount++
		}
	}

	if m.TotalCount > 0 {
		pct := (m.WithAlertTargetsCount * 100) / m.TotalCount
		m.WithAlertTargetsPct = &pct
	}

	var truncationMsg *string
	if level.AtLeast(LevelAudit) {
		items := monitors
		if len(items) > maxInventorySize {
			msg := fmt.Sprintf("monitors truncated from %d to %d", len(items), maxInventorySize)
			truncationMsg = &msg
			items = items[:maxInventorySize]
		}
		inv := make([]MonitorInventory, 0, len(items))
		for _, mon := range items {
			inv = append(inv, buildMonitorInventory(mon, level))
		}
		m.Inventory = inv
	}

	return m, truncationMsg
}

func buildMonitorInventory(mon sentry.Monitor, level Level) MonitorInventory {
	item := MonitorInventory{
		ID:           mon.ID,
		Name:         mon.Name,
		Type:         mon.Type,
		Status:       mon.Status,
		IsMuted:      mon.IsMuted,
		ScheduleType: mon.Config.ScheduleType,
		Schedule:     mon.Config.Schedule,
		Project:      mon.Project.Slug,
		ConnectedWorkflows: buildConnectedWorkflows(mon),
		LatestGroup:        buildLatestGroup(mon),
		Environments:       buildEnvironmentNames(mon),
	}

	if mon.Owner != nil {
		item.Owner = mon.Owner.Type + ":" + mon.Owner.Name
	}

	if level.AtLeast(LevelInternal) {
		count := countOpenAlerts(mon)
		item.OpenAlertCount = &count
	}

	return item
}

func countOpenAlerts(mon sentry.Monitor) int {
	count := 0
	for _, env := range mon.Environments {
		if env.ActiveIncident != nil && env.ActiveIncident.ResolvingTimestamp == nil {
			count++
		}
	}
	return count
}

func buildConnectedWorkflows(mon sentry.Monitor) []ConnectedWorkflow {
	if mon.AlertRule == nil {
		return []ConnectedWorkflow{}
	}
	workflows := make([]ConnectedWorkflow, 0, len(mon.AlertRule.Targets))
	for _, t := range mon.AlertRule.Targets {
		targetType := "unknown"
		switch t.TargetType {
		case 1:
			targetType = "user"
		case 2:
			targetType = "team"
		}
		workflows = append(workflows, ConnectedWorkflow{
			TargetType:       targetType,
			TargetIdentifier: t.TargetIdentifier,
		})
	}
	return workflows
}

func buildLatestGroup(mon sentry.Monitor) *LatestGroup {
	if len(mon.Environments) == 0 {
		return nil
	}
	env := mon.Environments[0]
	lg := &LatestGroup{
		Environment: env.Name,
		Status:      env.Status,
		LastCheckIn: env.LastCheckIn,
		NextCheckIn: env.NextCheckIn,
	}
	if env.ActiveIncident != nil {
		lg.ActiveIncident = &IncidentEntry{
			StartingTimestamp:  env.ActiveIncident.StartingTimestamp,
			ResolvingTimestamp: env.ActiveIncident.ResolvingTimestamp,
		}
	}
	return lg
}

func buildEnvironmentNames(mon sentry.Monitor) []string {
	names := make([]string, 0, len(mon.Environments))
	for _, e := range mon.Environments {
		names = append(names, e.Name)
	}
	return names
}
