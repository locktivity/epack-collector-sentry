package collector

import (
	"context"
	"encoding/json"
	"time"

	"github.com/locktivity/epack-collector-sentry/internal/sentry"
	"github.com/locktivity/epack/componentsdk"
)

// accessPostureSchema is the schema ID of the normalized access artifact.
const accessPostureSchema = "evidencepack/access-posture@v1"

// accessPosturePath is where the access artifact sits in the pack.
var accessPosturePath = componentsdk.ArtifactPath("sentry", accessPostureSchema)

// inactiveAfter is how long a member can go without activity before they
// count as inactive.
const inactiveAfter = 90 * 24 * time.Hour

// AccessPosture is the evidencepack/access-posture@v1 artifact for the
// organization. A run reads one organization, so the artifact has no scopes.
// The counts are written from the audit level, as the schema marks them, and
// the present flags and percentages at every level. A percentage over an
// empty population is null.
type AccessPosture struct {
	SchemaVersion string               `json:"schema_version"`
	CollectedAt   string               `json:"collected_at"`
	Provider      string               `json:"provider"`
	SystemType    string               `json:"system_type"`
	SignIn        *AccessSignIn        `json:"sign_in,omitempty"`
	Members       *AccessMembers       `json:"members,omitempty"`
	Admins        *AccessAdmins        `json:"admins,omitempty"`
	ConnectedApps *AccessConnectedApps `json:"connected_apps,omitempty"`
}

// AccessSignIn holds the organization's sign-in rules. The SSO values are
// copies of the detailed artifact's auth values.
type AccessSignIn struct {
	SSOConfigured *bool `json:"sso_configured,omitempty"`
	SSORequired   *bool `json:"sso_required,omitempty"`
	MFARequired   *bool `json:"mfa_required,omitempty"`
	SCIMEnabled   *bool `json:"scim_enabled,omitempty"`
}

// AccessMembers counts the organization's members. Sentry has no external
// member type, so the external values are left out with an unsupported gap.
type AccessMembers struct {
	MembersPresent      *bool      `json:"members_present,omitempty"`
	MemberCount         *int       `json:"member_count,omitempty"`
	SSOMemberCount      *int       `json:"sso_member_count,omitempty"`
	SSOCoveragePct      Percentage `json:"sso_coverage_pct,omitzero"`
	LocalMembersPresent *bool      `json:"local_members_present,omitempty"`
	LocalMemberCount    *int       `json:"local_member_count,omitempty"`
	LocalMFAMemberCount *int       `json:"local_mfa_member_count,omitempty"`
	LocalMFACoveragePct Percentage `json:"local_mfa_coverage_pct,omitzero"`
	InactiveMemberCount *int       `json:"inactive_member_count,omitempty"`
	InactivePct         Percentage `json:"inactive_pct,omitzero"`
}

// AccessAdmins counts the members with the owner or manager role, and with
// Open Membership on, the members with the admin role.
type AccessAdmins struct {
	AdminsPresent            *bool      `json:"admins_present,omitempty"`
	AdminCount               *int       `json:"admin_count,omitempty"`
	AdminSharePct            Percentage `json:"admin_share_pct,omitzero"`
	LocalAdminsPresent       *bool      `json:"local_admins_present,omitempty"`
	LocalAdminCount          *int       `json:"local_admin_count,omitempty"`
	LocalMFAAdminCount       *int       `json:"local_mfa_admin_count,omitempty"`
	LocalAdminMFACoveragePct Percentage `json:"local_admin_mfa_coverage_pct,omitzero"`
}

// AccessConnectedApps counts the integrations and Sentry apps installed in
// the organization.
type AccessConnectedApps struct {
	AppsPresent *bool `json:"apps_present,omitempty"`
	AppCount    *int  `json:"app_count,omitempty"`
}

// Percentage is a percentage in the access artifact. It is left out when
// Read is false. Otherwise it is written as Value, which is nil over an
// empty population.
type Percentage struct {
	Value *int
	Read  bool
}

// IsZero reports whether the percentage was not read, for omitzero.
func (p Percentage) IsZero() bool { return !p.Read }

// MarshalJSON writes the value, or null.
func (p Percentage) MarshalJSON() ([]byte, error) { return json.Marshal(p.Value) }

func percent(count, total int, rounding componentsdk.Rounding) Percentage {
	return Percentage{Value: componentsdk.Percent(count, total, rounding), Read: true}
}

// accessLeftOut lists the values left out of every run's access artifact,
// with their code and reason. Sentry has no data for the unsupported ones.
// The credentials are not read, because Sentry lists organization auth
// tokens only for a browser session or an OAuth app token. The counts are
// audit-level fields.
var accessLeftOut = []struct {
	pointer string
	code    componentsdk.Code
	reason  string
	audit   bool
}{
	{"/credentials", componentsdk.CodeNotRead, credentialsNotRead, false},
	{"/members/external_member_count", componentsdk.CodeUnsupported, noExternalMembers, true},
	{"/members/external_members_present", componentsdk.CodeUnsupported, noExternalMembers, false},
	{"/root", componentsdk.CodeUnsupported, noRootAccount, false},
}

