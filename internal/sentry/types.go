package sentry

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Monitor struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Slug         string         `json:"slug"`
	Status       string         `json:"status"`
	Type         string         `json:"type"`
	IsMuted      bool           `json:"isMuted"`
	Config       MonitorConfig  `json:"config"`
	Project      MonitorProject `json:"project"`
	AlertRule    *MonitorAlert  `json:"alertRule"`
	Owner        *MonitorOwner  `json:"owner"`
	Environments []MonitorEnv   `json:"environments"`
	DateCreated  time.Time      `json:"dateCreated"`
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
	Name           string          `json:"name"`
	Status         string          `json:"status"`
	LastCheckIn    *time.Time      `json:"lastCheckIn"`
	NextCheckIn    *time.Time      `json:"nextCheckIn"`
	ActiveIncident *ActiveIncident `json:"activeIncident"`
}

type ActiveIncident struct {
	StartingTimestamp  time.Time  `json:"startingTimestamp"`
	ResolvingTimestamp *time.Time `json:"resolvingTimestamp"`
}

type AlertRule struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Type          string         `json:"type"`
	Status        any            `json:"status"`
	DateCreated   string         `json:"dateCreated"`
	Projects      []string       `json:"projects"`
	Environment   *string        `json:"environment"`
	Owner         *OwnerField    `json:"owner"`
	Triggers      []AlertTrigger `json:"triggers,omitempty"`
	Actions       []AlertAction  `json:"actions,omitempty"`
	Aggregate     string         `json:"aggregate,omitempty"`
	TimeWindow    float64        `json:"timeWindow,omitempty"` // minutes
	ThresholdType int            `json:"thresholdType,omitempty"`
	DetectionType string         `json:"detectionType,omitempty"`
}

type AlertTrigger struct {
	Label          string        `json:"label"`
	AlertThreshold *float64      `json:"alertThreshold,omitempty"`
	Actions        []AlertAction `json:"actions"`
}

type AlertAction struct {
	Type          string `json:"type"`
	TargetType    string `json:"targetType"`
	IntegrationID any    `json:"integrationId,omitempty"`
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

// AuthProvider is the org SSO configuration. RequireLink is the "Require SSO"
// enforcement setting: when true, members must authenticate through the
// provider and cannot use password login.
type AuthProvider struct {
	ProviderName string `json:"provider_name"`
	RequireLink  bool   `json:"require_link"`
	ScimEnabled  bool   `json:"scim_enabled"`
}

type Team struct {
	ID          string        `json:"id"`
	Slug        string        `json:"slug"`
	Name        string        `json:"name"`
	MemberCount int           `json:"memberCount"`
	Projects    []TeamProject `json:"projects"`
	DateCreated string        `json:"dateCreated"`
}

// TeamProject is a project the team is assigned to.
type TeamProject struct {
	Slug string `json:"slug"`
}

// TeamMember is one entry of a team's members list. ID is the organization
// membership's id, the same id the organization's members list gives. User
// is the member's Sentry user, with its own id. It is null for a member who
// was invited and has not joined yet.
type TeamMember struct {
	ID       string      `json:"id"`
	Email    string      `json:"email"`
	Name     string      `json:"name"`
	TeamRole string      `json:"teamRole"`
	User     *MemberUser `json:"user"`
}

// Detector represents a monitoring rule from the documented
// GET /api/0/organizations/{org}/detectors/ endpoint.
type Detector struct {
	ID             string                  `json:"id"`
	ProjectID      string                  `json:"projectId"`
	Name           string                  `json:"name"`
	Type           string                  `json:"type"`
	Description    *string                 `json:"description"`
	Owner          *OwnerField             `json:"owner"`
	CreatedBy      *string                 `json:"createdBy"`
	DateCreated    string                  `json:"dateCreated"`
	DateUpdated    string                  `json:"dateUpdated"`
	WorkflowIds    []string                `json:"workflowIds"`
	DataSources    []DetectorDataSource    `json:"dataSources"`
	ConditionGroup *DetectorConditionGroup `json:"conditionGroup"`
	Config         DetectorConfig          `json:"config"`
	Enabled        bool                    `json:"enabled"`
}

type DetectorDataSource struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	SourceID string            `json:"sourceId"`
	QueryObj *DetectorQueryObj `json:"queryObj"`
}

type DetectorQueryObj struct {
	ID         string              `json:"id"`
	SnubaQuery *DetectorSnubaQuery `json:"snubaQuery"`
}

type DetectorSnubaQuery struct {
	Dataset     string  `json:"dataset"`
	Query       string  `json:"query"`
	Aggregate   string  `json:"aggregate"`
	TimeWindow  float64 `json:"timeWindow"`
	Environment *string `json:"environment"`
}

type DetectorConditionGroup struct {
	ID         string              `json:"id"`
	LogicType  string              `json:"logicType"`
	Conditions []DetectorCondition `json:"conditions"`
}

type DetectorCondition struct {
	ID              string          `json:"id"`
	Type            string          `json:"type"`
	Comparison      json.RawMessage `json:"comparison"`
	ConditionResult any             `json:"conditionResult"`
}

type DetectorConfig struct {
	DetectionType   string `json:"detectionType"`
	ComparisonDelta any    `json:"comparisonDelta"`
}

// Workflow represents a notification/action config from the documented
// GET /api/0/organizations/{org}/workflows/ endpoint.
type Workflow struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	OrganizationID string                 `json:"organizationId"`
	CreatedBy      *string                `json:"createdBy"`
	DateCreated    string                 `json:"dateCreated"`
	DateUpdated    string                 `json:"dateUpdated"`
	Triggers       *WorkflowTriggers      `json:"triggers"`
	ActionFilters  []WorkflowActionFilter `json:"actionFilters"`
	Environment    *string                `json:"environment"`
	Config         json.RawMessage        `json:"config"`
	DetectorIds    []string               `json:"detectorIds"`
	Enabled        bool                   `json:"enabled"`
	LastTriggered  *string                `json:"lastTriggered"`
	Owner          *string                `json:"owner"`
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

// Organization is the part of the organization details the collector reads.
// Require2FA and OpenMembership are nil when the response leaves them out.
// OpenMembership is the Open Membership setting, which lets any member join
// any team.
type Organization struct {
	ID             string `json:"id"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	Require2FA     *bool  `json:"require2FA"`
	OpenMembership *bool  `json:"openMembership"`
}

// Integration is an integration installed in the organization, such as Slack
// or GitHub. OrganizationIntegrationStatus is active, disabled, or
// pending_deletion.
type Integration struct {
	ID                            string              `json:"id"`
	Name                          string              `json:"name"`
	Provider                      IntegrationProvider `json:"provider"`
	OrganizationIntegrationStatus string              `json:"organizationIntegrationStatus"`
}

// IntegrationProvider names the service an integration connects to.
type IntegrationProvider struct {
	Key string `json:"key"`
}

// SentryAppInstallation is a Sentry app installed in the organization. Its
// Status is pending, installed, or pending_deletion.
type SentryAppInstallation struct {
	UUID   string                   `json:"uuid"`
	Status string                   `json:"status"`
	App    SentryAppInstallationApp `json:"app"`
}

// SentryAppInstallationApp names the installed app.
type SentryAppInstallationApp struct {
	Slug string `json:"slug"`
}
