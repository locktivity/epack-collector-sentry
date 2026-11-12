package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/locktivity/epack-collector-sentry/internal/sentry"
	"github.com/locktivity/epack/componentsdk"
)

// accessFake is richFake with an organization that requires 2FA, two
// integrations, and two Sentry app installations.
func accessFake() *fakeSentryAPI {
	fake := richFake()
	fake.org = &sentry.Organization{ID: "1", Slug: "acme-corp", Name: "Acme Corp", Require2FA: new(true), OpenMembership: new(false)}
	fake.integrations = []sentry.Integration{
		{ID: "301", Name: "Acme Workspace", Provider: sentry.IntegrationProvider{Key: "slack"}, OrganizationIntegrationStatus: "active"},
		{ID: "303", Name: "acme.atlassian.net", Provider: sentry.IntegrationProvider{Key: "jira"}, OrganizationIntegrationStatus: "disabled"},
	}
	fake.installs = []sentry.SentryAppInstallation{
		{UUID: "i1", Status: "installed", App: sentry.SentryAppInstallationApp{Slug: "acme-deploy-bot"}},
		{UUID: "i2", Status: "pending", App: sentry.SentryAppInstallationApp{Slug: "linear"}},
	}
	return fake
}

// collectAccess runs c's collection with the access artifact selected and
// returns that artifact as JSON.
func collectAccess(t *testing.T, fake *fakeSentryAPI, level Level) (*Collector, map[string]any) {
	t.Helper()
	c := testCollector(fake, level)
	c.collectAccess = true
	if _, err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	data, err := json.Marshal(c.AccessPosture())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return c, m
}

// section returns the object at key in m.
func section(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	s, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("%s is %v, want an object", key, m[key])
	}
	return s
}

// accessGap returns the gap c recorded at pointer in the access artifact.
func accessGap(c *Collector, pointer string) (componentsdk.Gap, bool) {
	for _, g := range c.rec.Gaps() {
		if g.Path == accessPosturePath && g.Pointer == pointer {
			return g, true
		}
	}
	return componentsdk.Gap{}, false
}

func expectAccessGap(t *testing.T, c *Collector, pointer string, code componentsdk.Code) {
	t.Helper()
	g, ok := accessGap(c, pointer)
	if !ok {
		t.Fatalf("no gap at %s; gaps: %+v", pointer, c.rec.Gaps())
	}
	if g.Code != code {
		t.Errorf("gap at %s has code %s, want %s", pointer, g.Code, code)
	}
}

func member(role string, linked, mfa bool, lastActive string) sentry.Member {
	return sentry.Member{
		OrgRole: role,
		User:    &sentry.MemberUser{IsActive: true, Has2fa: mfa, LastActive: lastActive},
		Flags:   sentry.MemberFlags{SSOLinked: linked},
	}
}

func TestTallyMembers(t *testing.T) {
	recent := "2026-06-25T11:30:00.123456Z"
	deactivated := member("member", false, true, recent)
	deactivated.User.IsActive = false
	members := []sentry.Member{
		member("owner", false, true, recent),
		member("manager", false, false, recent),
		member("manager", true, false, recent),
		member("admin", false, true, recent),
		member("member", true, true, "2026-01-01T00:00:00Z"),
		member("member", false, false, ""),
		deactivated,
		{OrgRole: "owner", Pending: true},
	}
	got := tallyMembers(members, true, false, goldenTime)
	want := memberTally{
		members: 7, sso: 2, local: 5, localMFA: 3, inactive: 3,
		admins: 3, ssoAdmins: 1, localAdmins: 2, localMFAAdmins: 1,
	}
	if got != want {
		t.Errorf("tallyMembers() = %+v, want %+v", got, want)
	}
}

