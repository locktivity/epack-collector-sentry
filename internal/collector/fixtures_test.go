package collector

import (
	"net/http"
	"strings"

	"github.com/locktivity/epack/componentsdk/collectortest"
)

const (
	kitToken = "kit-sentry-token-0123456789"
	kitAPI   = "https://sentry.io/api/0"
	kitOrg   = kitAPI + "/organizations/acme-corp"
)

func kitConfig() map[string]any {
	return map[string]any{"organization": "acme-corp", "level": "internal"}
}

func kitSecrets() map[string]string {
	return map[string]string{"SENTRY_AUTH_TOKEN": kitToken}
}

// kitLink is the Link header of the first page of a list at url, with cursor
// as the next page's cursor. Each link carries its cursor in its URL and in a
// cursor parameter, as Sentry's links do.
func kitLink(url, cursor string) http.Header {
	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}
	return http.Header{"Link": []string{
		`<` + url + sep + `cursor=0:0:1>; rel="previous"; results="false"; cursor="0:0:1", ` +
			`<` + url + sep + `cursor=` + cursor + `>; rel="next"; results="true"; cursor="` + cursor + `"`,
	}}
}

// kitLastLink is the Link header of the last page of a list at url. Sentry
// still sends a next link there, with cursor as its cursor, marked
// results="false".
func kitLastLink(url, cursor string) http.Header {
	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}
	return http.Header{"Link": []string{
		`<` + url + sep + `cursor=0:0:1>; rel="previous"; results="true"; cursor="0:0:1", ` +
			`<` + url + sep + `cursor=` + cursor + `>; rel="next"; results="false"; cursor="` + cursor + `"`,
	}}
}

// kitResponses answer every call a clean internal-level run makes. They hold
// richFake's organization grown to three monitors over two pages, and a
// metric and an issue alert rule. They also hold four members over two pages,
// one a pending invitation, and three teams, one with no members. The
// organization has three integrations over two pages, one disabled, and two
// Sentry app installations, one pending.
func kitResponses() []collectortest.Response {
	return []collectortest.Response{
		{URL: kitOrg + "/", Body: kitOrganization},
		{URL: kitOrg + "/monitors/?project=-1", Header: kitLink(kitOrg+"/monitors/?project=-1", "0:100:0"), Body: `[` + kitMonitorDailySync + `,` + kitMonitorWeeklyReport + `]`},
		{URL: kitOrg + "/monitors/?project=-1&cursor=0:100:0", Header: kitLastLink(kitOrg+"/monitors/?project=-1", "0:200:0"), Body: `[` + kitMonitorNightlyBackup + `]`},
		{URL: kitOrg + "/detectors/?per_page=100", Body: kitDetectors},
		{URL: kitOrg + "/workflows/?per_page=100", Body: kitWorkflows},
		{URL: kitOrg + "/projects/?per_page=100", Body: kitProjects},
		{URL: kitOrg + "/auth-provider/", Body: kitAuthProvider},
		// The first page has no query, which would match the second page's
		// request too, so it answers once.
		{URL: kitOrg + "/members/", Times: 1, Header: kitLink(kitOrg+"/members/", "100:1:0"), Body: `[` + kitMemberAdmin + `,` + kitMemberDev + `]`},
		{URL: kitOrg + "/members/?cursor=100:1:0", Header: kitLastLink(kitOrg+"/members/", "100:2:0"), Body: `[` + kitMemberInvited + `,` + kitMemberOps + `]`},
		{URL: kitOrg + "/teams/?per_page=100", Body: kitTeams},
		{URL: kitAPI + "/teams/acme-corp/backend/members/", Body: kitBackendMembers},
		{URL: kitAPI + "/teams/acme-corp/frontend/members/", Body: kitFrontendMembers},
		{URL: kitAPI + "/teams/acme-corp/platform/members/", Body: `[]`},
		{URL: kitIntegrationsURL, Header: kitLink(kitIntegrationsURL, "100:1:0"), Body: `[` + kitIntegrationSlack + `,` + kitIntegrationGitHub + `]`},
		{URL: kitIntegrationsURL + "&cursor=100:1:0", Header: kitLastLink(kitIntegrationsURL, "100:2:0"), Body: `[` + kitIntegrationJira + `]`},
		{URL: kitInstallationsURL, Header: kitLastLink(kitInstallationsURL, "100:1:0"), Body: kitInstallations},
	}
}

