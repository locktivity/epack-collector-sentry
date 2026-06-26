package collector

import (
	"testing"

	"github.com/locktivity/epack-collector-sentry/internal/sentry"
)

func TestComputeUserMetrics_Empty(t *testing.T) {
	m, trunc := computeUserMetrics(nil, LevelAudit, nil)
	if m.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", m.TotalCount)
	}
	if m.Has2FAPct != nil {
		t.Errorf("Has2FAPct = %v, want nil", m.Has2FAPct)
	}
	if trunc != nil {
		t.Errorf("truncation = %v, want nil", trunc)
	}
}

func TestComputeUserMetrics_Mixed(t *testing.T) {
	members := []sentry.Member{
		{
			ID: "1", Email: "admin@acme.com", Name: "Admin",
			OrgRole: "admin",
			User:    &sentry.MemberUser{IsActive: true, Has2fa: true},
			Flags:   sentry.MemberFlags{SSOLinked: true},
		},
		{
			ID: "2", Email: "dev@acme.com", Name: "Dev",
			OrgRole: "member",
			User:    &sentry.MemberUser{IsActive: true, Has2fa: false},
			Flags:   sentry.MemberFlags{SSOLinked: true},
		},
		{
			ID: "3", Email: "pending@acme.com", Name: "Pending",
			OrgRole: "member", Pending: true,
			User:  nil,
			Flags: sentry.MemberFlags{SSOLinked: false},
		},
	}

	m, _ := computeUserMetrics(members, LevelAudit, nil)

	if m.TotalCount != 3 {
		t.Errorf("TotalCount = %d, want 3", m.TotalCount)
	}
	if m.ActiveCount != 2 {
		t.Errorf("ActiveCount = %d, want 2", m.ActiveCount)
	}
	if m.PendingCount != 1 {
		t.Errorf("PendingCount = %d, want 1", m.PendingCount)
	}
	if m.Has2FACount != 1 {
		t.Errorf("Has2FACount = %d, want 1", m.Has2FACount)
	}
	if m.Has2FAPct == nil || *m.Has2FAPct != 33 {
		t.Errorf("Has2FAPct = %v, want 33", m.Has2FAPct)
	}
	if m.SSOLinkedCount != 2 {
		t.Errorf("SSOLinkedCount = %d, want 2", m.SSOLinkedCount)
	}
	if m.SSOLinkedPct == nil || *m.SSOLinkedPct != 66 {
		t.Errorf("SSOLinkedPct = %v, want 66", m.SSOLinkedPct)
	}
}

func TestComputeUserMetrics_TrustOmitsInventory(t *testing.T) {
	members := []sentry.Member{
		{ID: "1", User: &sentry.MemberUser{IsActive: true}},
	}

	m, _ := computeUserMetrics(members, LevelTrust, nil)

	if m.Inventory != nil {
		t.Errorf("expected nil inventory at trust, got %d items", len(m.Inventory))
	}
	if m.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1 (counts still computed)", m.TotalCount)
	}
}

func TestComputeUserMetrics_AuditIncludesInventory(t *testing.T) {
	members := []sentry.Member{
		{
			ID: "u1", Email: "jane@acme.com", Name: "Jane",
			OrgRole: "admin", DateCreated: "2024-01-15T09:00:00Z",
			User:  &sentry.MemberUser{IsActive: true, Has2fa: true, LastActive: "2026-06-05T11:30:00Z"},
			Flags: sentry.MemberFlags{SSOLinked: true},
		},
	}

	m, _ := computeUserMetrics(members, LevelAudit, nil)

	if len(m.Inventory) != 1 {
		t.Fatalf("expected 1 inventory item, got %d", len(m.Inventory))
	}
	inv := m.Inventory[0]
	if inv.Email != "jane@acme.com" {
		t.Errorf("Email = %q, want %q", inv.Email, "jane@acme.com")
	}
	if inv.OrgRole != "admin" {
		t.Errorf("OrgRole = %q, want %q", inv.OrgRole, "admin")
	}
	if !inv.Has2FA {
		t.Error("Has2FA = false, want true")
	}
	if !inv.SSOLinked {
		t.Error("SSOLinked = false, want true")
	}
	if inv.LastActive != "2026-06-05T11:30:00Z" {
		t.Errorf("LastActive = %q, want %q", inv.LastActive, "2026-06-05T11:30:00Z")
	}
}

func TestComputeUserMetrics_InternalIncludesTeamMemberships(t *testing.T) {
	members := []sentry.Member{
		{
			ID: "u1", Email: "jane@acme.com", Name: "Jane",
			OrgRole: "admin",
			User:    &sentry.MemberUser{IsActive: true},
		},
		{
			ID: "u2", Email: "bob@acme.com", Name: "Bob",
			OrgRole: "member",
			User:    &sentry.MemberUser{IsActive: true},
		},
	}

	teamMemberships := TeamMembershipMap{
		"u1": {
			{TeamSlug: "backend", TeamRole: "admin"},
			{TeamSlug: "frontend", TeamRole: "contributor"},
		},
	}

	m, _ := computeUserMetrics(members, LevelInternal, teamMemberships)

	if len(m.Inventory) != 2 {
		t.Fatalf("expected 2 inventory items, got %d", len(m.Inventory))
	}
	if len(m.Inventory[0].TeamMemberships) != 2 {
		t.Errorf("expected 2 team memberships for u1, got %d", len(m.Inventory[0].TeamMemberships))
	}
	if m.Inventory[0].TeamMemberships[0].TeamSlug != "backend" {
		t.Errorf("TeamSlug = %q, want %q", m.Inventory[0].TeamMemberships[0].TeamSlug, "backend")
	}
	if m.Inventory[1].TeamMemberships != nil {
		t.Errorf("expected nil team memberships for u2, got %v", m.Inventory[1].TeamMemberships)
	}
}

func TestComputeUserMetrics_AuditOmitsTeamMemberships(t *testing.T) {
	members := []sentry.Member{
		{ID: "u1", User: &sentry.MemberUser{IsActive: true}},
	}
	teamMemberships := TeamMembershipMap{
		"u1": {{TeamSlug: "backend", TeamRole: "admin"}},
	}

	m, _ := computeUserMetrics(members, LevelAudit, teamMemberships)

	if m.Inventory[0].TeamMemberships != nil {
		t.Errorf("expected nil team memberships at audit level, got %v", m.Inventory[0].TeamMemberships)
	}
}