const (
	noExternalMembers  = "Sentry has no guest or external member type."
	noRootAccount      = "Sentry has no built-in root account."
	credentialsNotRead = "Sentry's org auth tokens API lists organization auth tokens only for a browser session or an OAuth app token with org:tokens, and this collector signs in with a personal or internal integration token."

	openMembershipMissing = "The organization's details have no openMembership value, so it is unknown whether members with the admin role can join any team and delete its projects."
)

// accessPosture builds the access artifact, collected at collectedAt, from
// the organization, the detailed artifact's auth summary read with authErr,
// and the members read with membersErr. It reads the installed integrations
// and Sentry apps itself.
func (c *Collector) accessPosture(ctx context.Context, collectedAt string, org *sentry.Organization, auth AuthSummary, authErr error, members []sentry.Member, membersErr error) *AccessPosture {
	a := &AccessPosture{
		SchemaVersion: "1.0.0",
		CollectedAt:   collectedAt,
		Provider:      "sentry",
		SystemType:    "monitoring",
		SignIn:        c.accessSignIn(org, auth),
	}
	if membersErr != nil {
		c.rec.NotCollected(accessLocation("/members"), membersErr)
		c.rec.NotCollected(accessLocation("/admins"), membersErr)
	} else {
		required := auth.SSORequired != nil && *auth.SSORequired
		open := org.OpenMembership != nil && *org.OpenMembership
		t := tallyMembers(members, required, open, c.clock.Now())
		a.Members, a.Admins = accessPeople(t, c.level)
		if org.OpenMembership == nil && hasAdminRole(members) {
			a.Admins = nil
			c.rec.NotCollectedCode(accessLocation("/admins"), componentsdk.CodeInvalidResponse, openMembershipMissing)
		}
		if auth.SSORequired == nil {
			c.leaveOutLocal(a.Members, a.Admins, t, authErr)
		}
	}
	a.ConnectedApps = c.connectedApps(ctx)
	for _, u := range accessLeftOut {
		if u.audit && !c.level.AtLeast(LevelAudit) {
			continue
		}
		c.rec.NotCollectedCode(accessLocation(u.pointer), u.code, u.reason)
	}
	return a
}

// accessSignIn copies the SSO values from the detailed artifact's auth
// summary, and reads mfa_required from the organization's require2FA.
func (c *Collector) accessSignIn(org *sentry.Organization, auth AuthSummary) *AccessSignIn {
	for _, field := range []string{"sso_configured", "sso_required", "scim_enabled"} {
		c.rec.Mirror(location("/auth/"+field), accessLocation("/sign_in/"+field))
	}
	if org.Require2FA == nil {
		c.rec.NotCollectedCode(accessLocation("/sign_in/mfa_required"), componentsdk.CodeInvalidResponse, "The organization's details have no require2FA value.")
	}
	return &AccessSignIn{
		SSOConfigured: auth.SSOConfigured,
		SSORequired:   auth.SSORequired,
		MFARequired:   org.Require2FA,
		SCIMEnabled:   auth.SCIMEnabled,
	}
}

// memberTally counts the members and admins for the access artifact. A
// local member is one who can sign in without SSO.
type memberTally struct {
	members, sso, local, localMFA, inactive        int
	admins, ssoAdmins, localAdmins, localMFAAdmins int
}

// tallyMembers counts members as of now, with ssoRequired as the Require
// SSO setting and open as the Open Membership setting. A pending invitation
// is not a member yet, so it is not counted. An admin has the owner or
// manager role. With Open Membership on, a member with the admin role is an
// admin too, since they can join any team and delete its projects. A member
// is local when not linked to SSO. A member linked to SSO is local too
// unless Require SSO is on, since they can still sign in with a password. A
// member is inactive when their Sentry account is deactivated or they were
// last active more than 90 days before now, or never.
func tallyMembers(members []sentry.Member, ssoRequired, open bool, now time.Time) memberTally {
	var t memberTally
	for _, m := range members {
		if m.Pending {
			continue
		}
		t.members++
		admin := m.OrgRole == "owner" || m.OrgRole == "manager" || (open && m.OrgRole == "admin")
		mfa := m.User != nil && m.User.Has2fa
		if admin {
			t.admins++
		}
		if m.Flags.SSOLinked {
			t.sso++
			if admin {
				t.ssoAdmins++
			}
		}
		if !m.Flags.SSOLinked || !ssoRequired {
			t.local++
			if mfa {
				t.localMFA++
			}
			if admin {
				t.localAdmins++
				if mfa {
					t.localMFAAdmins++
				}
			}
		}
		if inactive(m.User, now) {
			t.inactive++
		}
	}
	return t
}

// hasAdminRole reports whether a member who is not a pending invitation has
// the admin role.
func hasAdminRole(members []sentry.Member) bool {
	for _, m := range members {
		if !m.Pending && m.OrgRole == "admin" {
			return true
		}
	}
	return false
}