// kitEmptyResponses answer an organization with no monitors, alert rules,
// projects, members, or teams, and no auth provider.
func kitEmptyResponses() []collectortest.Response {
	return []collectortest.Response{
		{URL: kitOrg + "/", Body: kitOrganization},
		{URL: kitOrg + "/monitors/?project=-1", Body: `[]`},
		{URL: kitOrg + "/detectors/?per_page=100", Body: `[]`},
		{URL: kitOrg + "/workflows/?per_page=100", Body: `[]`},
		{URL: kitOrg + "/projects/?per_page=100", Body: `[]`},
		{URL: kitOrg + "/auth-provider/", Status: http.StatusNoContent},
		{URL: kitOrg + "/members/", Body: `[]`},
		{URL: kitOrg + "/teams/?per_page=100", Body: `[]`},
		{URL: kitIntegrationsURL, Header: kitLastLink(kitIntegrationsURL, "100:1:0"), Body: `[]`},
		{URL: kitInstallationsURL, Header: kitLastLink(kitInstallationsURL, "100:1:0"), Body: `[]`},
	}
}

const kitOrganization = `{"id":"4507","slug":"acme-corp","name":"Acme Corp","require2FA":true,"openMembership":false,"status":{"id":"active","name":"active"}}`

const (
	kitIntegrationsURL  = kitOrg + "/integrations/?includeConfig=0&per_page=100"
	kitInstallationsURL = kitOrg + "/sentry-app-installations/?per_page=100"
)

const (
	kitIntegrationSlack = `{"id": "301", "name": "Acme Workspace", "icon": null, "domainName": "acme.slack.com", "accountType": null, "scopes": ["chat:write"],
		"status": "active", "provider": {"key": "slack", "slug": "slack", "name": "Slack", "canAdd": true, "canDisable": false, "features": ["alert-rule", "chat-unfurl"], "aspects": {}},
		"configOrganization": [], "configData": null, "externalId": "T0123", "organizationId": 4507, "organizationIntegrationStatus": "active", "gracePeriodEnd": null}`
	kitIntegrationGitHub = `{"id": "302", "name": "acme", "icon": null, "domainName": "github.com/acme", "accountType": "Organization", "scopes": null,
		"status": "active", "provider": {"key": "github", "slug": "github", "name": "GitHub", "canAdd": true, "canDisable": false, "features": ["commits", "issue-basic", "stacktrace-link"], "aspects": {}},
		"configOrganization": [], "configData": null, "externalId": "4401", "organizationId": 4507, "organizationIntegrationStatus": "active", "gracePeriodEnd": null}`
	kitIntegrationJira = `{"id": "303", "name": "acme.atlassian.net", "icon": null, "domainName": "acme.atlassian.net", "accountType": null, "scopes": null,
		"status": "active", "provider": {"key": "jira", "slug": "jira", "name": "Jira", "canAdd": true, "canDisable": false, "features": ["issue-sync"], "aspects": {}},
		"configOrganization": [], "configData": null, "externalId": "acme.atlassian.net", "organizationId": 4507, "organizationIntegrationStatus": "disabled", "gracePeriodEnd": null}`
)

// kitInstallations are an installed internal integration and a public Sentry
// app whose installation is pending.
const kitInstallations = `[
	{"app": {"uuid": "6f6a0e8e-1d5b-4bd4-9a51-6b1f1b2b7c01", "slug": "acme-deploy-bot", "sentryAppId": 81}, "organization": {"slug": "acme-corp", "id": 4507},
		"uuid": "0b8f1a4e-7f0e-4d3a-9b0e-2c6d1e5a9f10", "status": "installed"},
	{"app": {"uuid": "2c1d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f", "slug": "linear", "sentryAppId": 12}, "organization": {"slug": "acme-corp", "id": 4507},
		"uuid": "9e8d7c6b-5a4f-4e3d-8c2b-1a0f9e8d7c6b", "status": "pending"}
]`

