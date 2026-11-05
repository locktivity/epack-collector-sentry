package sentry

import "encoding/json"

func mapDetectorsToAlertRules(detectors []Detector, workflows []Workflow, projects []Project) []AlertRule {
	projectMap := buildProjectMap(projects)
	workflowMap := buildWorkflowMap(workflows)

	rules := make([]AlertRule, 0, len(detectors))
	for _, d := range detectors {
		ruleType, ok := alertRuleType(d)
		if !ok {
			continue
		}
		rules = append(rules, mapDetectorToAlertRule(d, ruleType, workflowMap, projectMap))
	}
	return rules
}

func mapDetectorToAlertRule(d Detector, ruleType string, workflowMap map[string]Workflow, projectMap map[string]string) AlertRule {
	rule := AlertRule{
		ID:            d.ID,
		Name:          d.Name,
		Type:          ruleType,
		DateCreated:   d.DateCreated,
		DetectionType: d.Config.DetectionType,
	}

	if d.Enabled {
		rule.Status = "active"
	} else {
		rule.Status = "disabled"
	}

	if slug, ok := projectMap[d.ProjectID]; ok {
		rule.Projects = []string{slug}
	} else if d.ProjectID != "" {
		rule.Projects = []string{d.ProjectID}
	}

	if d.Owner != nil {
		rule.Owner = d.Owner
	}

	if ruleType == "alert_rule" {
		extractMetricFields(&rule, d)
		extractEnvironment(&rule, d)
	} else {
		rule.Environment = workflowEnvironment(d.WorkflowIds, workflowMap)
	}

	mapWorkflowActions(&rule, d.WorkflowIds, workflowMap)

	return rule
}

// alertRuleType returns the alert rule type of d, or false when d is not an
// alert rule. A metric monitor is a metric alert. Every project has an error
// monitor and an issue stream monitor by default. Either one counts as an
// issue alert only when an alert is connected to it. Other monitors, such as
// uptime monitors, are not alert rules.
func alertRuleType(d Detector) (string, bool) {
	switch d.Type {
	case "metric_issue":
		return "alert_rule", true
	case "error", "issue_stream":
		if len(d.WorkflowIds) > 0 {
			return "rule", true
		}
	}
	return "", false
}

func extractMetricFields(rule *AlertRule, d Detector) {
	if len(d.DataSources) > 0 && d.DataSources[0].QueryObj != nil && d.DataSources[0].QueryObj.SnubaQuery != nil {
		sq := d.DataSources[0].QueryObj.SnubaQuery
		rule.Aggregate = sq.Aggregate
		// A monitor's timeWindow is in seconds.
		rule.TimeWindow = sq.TimeWindow / 60
	}

	if c, ok := issueCondition(d.ConditionGroup); ok {
		rule.ThresholdType = thresholdType(c)
	}
}

// issueCondition returns the first condition in g that raises an issue. A
// condition whose conditionResult is 0 resolves the issue.
func issueCondition(g *DetectorConditionGroup) (DetectorCondition, bool) {
	if g == nil {
		return DetectorCondition{}, false
	}
	for _, c := range g.Conditions {
		if result, ok := c.ConditionResult.(float64); ok && result == 0 {
			continue
		}
		return c, true
	}
	return DetectorCondition{}, false
}

// thresholdType returns 0 for above and 1 for below. For a dynamic monitor,
// the direction is the comparison's thresholdType. For a static or change
// monitor, the comparison is a number and the direction comes from the
// condition type. Any other comparison counts as above.
func thresholdType(c DetectorCondition) int {
	var dynamic struct {
		ThresholdType int `json:"thresholdType"`
	}
	if json.Unmarshal(c.Comparison, &dynamic) == nil {
		return dynamic.ThresholdType
	}
	var threshold float64
	if json.Unmarshal(c.Comparison, &threshold) == nil && (c.Type == "lt" || c.Type == "lte") {
		return 1
	}
	return 0
}

func extractEnvironment(rule *AlertRule, d Detector) {
	if len(d.DataSources) > 0 && d.DataSources[0].QueryObj != nil && d.DataSources[0].QueryObj.SnubaQuery != nil {
		rule.Environment = d.DataSources[0].QueryObj.SnubaQuery.Environment
	}
}

// workflowEnvironment returns the environment that the alerts connected to a
// monitor evaluate in. It is nil when the alerts name no environment or name
// different ones.
func workflowEnvironment(workflowIds []string, workflowMap map[string]Workflow) *string {
	var env *string
	seen := false
	for _, id := range workflowIds {
		w, ok := workflowMap[id]
		if !ok {
			continue
		}
		if seen && !sameEnvironment(env, w.Environment) {
			return nil
		}
		env, seen = w.Environment, true
	}
	return env
}

func sameEnvironment(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func mapWorkflowActions(rule *AlertRule, workflowIds []string, workflowMap map[string]Workflow) {
	if len(workflowIds) == 0 {
		return
	}

	for _, wID := range workflowIds {
		w, ok := workflowMap[wID]
		if !ok {
			continue
		}

		actions := collectWorkflowActions(w)
		if len(actions) == 0 {
			continue
		}

		if rule.Type == "alert_rule" {
			trigger := AlertTrigger{
				Label:   w.Name,
				Actions: actions,
			}
			rule.Triggers = append(rule.Triggers, trigger)
		} else {
			rule.Actions = append(rule.Actions, actions...)
		}
	}
}

func collectWorkflowActions(w Workflow) []AlertAction {
	var actions []AlertAction

	if w.Triggers != nil {
		for _, a := range w.Triggers.Actions {
			actions = append(actions, mapWorkflowActionToAlertAction(a))
		}
	}

	for _, af := range w.ActionFilters {
		for _, a := range af.Actions {
			actions = append(actions, mapWorkflowActionToAlertAction(a))
		}
	}

	return actions
}

// mapWorkflowActionToAlertAction takes the target type from the action's
// config. An email action's target type is user, team or issue_owners, and an
// integration action's is specific.
func mapWorkflowActionToAlertAction(a WorkflowAction) AlertAction {
	var config struct {
		TargetType string `json:"targetType"`
	}
	_ = json.Unmarshal(a.Config, &config)
	aa := AlertAction{
		Type:       a.Type,
		TargetType: config.TargetType,
	}
	if a.IntegrationID != nil && a.IntegrationID.Value != "" {
		aa.IntegrationID = a.IntegrationID.Value
	}
	return aa
}

func buildProjectMap(projects []Project) map[string]string {
	m := make(map[string]string, len(projects))
	for _, p := range projects {
		m[p.ID] = p.Slug
	}
	return m
}

func buildWorkflowMap(workflows []Workflow) map[string]Workflow {
	m := make(map[string]Workflow, len(workflows))
	for _, w := range workflows {
		m[w.ID] = w
	}
	return m
}
