package sentry

import (
	"encoding/json"
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
	AlertThreshold float64       `json:"alertThreshold"`
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