const kitMonitorDailySync = `{
	"id": "m1", "name": "daily-sync", "slug": "daily-sync", "status": "active", "type": "cron_job", "isMuted": false,
	"config": {"schedule_type": "crontab", "schedule": "0 2 * * *", "checkin_margin": 5, "max_runtime": 30},
	"project": {"id": "11", "slug": "billing", "name": "Billing"},
	"alertRule": {"targets": [{"targetType": 2, "targetIdentifier": 42}]},
	"owner": {"type": "team", "id": "42", "name": "backend"},
	"environments": [{"name": "production", "status": "ok", "lastCheckIn": "2026-06-26T02:00:05Z", "nextCheckIn": "2026-06-27T02:00:00Z", "activeIncident": null}],
	"dateCreated": "2025-01-10T09:00:00Z"
}`

const kitMonitorWeeklyReport = `{
	"id": "m2", "name": "weekly-report", "slug": "weekly-report", "status": "disabled", "type": "cron_job", "isMuted": false,
	"config": {"schedule_type": "interval", "schedule": [7, "day"]},
	"project": {"id": "12", "slug": "reporting", "name": "Reporting"},
	"alertRule": null, "owner": null, "environments": [],
	"dateCreated": "2025-03-02T09:00:00Z"
}`

const kitMonitorNightlyBackup = `{
	"id": "m3", "name": "nightly-backup", "slug": "nightly-backup", "status": "error", "type": "cron_job", "isMuted": true,
	"config": {"schedule_type": "crontab", "schedule": "0 3 * * *", "checkin_margin": 10, "max_runtime": 120},
	"project": {"id": "11", "slug": "billing", "name": "Billing"},
	"alertRule": {"targets": [{"targetType": 1, "targetIdentifier": 9001}]},
	"owner": null,
	"environments": [{"name": "production", "status": "error", "lastCheckIn": "2026-06-26T03:00:02Z", "nextCheckIn": "2026-06-27T03:00:00Z",
		"activeIncident": {"startingTimestamp": "2026-06-26T03:30:00Z", "resolvingTimestamp": null}}],
	"dateCreated": "2025-05-20T09:00:00Z"
}`

const kitDetectors = `[
	{
		"id": "d1", "projectId": "11", "name": "High Error Rate", "type": "metric_issue", "description": null,
		"owner": {"type": "team", "id": "42", "name": "backend"}, "createdBy": "9001",
		"dateCreated": "2025-11-01T10:00:00Z", "dateUpdated": "2025-11-01T10:00:00Z", "workflowIds": ["w1"],
		"dataSources": [{"id": "ds1", "type": "snuba_query_subscription", "sourceId": "s1",
			"queryObj": {"id": "q1", "snubaQuery": {"dataset": "events", "query": "", "aggregate": "count()", "timeWindow": 300, "environment": "production"}}}],
		"conditionGroup": {"id": "cg1", "logicType": "any", "conditions": [
			{"id": "c1", "type": "gt", "comparison": 100, "conditionResult": 75},
			{"id": "c2", "type": "lte", "comparison": 100, "conditionResult": 0}]},
		"config": {"detectionType": "static", "comparisonDelta": null}, "enabled": true
	},
	{
		"id": "d2", "projectId": "12", "name": "New Issue", "type": "error", "description": null, "owner": null, "createdBy": null,
		"dateCreated": "2025-12-01T10:00:00Z", "dateUpdated": "2025-12-01T10:00:00Z", "workflowIds": ["w2"],
		"dataSources": [], "conditionGroup": null, "config": {}, "enabled": false
	}
]`

// kitDefaultMonitors are the error monitor and the issue stream monitor
// every project has by default, with no alert connected.
const kitDefaultMonitors = `[
	{"id": "d3", "projectId": "11", "name": "Error Monitor", "type": "error", "description": null, "owner": null, "createdBy": null,
		"dateCreated": "2025-01-01T00:00:00Z", "dateUpdated": "2025-01-01T00:00:00Z", "workflowIds": [],
		"dataSources": null, "conditionGroup": null, "config": {}, "enabled": true},
	{"id": "d4", "projectId": "11", "name": "Issue Stream Monitor", "type": "issue_stream", "description": null, "owner": null, "createdBy": null,
		"dateCreated": "2025-01-01T00:00:00Z", "dateUpdated": "2025-01-01T00:00:00Z", "workflowIds": [],
		"dataSources": null, "conditionGroup": null, "config": {}, "enabled": true}
]`