// With Require SSO off, a member linked to SSO can still sign in with a
// password, so every member counts as local.
func TestTallyMembersCountsSSOLinkedMembersAsLocalUnlessSSOIsRequired(t *testing.T) {
	recent := "2026-06-25T11:30:00Z"
	members := []sentry.Member{
		member("owner", true, true, recent),
		member("manager", true, false, recent),
		member("member", true, false, recent),
		member("member", false, true, recent),
	}
	if got, want := tallyMembers(members, true, false, goldenTime), (memberTally{members: 4, sso: 3, local: 1, localMFA: 1, admins: 2, ssoAdmins: 2}); got != want {
		t.Errorf("Require SSO on: tallyMembers() = %+v, want %+v", got, want)
	}
	want := memberTally{members: 4, sso: 3, local: 4, localMFA: 2, admins: 2, ssoAdmins: 2, localAdmins: 2, localMFAAdmins: 1}
	if got := tallyMembers(members, false, false, goldenTime); got != want {
		t.Errorf("Require SSO off: tallyMembers() = %+v, want %+v", got, want)
	}
}

// With Open Membership on, a member with the admin role can join any team
// and delete its projects, so they count as an admin.
func TestTallyMembersCountsTheAdminRoleWithOpenMembership(t *testing.T) {
	recent := "2026-06-25T11:30:00Z"
	members := []sentry.Member{
		member("owner", true, true, recent),
		member("admin", false, false, recent),
		member("member", false, true, recent),
	}
	if got, want := tallyMembers(members, true, false, goldenTime), (memberTally{members: 3, sso: 1, local: 2, localMFA: 1, admins: 1, ssoAdmins: 1}); got != want {
		t.Errorf("Open Membership off: tallyMembers() = %+v, want %+v", got, want)
	}
	want := memberTally{members: 3, sso: 1, local: 2, localMFA: 1, admins: 2, ssoAdmins: 1, localAdmins: 1}
	if got := tallyMembers(members, true, true, goldenTime); got != want {
		t.Errorf("Open Membership on: tallyMembers() = %+v, want %+v", got, want)
	}
}

