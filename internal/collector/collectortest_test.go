package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/locktivity/epack/componentsdk"
	"github.com/locktivity/epack/componentsdk/collectortest"
)

var kitOperations = []string{
	"sentry.organization.get",
	"sentry.monitors.list",
	"sentry.detectors.list",
	"sentry.workflows.list",
	"sentry.projects.list",
	"sentry.auth_provider.get",
	"sentry.members.list",
	"sentry.teams.list",
	"sentry.teams.members.list",
	"sentry.integrations.list",
	"sentry.sentry_app_installations.list",
}

func TestCollectorKit(t *testing.T) {
	collectortest.Run(t, collectortest.Suite{
		Spec:    Spec("test", "collectortest"),
		Run:     Run,
		Config:  kitConfig(),
		Secrets: kitSecrets(),
		Clock:   goldenTime,
		Scenarios: []collectortest.Scenario{
			{
				Name:      "clean",
				Responses: kitResponses(),
				// The pending invitation is in the detailed artifact's 2FA
				// denominator, 2 of 4. The access artifact has 3 members
				// without it. Only the manager counts as an admin there.
				// Sentry's admin role cannot manage members. A team member's
				// user_id is the id of the member's Sentry user. A user's
				// teams are matched by the membership id.
				Expect: map[string]map[string]any{
					artifactPath: {
						"/monitoring_summary/with_actions_pct": 80,
						"/users/has_2fa_pct":                   50,
						"/users/inventory/0/id":                "101",
						"/users/inventory/0/team_memberships":  []any{map[string]any{"team_slug": "backend", "team_role": "admin"}},
						"/users/inventory/2/team_memberships":  []any{},
						"/users/inventory/3/team_memberships":  []any{map[string]any{"team_slug": "frontend", "team_role": "contributor"}},
						"/teams/inventory/0/members": []any{
							map[string]any{"user_id": "9001", "email": "admin@acme.com", "team_role": "admin"},
							map[string]any{"user_id": "9002", "email": "dev@acme.com", "team_role": "contributor"},
						},
						"/teams/inventory/1/members/0/user_id": "9004",
						"/teams/inventory/2/members":           []any{},
					},
					accessPosturePath: {
						"/provider":                            "sentry",
						"/system_type":                         "monitoring",
						"/sign_in/sso_configured":              true,
						"/sign_in/sso_required":                true,
						"/sign_in/mfa_required":                true,
						"/sign_in/scim_enabled":                true,
						"/members/members_present":             true,
						"/members/member_count":                3,
						"/members/sso_member_count":            2,
						"/members/sso_coverage_pct":            66,
						"/members/local_members_present":       true,
						"/members/local_member_count":          1,
						"/members/local_mfa_member_count":      0,
						"/members/local_mfa_coverage_pct":      0,
						"/members/inactive_member_count":       0,
						"/members/inactive_pct":                0,
						"/members/external_members_present":    collectortest.Absent,
						"/admins/admins_present":               true,
						"/admins/admin_count":                  1,
						"/admins/admin_share_pct":              34,
						"/admins/local_admins_present":         false,
						"/admins/local_admin_count":            0,
						"/admins/local_admin_mfa_coverage_pct": nil,
						"/root":                                collectortest.Absent,
						"/credentials":                         collectortest.Absent,
						"/connected_apps/apps_present":         true,
						"/connected_apps/app_count":            3,
					},
				},
			},
			{
				Name:          "token rejected",
				ExpectFailure: componentsdk.CodeAuthFailed,
				Responses: []collectortest.Response{{
					URL:    kitOrg + "/",
					Status: http.StatusUnauthorized,
					Body:   `{"detail":"Invalid token"}`,
				}},
			},
			{
				Name:          "organization not found",
				ExpectFailure: componentsdk.CodeConfigInvalid,
				Responses: []collectortest.Response{{
					URL:    kitOrg + "/",
					Status: http.StatusNotFound,
					Body:   `{"detail":"The requested resource does not exist"}`,
				}},
			},
			{
				// With a token that lacks member:read and team:read, the
				// monitors, alert rules, and SSO settings are still collected.
				Name: "no member or team scope",
				Responses: append(kitResponsesExcept(kitOrg+"/members/", kitOrg+"/teams/", kitAPI+"/teams/"),
					kitForbidden(kitOrg+"/members/"), kitForbidden(kitOrg+"/teams/?per_page=100")),
				Expect: map[string]map[string]any{
					artifactPath: {
						"/auth/sso_required":                             true,
						"/monitoring_summary/alerts_or_monitors_enabled": true,
						"/monitors/total_count":                          3,
						"/users":                                         nil,
						"/teams":                                         nil,
					},
					accessPosturePath: {
						"/sign_in/sso_required":        true,
						"/sign_in/mfa_required":        true,
						"/members":                     collectortest.Absent,
						"/admins":                      collectortest.Absent,
						"/connected_apps/apps_present": true,
					},
				},
			},
			{
				// The owner and the manager are not linked to SSO, and only
				// the owner has 2FA. With Require SSO off, the member linked
				// to SSO can still sign in without it, so all five members
				// count as local. The member last active in February and the
				// deactivated one are inactive.
				Name: "admins without SSO",
				Responses: replaceResponses(kitEmptyResponses(),
					collectortest.Response{URL: kitOrg + "/", Body: kitAccessOrganization},
					collectortest.Response{URL: kitOrg + "/auth-provider/", Body: kitAccessAuthProvider},
					collectortest.Response{URL: kitOrg + "/members/", Body: kitAccessMembers},
				),
				Expect: map[string]map[string]any{accessPosturePath: {
					"/sign_in/sso_configured":              true,
					"/sign_in/sso_required":                false,
					"/sign_in/mfa_required":                false,
					"/sign_in/scim_enabled":                false,
					"/members/member_count":                5,
					"/members/sso_coverage_pct":            20,
					"/members/local_member_count":          5,
					"/members/local_mfa_coverage_pct":      40,
					"/members/inactive_member_count":       2,
					"/members/inactive_pct":                40,
					"/admins/admin_count":                  2,
					"/admins/admin_share_pct":              40,
					"/admins/local_admins_present":         true,
					"/admins/local_mfa_admin_count":        1,
					"/admins/local_admin_mfa_coverage_pct": 50,
					"/connected_apps/apps_present":         false,
				}},
			},
			{
				// With Require SSO off, the admin and the manager linked to
				// SSO can still sign in with a password, so every member
				// counts as local.
				Name:      "Require SSO off",
				Responses: replaceResponse(kitResponses(), collectortest.Response{URL: kitOrg + "/auth-provider/", Body: kitAuthProviderOptional}),
				Expect: map[string]map[string]any{accessPosturePath: {
					"/sign_in/sso_configured":              true,
					"/sign_in/sso_required":                false,
					"/members/sso_member_count":            2,
					"/members/local_members_present":       true,
					"/members/local_member_count":          3,
					"/members/local_mfa_member_count":      2,
					"/members/local_mfa_coverage_pct":      66,
					"/admins/local_admins_present":         true,
					"/admins/local_admin_count":            1,
					"/admins/local_mfa_admin_count":        1,
					"/admins/local_admin_mfa_coverage_pct": 100,
				}},
			},
			{
				// With Open Membership on, the member with the admin role can
				// join any team and delete its projects, so they count as an
				// admin beside the manager.
				Name:      "Open Membership on",
				Responses: replaceResponse(kitResponses(), collectortest.Response{URL: kitOrg + "/", Body: kitOpenOrganization}),
				Expect: map[string]map[string]any{accessPosturePath: {
					"/members/member_count":     3,
					"/admins/admins_present":    true,
					"/admins/admin_count":       2,
					"/admins/admin_share_pct":   67,
					"/admins/local_admin_count": 0,
				}},
			},
			{
				// Without the Open Membership setting, the member with the
				// admin role may or may not be an admin.
				Name:      "no Open Membership value",
				Responses: replaceResponse(kitResponses(), collectortest.Response{URL: kitOrg + "/", Body: kitOrganizationWithoutOpenMembership}),
				Expect: map[string]map[string]any{accessPosturePath: {
					"/members/member_count": 3,
					"/admins":               collectortest.Absent,
				}},
			},
			{
				// Without the SSO settings, the members linked to SSO may or
				// may not be local. The member not linked to SSO is local
				// either way, and both admins are linked.
				Name:      "SSO settings forbidden",
				Responses: replaceResponse(kitResponses(), kitForbidden(kitOrg+"/auth-provider/")),
				Expect: map[string]map[string]any{
					artifactPath: {
						"/auth/sso_required": nil,
					},
					accessPosturePath: {
						"/sign_in/sso_required":                collectortest.Absent,
						"/sign_in/mfa_required":                true,
						"/members/sso_coverage_pct":            66,
						"/members/local_members_present":       true,
						"/members/local_member_count":          collectortest.Absent,
						"/members/local_mfa_coverage_pct":      collectortest.Absent,
						"/admins/admins_present":               true,
						"/admins/local_admins_present":         collectortest.Absent,
						"/admins/local_admin_count":            collectortest.Absent,
						"/admins/local_admin_mfa_coverage_pct": collectortest.Absent,
					},
				},
			},
			{
				// With both integration lists forbidden, the members and
				// sign-in rules are still collected.
				Name: "integrations forbidden",
				Responses: replaceResponses(kitResponsesExcept(kitIntegrationsURL, kitInstallationsURL),
					kitForbidden(kitIntegrationsURL), kitForbidden(kitInstallationsURL)),
				Expect: map[string]map[string]any{accessPosturePath: {
					"/members/member_count":        3,
					"/connected_apps/apps_present": collectortest.Absent,
					"/connected_apps/app_count":    collectortest.Absent,
				}},
			},
			{
				// At trust, the access artifact holds the present flags and
				// percentages and leaves out the counts.
				Name:      "trust level",
				Config:    map[string]any{"level": "trust"},
				Responses: kitTrustResponses(),
				Expect: map[string]map[string]any{accessPosturePath: {
					"/sign_in/mfa_required":                true,
					"/members/members_present":             true,
					"/members/member_count":                collectortest.Absent,
					"/members/sso_coverage_pct":            66,
					"/members/inactive_pct":                0,
					"/admins/admin_share_pct":              34,
					"/admins/local_admins_present":         false,
					"/admins/local_admin_mfa_coverage_pct": nil,
					"/connected_apps/apps_present":         true,
					"/connected_apps/app_count":            collectortest.Absent,
				}},
			},
			{
				// Without the access artifact, a trust run reads no members
				// and no integrations.
				Name:      "access artifact left out",
				Config:    map[string]any{"level": "trust", "collect": []any{"details"}},
				Responses: kitResponsesExcept(kitOrg+"/members/", kitOrg+"/teams/", kitAPI+"/teams/", kitIntegrationsURL, kitInstallationsURL),
				Expect: map[string]map[string]any{artifactPath: {
					"/auth/sso_required": true,
				}},
			},
			{
				// The monitors every project has by default are not alert
				// rules while no alert is connected to them.
				Name: "default monitors only",
				Responses: replaceResponse(kitEmptyResponses(), collectortest.Response{
					URL: kitOrg + "/detectors/?per_page=100", Body: kitDefaultMonitors,
				}),
				Expect: map[string]map[string]any{artifactPath: {
					"/monitoring_summary/alerts_or_monitors_enabled": false,
					"/monitoring_summary/alert_rules":                0,
					"/alert_rules/total_count":                       0,
				}},
			},
			{
				Name:      "empty tenant",
				Responses: kitEmptyResponses(),
				Expect: map[string]map[string]any{
					artifactPath: {
						"/auth/sso_configured":                           false,
						"/monitoring_summary/alerts_or_monitors_enabled": false,
						"/monitoring_summary/total_monitoring_rules":     0,
						"/monitoring_summary/with_actions_pct":           nil,
						"/monitors/with_alert_targets_pct":               nil,
						"/alert_rules/with_actions_pct":                  nil,
						"/users/has_2fa_pct":                             nil,
						"/monitors/inventory":                            []any{},
						"/users/inventory":                               []any{},
						"/teams/inventory":                               []any{},
					},
					accessPosturePath: {
						"/sign_in/sso_configured":              false,
						"/sign_in/sso_required":                false,
						"/sign_in/scim_enabled":                false,
						"/members/members_present":             false,
						"/members/member_count":                0,
						"/members/sso_coverage_pct":            nil,
						"/members/local_members_present":       false,
						"/members/local_mfa_coverage_pct":      nil,
						"/members/inactive_pct":                nil,
						"/admins/admins_present":               false,
						"/admins/admin_share_pct":              nil,
						"/admins/local_admins_present":         false,
						"/admins/local_admin_mfa_coverage_pct": nil,
						"/connected_apps/apps_present":         false,
						"/connected_apps/app_count":            0,
					},
				},
			},
		},
		Required: []string{"sentry.organization.get"},
		Semantics: []collectortest.Semantic{{
			Operation:     "sentry.organization.get",
			Fault:         "404",
			Meaning:       "organization not found",
			ExpectFailure: componentsdk.CodeConfigInvalid,
		}},
		Ignore:     map[string][]string{artifactPath: {"/diagnostics"}},
		Operations: kitOperations,
	})
}