const kitWorkflows = `[
	{
		"id": "w1", "name": "Notify Slack", "organizationId": "4507", "createdBy": "9001",
		"dateCreated": "2025-11-01T10:00:00Z", "dateUpdated": "2025-11-01T10:00:00Z",
		"triggers": {"logicType": "any-short", "conditions": [], "actions": []},
		"actionFilters": [{"logicType": "all", "conditions": [], "actions": [{"id": "a1", "type": "slack", "integrationId": 123, "data": {},
			"config": {"targetType": "specific", "targetDisplay": "#alerts", "targetIdentifier": "C0123"}, "status": "active"}]}],
		"environment": null, "config": {}, "detectorIds": ["d1"], "enabled": true, "lastTriggered": null, "owner": null
	},
	{
		"id": "w2", "name": "Email Team", "organizationId": "4507", "createdBy": null,
		"dateCreated": "2025-12-01T10:00:00Z", "dateUpdated": "2025-12-01T10:00:00Z",
		"triggers": {"logicType": "any", "conditions": [], "actions": [{"id": "a2", "type": "email", "integrationId": null, "data": {},
			"config": {"targetType": "team", "targetDisplay": null, "targetIdentifier": "42"}, "status": "active"}]},
		"actionFilters": [], "environment": null, "config": {}, "detectorIds": ["d2"], "enabled": true, "lastTriggered": null, "owner": null
	}
]`

const kitProjects = `[{"id": "11", "slug": "billing", "name": "Billing"}, {"id": "12", "slug": "reporting", "name": "Reporting"}]`

const kitAuthProvider = `{"id":"1","provider_name":"okta","pending_links_count":0,"login_url":"https://sentry.io/auth/login/acme-corp/","default_role":"member","require_link":true,"scim_enabled":true}`

// kitAuthProviderOptional is kitAuthProvider with Require SSO off.
const kitAuthProviderOptional = `{"id":"1","provider_name":"okta","pending_links_count":0,"login_url":"https://sentry.io/auth/login/acme-corp/","default_role":"member","require_link":false,"scim_enabled":true}`

const (
	kitMemberAdmin = `{"id": "101", "email": "admin@acme.com", "name": "Admin", "orgRole": "admin", "pending": false, "expired": false,
		"dateCreated": "2024-01-15T09:00:00Z", "user": {"id": "9001", "isActive": true, "has2fa": true, "lastActive": "2026-06-25T11:30:00Z"}, "flags": {"sso:linked": true}}`
	kitMemberDev = `{"id": "102", "email": "dev@acme.com", "name": "Dev", "orgRole": "member", "pending": false, "expired": false,
		"dateCreated": "2024-03-01T09:00:00Z", "user": {"id": "9002", "isActive": true, "has2fa": false, "lastActive": "2026-06-20T08:00:00Z"}, "flags": {"sso:linked": false}}`
	kitMemberInvited = `{"id": "103", "email": "new@acme.com", "name": "new@acme.com", "orgRole": "member", "pending": true, "expired": false,
		"dateCreated": "2026-06-24T09:00:00Z", "user": null, "flags": {"sso:linked": false}}`
	kitMemberOps = `{"id": "104", "email": "ops@acme.com", "name": "Ops", "orgRole": "manager", "pending": false, "expired": false,
		"dateCreated": "2024-06-01T09:00:00Z", "user": {"id": "9004", "isActive": true, "has2fa": true, "lastActive": "2026-06-26T07:00:00Z"}, "flags": {"sso:linked": true}}`
)

