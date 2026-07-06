package sentry

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Monitor struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Status      string          `json:"status"`
	Type        string          `json:"type"`
	IsMuted     bool            `json:"isMuted"`
	Config      MonitorConfig   `json:"config"`
	Project     MonitorProject  `json:"project"`
	AlertRule   *MonitorAlert   `json:"alertRule"`
	Owner       *MonitorOwner   `json:"owner"`
	Environments []MonitorEnv   `json:"environments"`
	DateCreated time.Time       `json:"dateCreated"`
}

type MonitorConfig struct {
	ScheduleType  string `json:"schedule_type"`
	Schedule      any    `json:"schedule"`
	CheckinMargin int    `json:"checkin_margin,omitempty"`
	MaxRuntime    int    `json:"max_runtime,omitempty"`
}

type MonitorProject struct {
	ID   any    `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type MonitorAlert struct {
	Targets []AlertTarget `json:"targets"`
}

type AlertTarget struct {
	TargetType       int `json:"targetType"`
	TargetIdentifier int `json:"targetIdentifier"`
}

type MonitorOwner struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FlexibleID handles Sentry API fields that may be returned as a string,
// number, or null. Normalizes to a string value.
type FlexibleID struct {
	Value string
}

func (id *FlexibleID) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		id.Value = s
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		id.Value = n.String()
		return nil
	}
	return fmt.Errorf("integrationId: expected string, number, or null, got %s", string(data))
}

// OwnerField handles the Sentry API returning owner as either a string
// ("team:backend") or an object ({"type":"team","id":"42","name":"backend"}).
type OwnerField struct {
	*MonitorOwner
}

func (o *OwnerField) UnmarshalJSON(data []byte) error {
	// Try object first
	var obj MonitorOwner
	if err := json.Unmarshal(data, &obj); err == nil && obj.Type != "" {
		o.MonitorOwner = &obj
		return nil
	}
	// Fall back to string like "team:backend"
	var s string
	if err := json.Unmarshal(data, &s); err == nil && s != "" {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) == 2 {
			o.MonitorOwner = &MonitorOwner{Type: parts[0], Name: parts[1]}
		} else {
			o.MonitorOwner = &MonitorOwner{Name: s}
		}
		return nil
	}
	return nil
}

type MonitorEnv struct {
	Name            string           `json:"name"`
	Status          string           `json:"status"`
	LastCheckIn     *time.Time       `json:"lastCheckIn"`
	NextCheckIn     *time.Time       `json:"nextCheckIn"`
	ActiveIncident  *ActiveIncident  `json:"activeIncident"`
}

type ActiveIncident struct {
	StartingTimestamp  time.Time  `json:"startingTimestamp"`
	ResolvingTimestamp *time.Time `json:"resolvingTimestamp"`
}

type AlertRule struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	Status        any               `json:"status"`
	DateCreated   string            `json:"dateCreated"`
	Projects      []string          `json:"projects"`
	Environment   *string           `json:"environment"`
	Owner         *OwnerField       `json:"owner"`
	Triggers      []AlertTrigger    `json:"triggers,omitempty"`
	Actions       []AlertAction     `json:"actions,omitempty"`
	Aggregate     string            `json:"aggregate,omitempty"`
	TimeWindow    float64           `json:"timeWindow,omitempty"`
	ThresholdType int               `json:"thresholdType,omitempty"`
	DetectionType string            `json:"detectionType,omitempty"`
}

type AlertTrigger struct {
	Label          string        `json:"label"`
	AlertThreshold *float64      `json:"alertThreshold,omitempty"`
	Actions        []AlertAction `json:"actions"`
}

type AlertAction struct {
	Type           string `json:"type"`
	TargetType     string `json:"targetType"`
	IntegrationID  any    `json:"integrationId,omitempty"`
}

type Member struct {
	ID          string      `json:"id"`
	Email       string      `json:"email"`
	Name        string      `json:"name"`
	OrgRole     string      `json:"orgRole"`
	Pending     bool        `json:"pending"`
	Expired     bool        `json:"expired"`
	DateCreated string      `json:"dateCreated"`
	User        *MemberUser `json:"user"`
	Flags       MemberFlags `json:"flags"`
}

type MemberUser struct {
	ID         string `json:"id"`
	IsActive   bool   `json:"isActive"`
	Has2fa     bool   `json:"has2fa"`
	LastActive string `json:"lastActive,omitempty"`
}

type MemberFlags struct {
	SSOLinked bool `json:"sso:linked"`
}

type Team struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	MemberCount  int    `json:"memberCount"`
	ProjectCount int    `json:"projectCount,omitempty"`
	DateCreated  string `json:"dateCreated"`
}

type TeamMember struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	TeamRole string `json:"teamRole"`
}

// Detector represents a monitoring rule from the documented
// GET /api/0/organizations/{org}/detectors/ endpoint.
type Detector struct {
	ID           string              `json:"id"`
	ProjectID    string              `json:"projectId"`
	Name         string              `json:"name"`
	Type         string              `json:"type"`
	Description  *string             `json:"description"`
	Owner        *OwnerField         `json:"owner"`
	CreatedBy    *string             `json:"createdBy"`
	DateCreated  string              `json:"dateCreated"`
	DateUpdated  string              `json:"dateUpdated"`
	WorkflowIds  []string            `json:"workflowIds"`
	DataSources  []DetectorDataSource `json:"dataSources"`
	ConditionGroup *DetectorConditionGroup `json:"conditionGroup"`
	Config       DetectorConfig      `json:"config"`
	Enabled      bool                `json:"enabled"`
}

type DetectorDataSource struct {
	ID       string                `json:"id"`
	Type     string                `json:"type"`
	SourceID string                `json:"sourceId"`
	QueryObj *DetectorQueryObj     `json:"queryObj"`
}

type DetectorQueryObj struct {
	ID          string               `json:"id"`
	SnubaQuery  *DetectorSnubaQuery  `json:"snubaQuery"`
}

type DetectorSnubaQuery struct {
	Dataset     string   `json:"dataset"`
	Query       string   `json:"query"`
	Aggregate   string   `json:"aggregate"`
	TimeWindow  float64  `json:"timeWindow"`
	Environment *string  `json:"environment"`
}

type DetectorConditionGroup struct {
	ID         string               `json:"id"`
	LogicType  string               `json:"logicType"`
	Conditions []DetectorCondition  `json:"conditions"`
}

type DetectorCondition struct {
	ID              string                    `json:"id"`
	Type            string                    `json:"type"`
	Comparison      DetectorConditionComparison `json:"comparison"`
	ConditionResult any                       `json:"conditionResult"`
}

type DetectorConditionComparison struct {
	ThresholdType int     `json:"thresholdType"`
	Sensitivity   string  `json:"sensitivity,omitempty"`
	Seasonality   string  `json:"seasonality,omitempty"`
}

type DetectorConfig struct {
	DetectionType   string `json:"detectionType"`
	ComparisonDelta any    `json:"comparisonDelta"`
}

// Workflow represents a notification/action config from the documented
// GET /api/0/organizations/{org}/workflows/ endpoint.
type Workflow struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	OrganizationID string              `json:"organizationId"`
	CreatedBy      *string             `json:"createdBy"`
	DateCreated    string              `json:"dateCreated"`
	DateUpdated    string              `json:"dateUpdated"`
	Triggers       *WorkflowTriggers   `json:"triggers"`
	ActionFilters  []WorkflowActionFilter `json:"actionFilters"`
	Environment    *string             `json:"environment"`
	Config         json.RawMessage     `json:"config"`
	DetectorIds    []string            `json:"detectorIds"`
	Enabled        bool                `json:"enabled"`
	LastTriggered  *string             `json:"lastTriggered"`
	Owner          *string             `json:"owner"`
}

type WorkflowTriggers struct {
	LogicType  string              `json:"logicType"`
	Conditions []WorkflowCondition `json:"conditions"`
	Actions    []WorkflowAction    `json:"actions"`
}

type WorkflowActionFilter struct {
	LogicType  string              `json:"logicType"`
	Conditions []WorkflowCondition `json:"conditions"`
	Actions    []WorkflowAction    `json:"actions"`
}

type WorkflowCondition struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	Comparison      any    `json:"comparison"`
	ConditionResult any    `json:"conditionResult"`
}

type WorkflowAction struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	IntegrationID *FlexibleID     `json:"integrationId"`
	Data          json.RawMessage `json:"data"`
	Config        json.RawMessage `json:"config"`
}

// Project is used to resolve project IDs to slugs.
type Project struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}
