package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/locktivity/epack-collector-sentry/internal/sentry"
	"github.com/locktivity/epack/componentsdk"
)

type Level = componentsdk.Level

const (
	LevelTrust    = componentsdk.LevelTrust
	LevelAudit    = componentsdk.LevelAudit
	LevelInternal = componentsdk.LevelInternal
)

type Output struct {
	SchemaVersion     string            `json:"schema_version"`
	CollectedAt       string            `json:"collected_at"`
	CollectedAtLevel  string            `json:"collected_at_level"`
	Organization      string            `json:"organization"`
	Auth              AuthSummary       `json:"auth"`
	MonitoringSummary MonitoringSummary `json:"monitoring_summary"`
	Monitors          *MonitorMetrics   `json:"monitors,omitempty"`
	AlertRules        *AlertRuleMetrics `json:"alert_rules,omitempty"`
	Users             *UserMetrics      `json:"users,omitempty"`
	Teams             *TeamMetrics      `json:"teams,omitempty"`
	Diagnostics       Diagnostics       `json:"diagnostics"`
}

type MonitoringSummary struct {
	AlertsOrMonitorsEnabled *bool `json:"alerts_or_monitors_enabled"`
	TotalMonitoringRules    *int  `json:"total_monitoring_rules,omitempty"`
	CronMonitors            *int  `json:"cron_monitors,omitempty"`
	AlertRules              *int  `json:"alert_rules,omitempty"`
	WithActionsPct          *int  `json:"with_actions_pct,omitempty"`
}

// MarshalJSON writes with_actions_pct as null when there are no monitoring
// rules to take it over. omitempty would drop it, as it must at trust, where
// no counts are taken.
func (s MonitoringSummary) MarshalJSON() ([]byte, error) {
	type summary MonitoringSummary
	if s.TotalMonitoringRules == nil || *s.TotalMonitoringRules != 0 {
		return json.Marshal(summary(s))
	}
	return json.Marshal(struct {
		summary
		WithActionsPct *int `json:"with_actions_pct"`
	}{summary: summary(s)})
}

type Diagnostics struct {
	Errors     []string          `json:"errors"`
	Warnings   []string          `json:"warnings"`
	Truncation map[string]string `json:"truncation"`
}

type SentryAPI interface {
	GetOrganization(ctx context.Context) (*sentry.Organization, error)
	ListMonitors(ctx context.Context, projects []string, environments []string) ([]sentry.Monitor, error)
	ListAlertRules(ctx context.Context, projects []string) ([]sentry.AlertRule, error)
	ListMembers(ctx context.Context) ([]sentry.Member, error)
	ListTeams(ctx context.Context) ([]sentry.Team, error)
	ListTeamMembers(ctx context.Context, teamSlug string) ([]sentry.TeamMember, error)
	GetAuthProvider(ctx context.Context) (*sentry.AuthProvider, error)
	ListIntegrations(ctx context.Context) ([]sentry.Integration, error)
	ListSentryAppInstallations(ctx context.Context) ([]sentry.SentryAppInstallation, error)
}

type Clock interface {
	Now() time.Time
}

type Collector struct {
	cfg    Config
	client SentryAPI
	level  Level
	clock  Clock
	rec    *componentsdk.Recorder
	// collectAccess is true when the run collects the access artifact, which
	// Collect then builds as access.
	collectAccess bool
	access        *AccessPosture
}

// New returns a collector that reads cfg's organization through client at
// level, stamps its output with clock, and records in rec each value it
// leaves out.
func New(cfg Config, client SentryAPI, level Level, clock Clock, rec *componentsdk.Recorder) *Collector {
	return &Collector{
		cfg:    cfg,
		client: client,
		level:  level,
		clock:  clock,
		rec:    rec,
	}
}

