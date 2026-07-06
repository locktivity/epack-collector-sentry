package collector

import (
	"context"
	"net/http"
	"time"

	"github.com/locktivity/epack/componentsdk"
	"github.com/locktivity/epack-collector-sentry/internal/sentry"
)

type Level = componentsdk.Level

const (
	LevelTrust    = componentsdk.LevelTrust
	LevelAudit    = componentsdk.LevelAudit
	LevelInternal = componentsdk.LevelInternal
)

type Output struct {
	SchemaVersion     string             `json:"schema_version"`
	CollectedAt       string             `json:"collected_at"`
	CollectedAtLevel  string             `json:"collected_at_level"`
	Organization      string             `json:"organization"`
	MonitoringSummary MonitoringSummary   `json:"monitoring_summary"`
	Monitors          *MonitorMetrics    `json:"monitors,omitempty"`
	AlertRules        *AlertRuleMetrics  `json:"alert_rules,omitempty"`
	Users             *UserMetrics       `json:"users,omitempty"`
	Teams             *TeamMetrics       `json:"teams,omitempty"`
	Diagnostics       Diagnostics        `json:"diagnostics"`
}

type MonitoringSummary struct {
	AlertsOrMonitorsEnabled bool `json:"alerts_or_monitors_enabled"`
	TotalMonitoringRules    *int `json:"total_monitoring_rules,omitempty"`
	CronMonitors            *int `json:"cron_monitors,omitempty"`
	AlertRules              *int `json:"alert_rules,omitempty"`
	WithActionsPct          *int `json:"with_actions_pct,omitempty"`
}

type Diagnostics struct {
	Errors     []string          `json:"errors"`
	Warnings   []string          `json:"warnings"`
	Truncation map[string]string `json:"truncation"`
}

type SentryAPI interface {
	ListMonitors(ctx context.Context, projects []string, environments []string) ([]sentry.Monitor, error)
	ListAlertRules(ctx context.Context, projects []string) ([]sentry.AlertRule, error)
	ListMembers(ctx context.Context) ([]sentry.Member, error)
	ListTeams(ctx context.Context) ([]sentry.Team, error)
	ListTeamMembers(ctx context.Context, teamSlug string) ([]sentry.TeamMember, error)
}

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

type Collector struct {
	cfg    Config
	client SentryAPI
	level  Level
	clock  Clock
}

func New(cfg Config, client SentryAPI, level Level) *Collector {
	return &Collector{
		cfg:    cfg,
		client: client,
		level:  level,
		clock:  realClock{},
	}
}

func (c *Collector) Collect(ctx context.Context) (*Output, error) {
	monitors, err := c.client.ListMonitors(ctx, c.cfg.Projects, c.cfg.Environments)
	if err != nil {
		return nil, c.classifyError(err)
	}

	alertRules, err := c.client.ListAlertRules(ctx, c.cfg.Projects)
	if err != nil {
		return nil, c.classifyError(err)
	}

	diag := Diagnostics{
		Errors:     []string{},
		Warnings:   []string{},
		Truncation: map[string]string{},
	}

	summary := computeMonitoringSummary(monitors, alertRules, c.level)

	out := &Output{
		SchemaVersion:     "1.0.0",
		CollectedAt:       c.clock.Now().UTC().Format(time.RFC3339),
		CollectedAtLevel:  string(c.level),
		Organization:      c.cfg.Organization,
		MonitoringSummary: summary,
		Diagnostics:       diag,
	}

	if c.level.AtLeast(LevelAudit) {
		monitorMetrics, monTrunc := computeMonitorMetrics(monitors, c.level)
		alertRuleMetrics, arTrunc := computeAlertRuleMetrics(alertRules, c.level)
		out.Monitors = &monitorMetrics
		out.AlertRules = &alertRuleMetrics
		if monTrunc != nil {
			diag.Truncation["monitors"] = *monTrunc
		}
		if arTrunc != nil {
			diag.Truncation["alert_rules"] = *arTrunc
		}

		var members []sentry.Member
		var teams []sentry.Team

		members, err = c.client.ListMembers(ctx)
		if err != nil {
			if isPermissionError(err) {
				diag.Warnings = append(diag.Warnings, "member:read scope missing; user metrics unavailable")
			} else {
				return nil, c.classifyError(err)
			}
		}

		teams, err = c.client.ListTeams(ctx)
		if err != nil {
			if isPermissionError(err) {
				diag.Warnings = append(diag.Warnings, "team:read scope missing; team metrics unavailable")
			} else {
				return nil, c.classifyError(err)
			}
		}

		var teamMemberships TeamMembershipMap
		var teamMembersMap TeamMembersMap
		if c.level.AtLeast(LevelInternal) && teams != nil {
			teamMemberships = make(TeamMembershipMap)
			teamMembersMap = make(TeamMembersMap)
			for _, team := range teams {
				tmembers, tmErr := c.client.ListTeamMembers(ctx, team.Slug)
				if tmErr != nil {
					if isPermissionError(tmErr) {
						diag.Warnings = append(diag.Warnings, "team:read scope missing; team member details unavailable")
						teamMemberships = nil
						teamMembersMap = nil
						break
					}
					return nil, c.classifyError(tmErr)
				}
				teamMembersMap[team.Slug] = tmembers
				for _, tm := range tmembers {
					teamMemberships[tm.ID] = append(teamMemberships[tm.ID], TeamMembership{
						TeamSlug: team.Slug,
						TeamRole: tm.TeamRole,
					})
				}
			}
		}

		if members != nil {
			userMetrics, userTrunc := computeUserMetrics(members, c.level, teamMemberships)
			out.Users = &userMetrics
			if userTrunc != nil {
				diag.Truncation["users"] = *userTrunc
			}
		}

		if teams != nil {
			teamMetrics, teamTrunc := computeTeamMetrics(teams, c.level, teamMembersMap)
			out.Teams = &teamMetrics
			if teamTrunc != nil {
				diag.Truncation["teams"] = *teamTrunc
			}
		}

		out.Diagnostics = diag
	}

	return out, nil
}

func isPermissionError(err error) bool {
	apiErr, ok := err.(*sentry.APIError)
	if !ok {
		return false
	}
	return apiErr.StatusCode == http.StatusForbidden
}

func (c *Collector) classifyError(err error) error {
	apiErr, ok := err.(*sentry.APIError)
	if !ok {
		return componentsdk.NewNetworkError("sentry API request failed: %s", err)
	}

	switch apiErr.StatusCode {
	case http.StatusUnauthorized:
		return componentsdk.NewAuthError("invalid or expired auth token")
	case http.StatusForbidden:
		return componentsdk.NewAuthError("token missing required scope (need org:read, alerts:read)")
	default:
		return componentsdk.NewNetworkError("sentry API error (status %d)", apiErr.StatusCode)
	}
}