func TestRunFailsOnInvalidConfiguration(t *testing.T) {
	cases := []struct {
		name    string
		config  map[string]any
		secrets map[string]string
	}{
		{"no organization", map[string]any{"level": "trust"}, kitSecrets()},
		{"no token", kitConfig(), nil},
		{"plain http", map[string]any{"organization": "acme-corp", "sentry_url": "http://sentry.example.com"}, kitSecrets()},
		{"no host", map[string]any{"organization": "acme-corp", "sentry_url": "https://"}, kitSecrets()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := runInProcess(t, tc.config, tc.secrets, nil)
			if res.ExitCode != 2 || res.Failure == nil || res.Failure.Code != componentsdk.CodeConfigInvalid {
				t.Fatalf("exit %d, failure %+v; want exit 2 with config_invalid", res.ExitCode, res.Failure)
			}
		})
	}
}

func TestRunRecordsTheSignedInPrincipal(t *testing.T) {
	res := runInProcess(t, kitConfig(), kitSecrets(), kitResponses())
	if res.ExitCode != 0 || res.Result == nil {
		t.Fatalf("run failed with exit %d: %s", res.ExitCode, res.Stderr)
	}
	want := &componentsdk.Principal{Provider: "sentry", Kind: "auth_token", TenantID: "acme-corp"}
	if !reflect.DeepEqual(res.Result.SignedInAs, want) {
		t.Errorf("signed_in_as = %+v, want %+v", res.Result.SignedInAs, want)
	}
	if len(res.Result.Checks) != 0 {
		t.Errorf("checks = %+v, want none", res.Result.Checks)
	}
	for _, g := range res.Result.Gaps {
		if g.Code != componentsdk.CodeUnsupported && g.Code != componentsdk.CodeNotRead {
			t.Errorf("gap %+v; want only the gaps on values Sentry has no data for or does not report to this token", g)
		}
	}
	if res.Result.Outcome.Status != componentsdk.OutcomeCollected {
		t.Errorf("outcome = %+v, want collected", res.Result.Outcome)
	}
}