// Collect signs in and reads the organization. It returns an error only when
// the sign-in fails; a value it cannot read after that is left out of the
// output and recorded as a gap. When collectAccess is set, it also builds
// the access artifact, which AccessPosture returns.
func (c *Collector) Collect(ctx context.Context) (*Output, error) {
	org, err := c.client.GetOrganization(ctx)
	if err != nil {
		return nil, fmt.Errorf("signing in to Sentry: %w", err)
	}
	c.rec.SignedInAs(componentsdk.Principal{Provider: "sentry", Kind: "auth_token", TenantID: org.Slug})

	monitors, monitorsErr := c.client.ListMonitors(ctx, c.cfg.Projects, c.cfg.Environments)
	alertRules, alertRulesErr := c.client.ListAlertRules(ctx, c.cfg.Projects)

	diag := Diagnostics{
		Errors:     []string{},
		Warnings:   []string{},
		Truncation: map[string]string{},
	}

	// The auth-provider endpoint is private API, so a failure downgrades the
	// auth fields to unknown instead of failing the run.
	authProvider, authErr := c.client.GetAuthProvider(ctx)
	if authErr != nil {
		diag.Warnings = append(diag.Warnings, "auth-provider lookup failed; sso settings unknown")
		c.leaveOut(authErr, "/auth/sso_configured", "/auth/sso_provider", "/auth/sso_required", "/auth/scim_enabled")
	}

	// Members are read at every level for the access artifact, and from the
	// audit level for the detailed artifact.
	var members []sentry.Member
	var membersErr error
	if c.collectAccess || c.level.AtLeast(LevelAudit) {
		members, membersErr = c.client.ListMembers(ctx)
	}

	summary := c.monitoringSummary(monitors, monitorsErr, alertRules, alertRulesErr)

	out := &Output{
		SchemaVersion:     "1.0.0",
		CollectedAt:       c.clock.Now().UTC().Format(time.RFC3339),
		CollectedAtLevel:  string(c.level),
		Organization:      org.Slug,
		Auth:              computeAuthSummary(authProvider, authErr),
		MonitoringSummary: summary,
	}

	if c.level.AtLeast(LevelAudit) {
		if monitorsErr != nil {
			c.leaveOut(monitorsErr, "/monitors")
		} else {
			monitorMetrics, monTrunc := computeMonitorMetrics(monitors, c.level)
			out.Monitors = &monitorMetrics
			if monTrunc != nil {
				diag.Truncation["monitors"] = *monTrunc
			}
		}
		if alertRulesErr != nil {
			c.leaveOut(alertRulesErr, "/alert_rules")
		} else {
			alertRuleMetrics, arTrunc := computeAlertRuleMetrics(alertRules, c.level)
			out.AlertRules = &alertRuleMetrics
			if arTrunc != nil {
				diag.Truncation["alert_rules"] = *arTrunc
			}
		}
		c.collectPeople(ctx, out, &diag, members, membersErr)
	}

	out.Diagnostics = diag

	if c.collectAccess {
		c.access = c.accessPosture(ctx, out.CollectedAt, org, out.Auth, authErr, members, membersErr)
	}

	return out, nil
}

// AccessPosture returns the access artifact Collect built, or nil when the
// run does not collect it.
func (c *Collector) AccessPosture() *AccessPosture {
	return c.access
}

// monitoringSummary summarizes the monitors and alert rules, leaving out what
// a failed read makes unknown. Either list with an entry shows that
// monitoring is enabled, even when the other could not be read.
func (c *Collector) monitoringSummary(monitors []sentry.Monitor, monitorsErr error, alertRules []sentry.AlertRule, alertRulesErr error) MonitoringSummary {
	s := computeMonitoringSummary(monitors, alertRules, c.level)
	err := firstError(monitorsErr, alertRulesErr)
	if err == nil {
		return s
	}
	if (monitorsErr != nil || len(monitors) == 0) && (alertRulesErr != nil || len(alertRules) == 0) {
		s.AlertsOrMonitorsEnabled = nil
		c.leaveOut(err, "/monitoring_summary/alerts_or_monitors_enabled")
	}
	if !c.level.AtLeast(LevelAudit) {
		return s
	}
	s.TotalMonitoringRules, s.WithActionsPct = nil, nil
	c.leaveOut(err, "/monitoring_summary/total_monitoring_rules", "/monitoring_summary/with_actions_pct")
	if monitorsErr != nil {
		s.CronMonitors = nil
		c.leaveOut(monitorsErr, "/monitoring_summary/cron_monitors")
	}
	if alertRulesErr != nil {
		s.AlertRules = nil
		c.leaveOut(alertRulesErr, "/monitoring_summary/alert_rules")
	}
	return s
}