const kitTeams = `[
	{"id": "t1", "slug": "backend", "name": "Backend", "memberCount": 2, "dateCreated": "2024-01-10T09:00:00Z",
		"projects": [{"id": "11", "slug": "billing", "name": "Billing"}, {"id": "12", "slug": "reporting", "name": "Reporting"}]},
	{"id": "t2", "slug": "frontend", "name": "Frontend", "memberCount": 1, "dateCreated": "2024-02-10T09:00:00Z",
		"projects": [{"id": "11", "slug": "billing", "name": "Billing"}]},
	{"id": "t3", "slug": "platform", "name": "Platform", "memberCount": 0, "dateCreated": "2025-02-10T09:00:00Z", "projects": []}
]`

const (
	kitBackendMembers = `[{"id": "101", "email": "admin@acme.com", "name": "Admin", "teamRole": "admin", "user": {"id": "9001", "isActive": true, "has2fa": true}},
		{"id": "102", "email": "dev@acme.com", "name": "Dev", "teamRole": "contributor", "user": {"id": "9002", "isActive": true, "has2fa": false}}]`
	kitFrontendMembers = `[{"id": "104", "email": "ops@acme.com", "name": "Ops", "teamRole": "contributor", "user": {"id": "9004", "isActive": true, "has2fa": true}}]`
)

// kitOpenOrganization is kitOrganization with Open Membership on.
const kitOpenOrganization = `{"id":"4507","slug":"acme-corp","name":"Acme Corp","require2FA":true,"openMembership":true,"status":{"id":"active","name":"active"}}`

// kitOrganizationWithoutOpenMembership is kitOrganization without its
// openMembership value.
const kitOrganizationWithoutOpenMembership = `{"id":"4507","slug":"acme-corp","name":"Acme Corp","require2FA":true,"status":{"id":"active","name":"active"}}`

// kitAccessOrganization requires no two-factor authentication.
const kitAccessOrganization = `{"id":"4507","slug":"acme-corp","name":"Acme Corp","require2FA":false,"openMembership":false,"status":{"id":"active","name":"active"}}`

// kitAccessAuthProvider is a Google provider with Require SSO off.
const kitAccessAuthProvider = `{"id":"2","provider_name":"google","pending_links_count":0,"login_url":"https://sentry.io/auth/login/acme-corp/","default_role":"member","require_link":false,"scim_enabled":false}`

// kitAccessMembers are five members. An owner and a manager sign in without
// SSO, and only the owner has 2FA. One member was last active in February.
// Another has a deactivated account, and the last is linked to SSO.
const kitAccessMembers = `[
	{"id": "201", "email": "owner@acme.com", "name": "Owner", "orgRole": "owner", "pending": false, "expired": false, "dateCreated": "2024-01-15T09:00:00Z",
		"user": {"id": "9101", "isActive": true, "has2fa": true, "lastActive": "2026-06-25T11:30:00.123456Z"}, "flags": {"sso:linked": false}},
	{"id": "202", "email": "manager@acme.com", "name": "Manager", "orgRole": "manager", "pending": false, "expired": false, "dateCreated": "2024-02-15T09:00:00Z",
		"user": {"id": "9102", "isActive": true, "has2fa": false, "lastActive": "2026-06-24T08:00:00Z"}, "flags": {"sso:linked": false}},
	{"id": "203", "email": "dormant@acme.com", "name": "Dormant", "orgRole": "member", "pending": false, "expired": false, "dateCreated": "2024-03-15T09:00:00Z",
		"user": {"id": "9103", "isActive": true, "has2fa": true, "lastActive": "2026-02-01T08:00:00Z"}, "flags": {"sso:linked": false}},
	{"id": "204", "email": "former@acme.com", "name": "Former", "orgRole": "member", "pending": false, "expired": false, "dateCreated": "2024-04-15T09:00:00Z",
		"user": {"id": "9104", "isActive": false, "has2fa": false, "lastActive": "2026-06-20T08:00:00Z"}, "flags": {"sso:linked": false}},
	{"id": "205", "email": "linked@acme.com", "name": "Linked", "orgRole": "member", "pending": false, "expired": false, "dateCreated": "2024-05-15T09:00:00Z",
		"user": {"id": "9105", "isActive": true, "has2fa": false, "lastActive": "2026-06-26T07:00:00Z"}, "flags": {"sso:linked": true}}
]`