// Every level declares the organization the config named, at the detailed
// artifact's organization slug, which every level holds. Only the audit
// level counts it.
func TestRunDeclaresTheConfiguredOrganization(t *testing.T) {
	one, complete := 1, true
	for _, tc := range []struct {
		level     string
		responses []collectortest.Response
		count     *int
	}{
		{"trust", kitTrustResponses(), nil},
		{"audit", kitResponses(), &one},
	} {
		config := kitConfig()
		config["level"] = tc.level
		res := runInProcess(t, config, kitSecrets(), tc.responses)
		if res.ExitCode != 0 || res.Result == nil {
			t.Fatalf("%s: run failed with exit %d: %s", tc.level, res.ExitCode, res.Stderr)
		}
		want := []componentsdk.ResultTarget{{
			Kind: componentsdk.TargetOrganization, ChosenBy: componentsdk.ChosenByConfig, ConfigKey: "organization",
			At:    &componentsdk.Location{Path: artifactPath, Pointer: "/organization"},
			Count: tc.count, Complete: &complete,
		}}
		if !reflect.DeepEqual(res.Result.Targets, want) {
			got, _ := json.Marshal(res.Result.Targets)
			wanted, _ := json.Marshal(want)
			t.Errorf("%s: targets = %s, want %s", tc.level, got, wanted)
		}
		for _, f := range res.Result.Checks {
			t.Errorf("%s: check finding %+v", tc.level, f)
		}
	}
}

