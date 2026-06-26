package collector

import (
	"fmt"

	"github.com/locktivity/epack-collector-sentry/internal/sentry"
)

type AlertRuleMetrics struct {
	TotalCount            int                   `json:"total_count"`
	MetricAlertCount      int                   `json:"metric_alert_count"`
	IssueAlertCount       int                   `json:"issue_alert_count"`
	WithActionsCount      int                   `json:"with_actions_count"`
	WithActionsPct        *int                  `json:"with_actions_pct"`
	DenominatorAlertRules int                   `json:"denominator_alert_rules"`
	Inventory             []AlertRuleInventory  `json:"inventory,omitempty"`
}

type AlertRuleInventory struct {
	ID               string                   `json:"id"`
	Name             string                   `json:"name"`
	Type             string                   `json:"type"`
	Status           any                      `json:"status"`
	Aggregate        string                   `json:"aggregate,omitempty"`
	TimeWindowMinutes float64                 `json:"time_window_minutes,omitempty"`
	ThresholdType    string                   `json:"threshold_type,omitempty"`
	DetectionType    string                   `json:"detection_type,omitempty"`
	Project          string                   `json:"project,omitempty"`
	Environment      string                   `json:"environment,omitempty"`
	Owner            string                   `json:"owner,omitempty"`
	Triggers         []AlertTriggerInventory  `json:"triggers,omitempty"`
	DateCreated      string                   `json:"date_created,omitempty"`
}

type AlertTriggerInventory struct {
	Label          string                 `json:"label"`
	AlertThreshold float64               `json:"alert_threshold"`
	Actions        []AlertActionInventory `json:"actions"`
}

type AlertActionInventory struct {
	Type          string `json:"type"`
	TargetType    string `json:"target_type"`
	IntegrationID string `json:"integration_id,omitempty"`
}

func computeAlertRuleMetrics(rules []sentry.AlertRule, level Level) (AlertRuleMetrics, *string) {
	m := AlertRuleMetrics{
		TotalCount:            len(rules),
		DenominatorAlertRules: len(rules),
	}

	for _, r := range rules {
		switch r.Type {
		case "alert_rule":
			m.MetricAlertCount++
		case "rule":
			m.IssueAlertCount++
		}

		if hasActions(r) {
			m.WithActionsCount++
		}
	}

	if m.TotalCount > 0 {
		pct := (m.WithActionsCount * 100) / m.TotalCount
		m.WithActionsPct = &pct
	}

	var truncationMsg *string
	if level.AtLeast(LevelAudit) {
		items := rules
		if len(items) > maxInventorySize {
			msg := fmt.Sprintf("alert_rules truncated from %d to %d", len(items), maxInventorySize)
			truncationMsg = &msg
			items = items[:maxInventorySize]
		}
		inv := make([]AlertRuleInventory, 0, len(items))
		for _, r := range items {
			inv = append(inv, buildAlertRuleInventory(r, level))
		}
		m.Inventory = inv
	}

	return m, truncationMsg
}

func buildAlertRuleInventory(r sentry.AlertRule, level Level) AlertRuleInventory {
	ruleType := "issue"
	if r.Type == "alert_rule" {
		ruleType = "metric"
	}

	item := AlertRuleInventory{
		ID:          r.ID,
		Name:        r.Name,
		Type:        ruleType,
		Status:      r.Status,
		DateCreated: r.DateCreated,
	}

	if ruleType == "metric" {
		item.Aggregate = r.Aggregate
		item.TimeWindowMinutes = r.TimeWindow
		item.DetectionType = r.DetectionType
		switch r.ThresholdType {
		case 0:
			item.ThresholdType = "above"
		case 1:
			item.ThresholdType = "below"
		}
	}

	if len(r.Projects) > 0 {
		item.Project = r.Projects[0]
	}
	if r.Environment != nil {
		item.Environment = *r.Environment
	}
	if r.Owner != nil {
		item.Owner = *r.Owner
	}

	triggers := make([]AlertTriggerInventory, 0, len(r.Triggers))
	for _, t := range r.Triggers {
		actions := make([]AlertActionInventory, 0, len(t.Actions))
		for _, a := range t.Actions {
			ai := AlertActionInventory{
				Type:       a.Type,
				TargetType: a.TargetType,
			}
			if level.AtLeast(LevelInternal) && a.IntegrationID != nil {
				ai.IntegrationID = fmt.Sprintf("%v", a.IntegrationID)
			}
			actions = append(actions, ai)
		}
		triggers = append(triggers, AlertTriggerInventory{
			Label:          t.Label,
			AlertThreshold: t.AlertThreshold,
			Actions:        actions,
		})
	}
	if len(triggers) > 0 {
		item.Triggers = triggers
	}

	return item
}

func hasActions(r sentry.AlertRule) bool {
	if len(r.Actions) > 0 {
		return true
	}
	for _, t := range r.Triggers {
		if len(t.Actions) > 0 {
			return true
		}
	}
	return false
}