// inactive reports whether a member's Sentry account u is deactivated, or
// has not been active in the 90 days before now. An account whose last
// active time is missing or does not parse is inactive too.
func inactive(u *sentry.MemberUser, now time.Time) bool {
	if u == nil || !u.IsActive {
		return true
	}
	last, err := time.Parse(time.RFC3339Nano, u.LastActive)
	if err != nil {
		return true
	}
	return last.Before(now.Add(-inactiveAfter))
}

// accessPeople writes t as the members and admins sections at level.
func accessPeople(t memberTally, level Level) (*AccessMembers, *AccessAdmins) {
	m := &AccessMembers{
		MembersPresent:      boolPtr(t.members > 0),
		SSOCoveragePct:      percent(t.sso, t.members, componentsdk.RoundDown),
		LocalMembersPresent: boolPtr(t.local > 0),
		LocalMFACoveragePct: percent(t.localMFA, t.local, componentsdk.RoundDown),
		InactivePct:         percent(t.inactive, t.members, componentsdk.RoundUp),
	}
	a := &AccessAdmins{
		AdminsPresent:            boolPtr(t.admins > 0),
		AdminSharePct:            percent(t.admins, t.members, componentsdk.RoundUp),
		LocalAdminsPresent:       boolPtr(t.localAdmins > 0),
		LocalAdminMFACoveragePct: percent(t.localMFAAdmins, t.localAdmins, componentsdk.RoundDown),
	}
	if level.AtLeast(LevelAudit) {
		m.MemberCount, m.SSOMemberCount = intPtr(t.members), intPtr(t.sso)
		m.LocalMemberCount, m.LocalMFAMemberCount = intPtr(t.local), intPtr(t.localMFA)
		m.InactiveMemberCount = intPtr(t.inactive)
		a.AdminCount, a.LocalAdminCount, a.LocalMFAAdminCount = intPtr(t.admins), intPtr(t.localAdmins), intPtr(t.localMFAAdmins)
	}
	return m, a
}

// leaveOutLocal leaves out the local values in m and a that depend on the
// Require SSO setting, which could not be read because of err. a is nil when
// the admins were left out already. t is the
// tally taken with the setting off. A member linked to SSO is then local
// only if the setting is off. A member not linked to SSO is local either
// way, so a present flag stays when such a member is there.
func (c *Collector) leaveOutLocal(m *AccessMembers, a *AccessAdmins, t memberTally, err error) {
	var pointers []string
	if t.sso > 0 {
		m.LocalMemberCount, m.LocalMFAMemberCount, m.LocalMFACoveragePct = nil, nil, Percentage{}
		pointers = append(pointers, "/members/local_mfa_coverage_pct")
		if c.level.AtLeast(LevelAudit) {
			pointers = append(pointers, "/members/local_member_count", "/members/local_mfa_member_count")
		}
		if t.members == t.sso {
			m.LocalMembersPresent = nil
			pointers = append(pointers, "/members/local_members_present")
		}
	}
	if a != nil && t.ssoAdmins > 0 {
		a.LocalAdminCount, a.LocalMFAAdminCount, a.LocalAdminMFACoveragePct = nil, nil, Percentage{}
		pointers = append(pointers, "/admins/local_admin_mfa_coverage_pct")
		if c.level.AtLeast(LevelAudit) {
			pointers = append(pointers, "/admins/local_admin_count", "/admins/local_mfa_admin_count")
		}
		if t.admins == t.ssoAdmins {
			a.LocalAdminsPresent = nil
			pointers = append(pointers, "/admins/local_admins_present")
		}
	}
	for _, pointer := range pointers {
		c.rec.NotCollected(accessLocation(pointer), err)
	}
}

// connectedApps counts the active integrations and the installed Sentry
// apps. An entry in either list makes apps_present true, even when the
// other list could not be read.
func (c *Collector) connectedApps(ctx context.Context) *AccessConnectedApps {
	integrations, integrationsErr := c.client.ListIntegrations(ctx)
	installations, installationsErr := c.client.ListSentryAppInstallations(ctx)
	count := 0
	for _, i := range integrations {
		if i.OrganizationIntegrationStatus == "active" {
			count++
		}
	}
	for _, i := range installations {
		if i.Status == "installed" {
			count++
		}
	}
	apps := &AccessConnectedApps{}
	audit := c.level.AtLeast(LevelAudit)
	err := firstError(integrationsErr, installationsErr)
	if err == nil {
		apps.AppsPresent = boolPtr(count > 0)
		if audit {
			apps.AppCount = intPtr(count)
		}
		return apps
	}
	if count > 0 {
		apps.AppsPresent = boolPtr(true)
	} else {
		c.rec.NotCollected(accessLocation("/connected_apps/apps_present"), err)
	}
	if audit {
		c.rec.NotCollected(accessLocation("/connected_apps/app_count"), err)
	}
	return apps
}

func accessLocation(pointer string) componentsdk.Location {
	return componentsdk.Location{Path: accessPosturePath, Pointer: pointer}
}

func boolPtr(b bool) *bool { return &b }

func intPtr(n int) *int { return &n }