// collectPeople writes the members read with membersErr, and reads teams and
// at internal each team's members. A failed read leaves out what it would
// have given.
func (c *Collector) collectPeople(ctx context.Context, out *Output, diag *Diagnostics, members []sentry.Member, membersErr error) {
	if membersErr != nil {
		if isPermissionError(membersErr) {
			diag.Warnings = append(diag.Warnings, "member:read scope missing; user metrics unavailable")
		}
		c.leaveOut(membersErr, "/users")
	}

	teams, teamsErr := c.client.ListTeams(ctx)
	if teamsErr != nil {
		if isPermissionError(teamsErr) {
			diag.Warnings = append(diag.Warnings, "org:read scope missing; team metrics unavailable")
		}
		c.leaveOut(teamsErr, "/teams")
	}

	internal := c.level.AtLeast(LevelInternal)
	var teamMembersMap TeamMembersMap
	rosterErrs := map[string]error{}
	var firstRosterErr error
	if internal && teamsErr == nil {
		teamMembersMap = make(TeamMembersMap)
		warned := false
		for _, team := range teams {
			tmembers, tmErr := c.client.ListTeamMembers(ctx, team.Slug)
			if tmErr != nil {
				if isPermissionError(tmErr) && !warned {
					diag.Warnings = append(diag.Warnings, "team:read scope missing; team member details unavailable")
					warned = true
				}
				rosterErrs[team.Slug] = tmErr
				if firstRosterErr == nil {
					firstRosterErr = tmErr
				}
				continue
			}
			teamMembersMap[team.Slug] = tmembers
		}
	}

	// A user's teams come from every team's members, so one team whose
	// members could not be read leaves every user's teams unknown.
	var teamMemberships TeamMembershipMap
	if internal && teamsErr == nil && len(rosterErrs) == 0 {
		teamMemberships = membershipsByUser(teams, teamMembersMap)
	}

	if membersErr == nil {
		userMetrics, userTrunc := computeUserMetrics(members, c.level, teamMemberships)
		out.Users = &userMetrics
		if userTrunc != nil {
			diag.Truncation["users"] = *userTrunc
		}
		if internal && teamMemberships == nil {
			for i := range userMetrics.Inventory {
				pointer := fmt.Sprintf("/users/inventory/%d/team_memberships", i)
				if teamsErr != nil {
					c.leaveOut(teamsErr, pointer)
				} else {
					c.rec.Partial(location(pointer), len(rosterErrs), firstRosterErr)
				}
			}
		}
	}

	if teamsErr == nil {
		teamMetrics, teamTrunc := computeTeamMetrics(teams, c.level, teamMembersMap)
		out.Teams = &teamMetrics
		if teamTrunc != nil {
			diag.Truncation["teams"] = *teamTrunc
		}
		for i, team := range teamMetrics.Inventory {
			if err := rosterErrs[team.Slug]; err != nil {
				c.leaveOut(err, fmt.Sprintf("/teams/inventory/%d/members", i))
			}
		}
	}
}

func membershipsByUser(teams []sentry.Team, teamMembers TeamMembersMap) TeamMembershipMap {
	memberships := make(TeamMembershipMap)
	for _, team := range teams {
		for _, tm := range teamMembers[team.Slug] {
			memberships[tm.ID] = append(memberships[tm.ID], TeamMembership{
				TeamSlug: team.Slug,
				TeamRole: tm.TeamRole,
			})
		}
	}
	return memberships
}

// leaveOut records the values at pointers as not collected because of err.
func (c *Collector) leaveOut(err error, pointers ...string) {
	for _, pointer := range pointers {
		c.rec.NotCollected(location(pointer), err)
	}
}

func location(pointer string) componentsdk.Location {
	return componentsdk.Location{Path: artifactPath, Pointer: pointer}
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func isPermissionError(err error) bool {
	var apiErr *sentry.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden
}