func TestRunWithoutMemberReadSucceedsWithAGap(t *testing.T) {
	responses := replaceResponse(kitResponses(), collectortest.Response{
		URL:    kitOrg + "/members/",
		Status: http.StatusForbidden,
		Body:   `{"detail":"You do not have permission to perform this action."}`,
	})
	config := kitConfig()
	config["level"] = "audit"
	res := runInProcess(t, config, kitSecrets(), responses)
	if res.ExitCode != 0 || res.Result == nil {
		t.Fatalf("run failed with exit %d: %s", res.ExitCode, res.Stderr)
	}
	if res.Result.Outcome.Status != componentsdk.OutcomePartial || res.Result.Outcome.Code != componentsdk.CodePermissionDenied {
		t.Errorf("outcome = %+v, want partial with permission_denied", res.Result.Outcome)
	}
	denied := func(at componentsdk.Location) componentsdk.Gap {
		return componentsdk.Gap{Location: at, Status: componentsdk.GapNotCollected, Code: componentsdk.CodePermissionDenied, Owner: componentsdk.OwnerCustomer,
			Operation: "sentry.members.list", MissingPermission: "member:read"}
	}
	access := func(pointer string) componentsdk.Location {
		at := accessLocation(pointer)
		at.Schema = accessPostureSchema
		return at
	}
	want := []componentsdk.Gap{denied(access("/admins")), denied(access("/members")), denied(location("/users"))}
	var gaps []componentsdk.Gap
	for _, g := range res.Result.Gaps {
		if g.Code == componentsdk.CodeUnsupported || g.Code == componentsdk.CodeNotRead {
			continue
		}
		g.Detail, g.Source, g.Call = "", "", 0
		gaps = append(gaps, g)
	}
	if !reflect.DeepEqual(gaps, want) {
		t.Errorf("gaps = %+v, want %+v", gaps, want)
	}

	var out struct {
		Users       *json.RawMessage `json:"users"`
		Diagnostics Diagnostics      `json:"diagnostics"`
	}
	for _, a := range res.Artifacts {
		if a.Path == artifactPath {
			if err := json.Unmarshal(a.Data, &out); err != nil {
				t.Fatal(err)
			}
		}
	}
	if out.Users != nil || !reflect.DeepEqual(out.Diagnostics.Warnings, []string{"member:read scope missing; user metrics unavailable"}) {
		t.Errorf("users = %s, warnings = %v; want users left out and the member:read warning", ptrString(out.Users), out.Diagnostics.Warnings)
	}
}