func TestInactive(t *testing.T) {
	cutoff := goldenTime.Add(-inactiveAfter)
	cases := []struct {
		name string
		user *sentry.MemberUser
		want bool
	}{
		{"active this week", &sentry.MemberUser{IsActive: true, LastActive: "2026-06-25T11:30:00Z"}, false},
		{"active exactly 90 days ago", &sentry.MemberUser{IsActive: true, LastActive: cutoff.Format(time.RFC3339)}, false},
		{"active 91 days ago", &sentry.MemberUser{IsActive: true, LastActive: cutoff.Add(-24 * time.Hour).Format(time.RFC3339)}, true},
		{"never active", &sentry.MemberUser{IsActive: true}, true},
		{"deactivated account", &sentry.MemberUser{IsActive: false, LastActive: "2026-06-25T11:30:00Z"}, true},
		{"no account", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := inactive(tc.user, goldenTime); got != tc.want {
				t.Errorf("inactive() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAccessPeopleWritesCountsFromAudit(t *testing.T) {
	tally := memberTally{members: 3, sso: 2, local: 1, localMFA: 0, inactive: 1, admins: 1, localAdmins: 1, localMFAAdmins: 0}

	m, a := accessPeople(tally, LevelTrust)
	if m.MemberCount != nil || m.SSOMemberCount != nil || m.LocalMemberCount != nil || m.LocalMFAMemberCount != nil || m.InactiveMemberCount != nil {
		t.Errorf("trust members = %+v, want no counts", m)
	}
	if a.AdminCount != nil || a.LocalAdminCount != nil || a.LocalMFAAdminCount != nil {
		t.Errorf("trust admins = %+v, want no counts", a)
	}
	if *m.SSOCoveragePct.Value != 66 || *m.InactivePct.Value != 34 || *a.AdminSharePct.Value != 34 || *a.LocalAdminMFACoveragePct.Value != 0 {
		t.Errorf("percentages = %d, %d, %d, %d; want 66 rounded down, 34 and 34 rounded up, and 0",
			*m.SSOCoveragePct.Value, *m.InactivePct.Value, *a.AdminSharePct.Value, *a.LocalAdminMFACoveragePct.Value)
	}

	m, a = accessPeople(tally, LevelAudit)
	if *m.MemberCount != 3 || *m.SSOMemberCount != 2 || *m.LocalMemberCount != 1 || *m.LocalMFAMemberCount != 0 || *m.InactiveMemberCount != 1 {
		t.Errorf("audit members = %+v", m)
	}
	if *a.AdminCount != 1 || *a.LocalAdminCount != 1 || *a.LocalMFAAdminCount != 0 {
		t.Errorf("audit admins = %+v", a)
	}
}

func TestAccessPeopleOverNoMembers(t *testing.T) {
	m, a := accessPeople(memberTally{}, LevelAudit)
	data, err := json.Marshal(struct {
		Members *AccessMembers `json:"members"`
		Admins  *AccessAdmins  `json:"admins"`
	}{m, a})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"members":{"members_present":false,"member_count":0,"sso_member_count":0,"sso_coverage_pct":null,` +
		`"local_members_present":false,"local_member_count":0,"local_mfa_member_count":0,"local_mfa_coverage_pct":null,` +
		`"inactive_member_count":0,"inactive_pct":null},` +
		`"admins":{"admins_present":false,"admin_count":0,"admin_share_pct":null,"local_admins_present":false,` +
		`"local_admin_count":0,"local_mfa_admin_count":0,"local_admin_mfa_coverage_pct":null}}`
	if string(data) != want {
		t.Errorf("got  %s\nwant %s", data, want)
	}
}

// The first App access check reads sso_required and mfa_required.
func TestAccessSignIn(t *testing.T) {
	c, m := collectAccess(t, accessFake(), LevelTrust)
	want := map[string]any{"sso_configured": true, "sso_required": true, "mfa_required": true, "scim_enabled": true}
	if got := section(t, m, "sign_in"); !reflect.DeepEqual(got, want) {
		t.Errorf("sign_in = %v, want %v", got, want)
	}
	if len(c.rec.Gaps()) != 3 {
		t.Errorf("gaps = %+v, want only the external, root, and credentials gaps", c.rec.Gaps())
	}
}

func TestAccessSignInWithoutSSO(t *testing.T) {
	fake := accessFake()
	fake.authProvider = nil
	fake.org.Require2FA = new(false)
	_, m := collectAccess(t, fake, LevelTrust)
	want := map[string]any{"sso_configured": false, "sso_required": false, "mfa_required": false, "scim_enabled": false}
	if got := section(t, m, "sign_in"); !reflect.DeepEqual(got, want) {
		t.Errorf("sign_in = %v, want %v", got, want)
	}
}

func TestAccessSignInCopiesTheAuthProviderGap(t *testing.T) {
	fake := accessFake()
	fake.authProvider = nil
	fake.authProviderErr = &sentry.APIError{StatusCode: http.StatusForbidden}
	c, m := collectAccess(t, fake, LevelTrust)
	if got := section(t, m, "sign_in"); !reflect.DeepEqual(got, map[string]any{"mfa_required": true}) {
		t.Errorf("sign_in = %v, want only mfa_required", got)
	}
	for _, field := range []string{"sso_configured", "sso_required", "scim_enabled"} {
		expectAccessGap(t, c, "/sign_in/"+field, componentsdk.CodePermissionDenied)
	}
}

func TestAccessSignInWithoutRequire2FA(t *testing.T) {
	fake := accessFake()
	fake.org.Require2FA = nil
	c, m := collectAccess(t, fake, LevelTrust)
	if _, ok := section(t, m, "sign_in")["mfa_required"]; ok {
		t.Error("mfa_required is written, want it left out")
	}
	expectAccessGap(t, c, "/sign_in/mfa_required", componentsdk.CodeInvalidResponse)
}

// The second App access check reads local_admin_mfa_coverage_pct over
// local_admins_present.
func TestAccessAdminMFA(t *testing.T) {
	fake := accessFake()
	fake.members = []sentry.Member{
		member("owner", false, true, "2026-06-25T11:30:00Z"),
		member("manager", false, false, "2026-06-25T11:30:00Z"),
		member("admin", false, false, "2026-06-25T11:30:00Z"),
	}
	_, m := collectAccess(t, fake, LevelTrust)
	admins := section(t, m, "admins")
	if admins["local_admins_present"] != true || admins["local_admin_mfa_coverage_pct"] != float64(50) {
		t.Errorf("admins = %v, want local admins present at 50%% MFA", admins)
	}

	fake.members = []sentry.Member{member("owner", true, false, "2026-06-25T11:30:00Z")}
	_, m = collectAccess(t, fake, LevelTrust)
	admins = section(t, m, "admins")
	if v, ok := admins["local_admin_mfa_coverage_pct"]; admins["local_admins_present"] != false || !ok || v != nil {
		t.Errorf("admins = %v, want no local admins and a null coverage", admins)
	}
}

// An owner linked to SSO still counts as a local admin when Require SSO is
// off.
func TestAccessAdminMFAWithoutRequireSSO(t *testing.T) {
	fake := accessFake()
	fake.authProvider.RequireLink = false
	fake.members = []sentry.Member{member("owner", true, false, "2026-06-25T11:30:00Z")}
	_, m := collectAccess(t, fake, LevelTrust)
	admins := section(t, m, "admins")
	if admins["local_admins_present"] != true || admins["local_admin_mfa_coverage_pct"] != float64(0) {
		t.Errorf("admins = %v, want a local admin at 0%% MFA", admins)
	}
}

// Without the Require SSO setting, a member linked to SSO may or may not be
// local. The local values that depend on such a member are left out with the
// read's gap. A member not linked to SSO is local either way.
func TestAccessLocalValuesWithoutTheSSOSettings(t *testing.T) {
	recent := "2026-06-25T11:30:00Z"
	fake := accessFake()
	fake.authProvider = nil
	fake.authProviderErr = &sentry.APIError{StatusCode: http.StatusInternalServerError}
	fake.members = []sentry.Member{
		member("owner", true, true, recent),
		member("member", false, false, recent),
	}
	c, m := collectAccess(t, fake, LevelAudit)
	members, admins := section(t, m, "members"), section(t, m, "admins")
	want := map[string]any{"members_present": true, "member_count": float64(2), "sso_member_count": float64(1), "sso_coverage_pct": float64(50),
		"local_members_present": true, "inactive_member_count": float64(0), "inactive_pct": float64(0)}
	if !reflect.DeepEqual(members, want) {
		t.Errorf("members = %v, want %v", members, want)
	}
	if want := map[string]any{"admins_present": true, "admin_count": float64(1), "admin_share_pct": float64(50)}; !reflect.DeepEqual(admins, want) {
		t.Errorf("admins = %v, want %v", admins, want)
	}
	for _, pointer := range []string{
		"/members/local_member_count", "/members/local_mfa_member_count", "/members/local_mfa_coverage_pct",
		"/admins/local_admins_present", "/admins/local_admin_count", "/admins/local_mfa_admin_count", "/admins/local_admin_mfa_coverage_pct",
	} {
		expectAccessGap(t, c, pointer, componentsdk.CodeVendorError)
	}

	// With no member linked to SSO, every member is local whatever the
	// setting.
	fake.members = []sentry.Member{member("owner", false, true, recent), member("member", false, false, recent)}
	c, m = collectAccess(t, fake, LevelAudit)
	members, admins = section(t, m, "members"), section(t, m, "admins")
	if members["local_member_count"] != float64(2) || members["local_mfa_coverage_pct"] != float64(50) || admins["local_admin_mfa_coverage_pct"] != float64(100) {
		t.Errorf("members = %v, admins = %v; want every member local", members, admins)
	}
	if g, ok := accessGap(c, "/members/local_member_count"); ok {
		t.Errorf("gap %+v, want none on the local values", g)
	}
}

// The third App access check reads never_expiring_pct over
// credentials_present. Sentry lists organization auth tokens only for a
// browser session or an OAuth app token, so the whole credentials section is
// left out as not read. Sentry has no root account and no external members.
func TestAccessLeftOutValues(t *testing.T) {
	c, m := collectAccess(t, accessFake(), LevelAudit)
	for key, code := range map[string]componentsdk.Code{"credentials": componentsdk.CodeNotRead, "root": componentsdk.CodeUnsupported} {
		if _, ok := m[key]; ok {
			t.Errorf("%s is written, want it left out", key)
		}
		expectAccessGap(t, c, "/"+key, code)
	}
	expectAccessGap(t, c, "/members/external_members_present", componentsdk.CodeUnsupported)
	expectAccessGap(t, c, "/members/external_member_count", componentsdk.CodeUnsupported)
}

func TestAccessExternalCountGapOnlyFromAudit(t *testing.T) {
	c, _ := collectAccess(t, accessFake(), LevelTrust)
	if g, ok := accessGap(c, "/members/external_member_count"); ok {
		t.Errorf("trust run has gap %+v on an audit-level count", g)
	}
	expectAccessGap(t, c, "/members/external_members_present", componentsdk.CodeUnsupported)
}

// The fourth App access check reads inactive_pct over members_present.
func TestAccessInactiveMembers(t *testing.T) {
	fake := accessFake()
	fake.members = []sentry.Member{
		member("member", true, true, "2026-06-25T11:30:00Z"),
		member("member", true, true, "2026-03-01T00:00:00Z"),
		member("member", true, true, ""),
		{OrgRole: "member", Pending: true},
	}
	_, m := collectAccess(t, fake, LevelAudit)
	members := section(t, m, "members")
	if members["members_present"] != true || members["member_count"] != float64(3) ||
		members["inactive_member_count"] != float64(2) || members["inactive_pct"] != float64(67) {
		t.Errorf("members = %v, want 2 of 3 inactive, rounded up to 67%%", members)
	}
}

func TestAccessMembersReadFails(t *testing.T) {
	fake := accessFake()
	fake.membersErr = &sentry.APIError{StatusCode: http.StatusForbidden}
	c, m := collectAccess(t, fake, LevelTrust)
	for _, key := range []string{"members", "admins"} {
		if _, ok := m[key]; ok {
			t.Errorf("%s is written, want it left out", key)
		}
		expectAccessGap(t, c, "/"+key, componentsdk.CodePermissionDenied)
	}
}

func TestAccessConnectedApps(t *testing.T) {
	_, m := collectAccess(t, accessFake(), LevelAudit)
	if got := section(t, m, "connected_apps"); !reflect.DeepEqual(got, map[string]any{"apps_present": true, "app_count": float64(2)}) {
		t.Errorf("connected_apps = %v, want the active integration and the installed app", got)
	}

	fake := accessFake()
	fake.integrations[0].OrganizationIntegrationStatus = "disabled"
	fake.installs[0].Status = "pending_deletion"
	_, m = collectAccess(t, fake, LevelAudit)
	if got := section(t, m, "connected_apps"); !reflect.DeepEqual(got, map[string]any{"apps_present": false, "app_count": float64(0)}) {
		t.Errorf("connected_apps = %v, want none present", got)
	}
}

func TestAccessConnectedAppsWithOneListUnread(t *testing.T) {
	fake := accessFake()
	fake.integrationsErr = &sentry.APIError{StatusCode: http.StatusInternalServerError}
	c, m := collectAccess(t, fake, LevelAudit)
	if got := section(t, m, "connected_apps"); !reflect.DeepEqual(got, map[string]any{"apps_present": true}) {
		t.Errorf("connected_apps = %v, want apps_present from the installed app alone", got)
	}
	expectAccessGap(t, c, "/connected_apps/app_count", componentsdk.CodeVendorError)

	fake.installs = nil
	c, m = collectAccess(t, fake, LevelAudit)
	if got := section(t, m, "connected_apps"); len(got) != 0 {
		t.Errorf("connected_apps = %v, want both values left out", got)
	}
	expectAccessGap(t, c, "/connected_apps/apps_present", componentsdk.CodeVendorError)
	expectAccessGap(t, c, "/connected_apps/app_count", componentsdk.CodeVendorError)
}

func TestAccessLeftOutReadsNoMembersAtTrust(t *testing.T) {
	fake := accessFake()
	fake.membersErr = &sentry.APIError{StatusCode: http.StatusForbidden}
	fake.integrationsErr = &sentry.APIError{StatusCode: http.StatusForbidden}
	fake.installsErr = &sentry.APIError{StatusCode: http.StatusForbidden}
	c := testCollector(fake, LevelTrust)
	if _, err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if c.AccessPosture() != nil {
		t.Error("AccessPosture() is set, want nil when the run does not collect it")
	}
	if gaps := c.rec.Gaps(); len(gaps) != 0 {
		t.Errorf("gaps = %+v, want none from reads the run skips", gaps)
	}
}
