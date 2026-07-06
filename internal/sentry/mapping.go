package sentry


func mapDetectorsToAlertRules(detectors []Detector, workflows []Workflow, projects []Project) []AlertRule {
	projectMap := buildProjectMap(projects)
	workflowMap := buildWorkflowMap(workflows)

	rules := make([]AlertRule, 0, len(detectors))
	for _, d := range detectors {
		rules = append(rules, mapDetectorToAlertRule(d, workflowMap, projectMap))
	}
	return rules
}

func mapDetectorToAlertRule(d Detector, workflowMap map[string]Workflow, projectMap map[string]string) AlertRule {
	ruleType := mapDetectorType(d.Type)

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
	}

	extractEnvironment(&rule, d)

	mapWorkflowActions(&rule, d.WorkflowIds, workflowMap)

	return rule
}

func extractMetricFields(rule *AlertRule, d Detector) {
	if len(d.DataSources) > 0 && d.DataSources[0].QueryObj != nil && d.DataSources[0].QueryObj.SnubaQuery != nil {
		sq := d.DataSources[0].QueryObj.SnubaQuery
		rule.Aggregate = sq.Aggregate
		rule.TimeWindow = sq.TimeWindow
	}

	if d.ConditionGroup != nil && len(d.ConditionGroup.Conditions) > 0 {
		rule.ThresholdType = d.ConditionGroup.Conditions[0].Comparison.ThresholdType
	}
}

func extractEnvironment(rule *AlertRule, d Detector) {
	if len(d.DataSources) > 0 && d.DataSources[0].QueryObj != nil && d.DataSources[0].QueryObj.SnubaQuery != nil {
		rule.Environment = d.DataSources[0].QueryObj.SnubaQuery.Environment
	}
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

func mapWorkflowActionToAlertAction(a WorkflowAction) AlertAction {
	aa := AlertAction{
		Type:       a.Type,
		TargetType: "specific",
	}
	if a.IntegrationID != nil && a.IntegrationID.Value != "" {
		aa.IntegrationID = a.IntegrationID.Value
	}
	return aa
}

func mapDetectorType(detectorType string) string {
	switch detectorType {
	case "metric_issue":
		return "alert_rule"
	case "error", "issue_stream":
		return "rule"
	default:
		return "rule"
	}
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