func TestRunSkipsRollupsOverCappedInventories(t *testing.T) {
	n := maxInventorySize + 1
	monitors, detectors, members, teams := make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	for i := range n {
		monitors[i] = fmt.Sprintf(`{"id":"m%d","name":"m%d","status":"disabled","type":"cron_job","isMuted":true,"config":{"schedule_type":"crontab","schedule":"0 2 * * *"},"project":{"id":"11","slug":"billing"},"environments":[]}`, i, i)
		detectors[i] = fmt.Sprintf(`{"id":"d%d","projectId":"11","name":"d%d","type":"metric_issue","enabled":true}`, i, i)
		members[i] = fmt.Sprintf(`{"id":"%d","email":"u%d@acme.com","name":"u%d","orgRole":"member","user":{"id":"%d","isActive":true,"has2fa":true},"flags":{"sso:linked":true}}`, i, i, i, i)
		teams[i] = fmt.Sprintf(`{"id":"t%d","slug":"t%d","name":"t%d","memberCount":1}`, i, i, i)
	}
	list := func(items []string) string { return "[" + strings.Join(items, ",") + "]" }
	responses := []collectortest.Response{
		{URL: kitOrg + "/", Body: kitOrganization},
		{URL: kitOrg + "/monitors/?project=-1", Body: list(monitors)},
		{URL: kitOrg + "/detectors/?per_page=100", Body: list(detectors)},
		{URL: kitOrg + "/workflows/?per_page=100", Body: `[]`},
		{URL: kitOrg + "/projects/?per_page=100", Body: kitProjects},
		{URL: kitOrg + "/auth-provider/", Body: kitAuthProvider},
		{URL: kitOrg + "/members/", Body: list(members)},
		{URL: kitOrg + "/teams/?per_page=100", Body: list(teams)},
		{URL: kitIntegrationsURL, Body: `[]`},
		{URL: kitInstallationsURL, Body: `[]`},
	}
	config := kitConfig()
	config["level"] = "audit"
	res := runInProcess(t, config, kitSecrets(), responses)
	if res.ExitCode != 0 || res.Result == nil {
		t.Fatalf("run failed with exit %d: %s", res.ExitCode, res.Stderr)
	}
	if len(res.Result.Checks) != 0 {
		t.Errorf("checks = %+v, want none: the rollups must skip inventories marked truncated", res.Result.Checks)
	}

	type section struct {
		TotalCount         int               `json:"total_count"`
		Inventory          []json.RawMessage `json:"inventory"`
		InventoryTruncated bool              `json:"inventory_truncated"`
	}
	var out struct {
		Monitors   section `json:"monitors"`
		AlertRules section `json:"alert_rules"`
		Users      section `json:"users"`
		Teams      section `json:"teams"`
	}
	for _, a := range res.Artifacts {
		if a.Path == artifactPath {
			if err := json.Unmarshal(a.Data, &out); err != nil {
				t.Fatal(err)
			}
		}
	}
	for name, s := range map[string]section{"monitors": out.Monitors, "alert_rules": out.AlertRules, "users": out.Users, "teams": out.Teams} {
		if s.TotalCount != n || len(s.Inventory) != maxInventorySize || !s.InventoryTruncated {
			t.Errorf("%s: total_count %d, %d inventory rows, inventory_truncated %v; want %d, %d, true", name, s.TotalCount, len(s.Inventory), s.InventoryTruncated, n, maxInventorySize)
		}
	}
}

// TestTrustOutputFile writes the clean scenario's trust-level artifact to
// the file SENTRY_TRUST_OUTPUT names, for scripts/check-forbidden-data.sh.
func TestTrustOutputFile(t *testing.T) {
	path := os.Getenv("SENTRY_TRUST_OUTPUT")
	if path == "" {
		t.Skip("SENTRY_TRUST_OUTPUT is not set")
	}
	config := kitConfig()
	config["level"] = "trust"
	res := runInProcess(t, config, kitSecrets(), kitResponses())
	for _, a := range res.Artifacts {
		if a.Path == artifactPath {
			if err := os.WriteFile(path, a.Data, 0o600); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no %s in the trust run (exit %d): %s", artifactPath, res.ExitCode, res.Stderr)
}

// kitResponsesExcept returns the clean scenario's responses without those
// whose URL starts with one of prefixes.
func kitResponsesExcept(prefixes ...string) []collectortest.Response {
	var out []collectortest.Response
	for _, r := range kitResponses() {
		if !slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(r.URL, p) }) {
			out = append(out, r)
		}
	}
	return out
}

// kitForbidden is Sentry's answer to a call to url from a token without the
// scope it needs.
func kitForbidden(url string) collectortest.Response {
	return collectortest.Response{
		URL:    url,
		Status: http.StatusForbidden,
		Body:   `{"detail":"You do not have permission to perform this action."}`,
	}
}

func ptrString(raw *json.RawMessage) string {
	if raw == nil {
		return "absent"
	}
	return string(*raw)
}

func replaceResponse(responses []collectortest.Response, with collectortest.Response) []collectortest.Response {
	return replaceResponses(responses, with)
}

// replaceResponses returns responses with each one whose URL matches one of
// with replaced by it, and the rest of with added.
func replaceResponses(responses []collectortest.Response, with ...collectortest.Response) []collectortest.Response {
	out := append([]collectortest.Response(nil), responses...)
	for _, w := range with {
		found := false
		for i := range out {
			if out[i].URL == w.URL {
				out[i], found = w, true
			}
		}
		if !found {
			out = append(out, w)
		}
	}
	return out
}

// kitTrustResponses answer every call a clean trust-level run makes. A trust
// run reads no teams.
func kitTrustResponses() []collectortest.Response {
	return kitResponsesExcept(kitOrg+"/teams/", kitAPI+"/teams/")
}

func runInProcess(t *testing.T, config map[string]any, secrets map[string]string, responses []collectortest.Response) componentsdk.InProcessResult {
	t.Helper()
	replay, err := collectortest.NewReplay(collectortest.Scenario{Responses: responses})
	if err != nil {
		t.Fatal(err)
	}
	res := componentsdk.RunInProcess(Spec("test", "collectortest"), Run, componentsdk.InProcessOptions{
		Config:    config,
		Secrets:   secrets,
		Transport: replay,
		Seed:      1,
		Sleep:     func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		Clock:     func() time.Time { return goldenTime },
	})
	if unmatched := replay.Unmatched(); len(unmatched) > 0 {
		t.Errorf("requests with no canned response: %v", unmatched)
	}
	return res
}
