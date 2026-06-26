package collector

import (
	"testing"

	"github.com/locktivity/epack-collector-sentry/internal/sentry"
)

func TestComputeTeamMetrics_Empty(t *testing.T) {
	m, trunc := computeTeamMetrics(nil, LevelAudit, nil)
	if m.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", m.TotalCount)
	}
	if trunc != nil {
		t.Errorf("truncation = %v, want nil", trunc)
	}
}

func TestComputeTeamMetrics_Mixed(t *testing.T) {
	teams := []sentry.Team{
		{ID: "1", Slug: "backend", Name: "Backend", MemberCount: 6},
		{ID: "2", Slug: "frontend", Name: "Frontend", MemberCount: 4},
		{ID: "3", Slug: "empty", Name: "Empty Team", MemberCount: 0},
	}

	m, _ := computeTeamMetrics(teams, LevelAudit, nil)

	if m.TotalCount != 3 {
		t.Errorf("TotalCount = %d, want 3", m.TotalCount)
	}
	if m.WithMembersCount != 2 {
		t.Errorf("WithMembersCount = %d, want 2", m.WithMembersCount)
	}
}

func TestComputeTeamMetrics_TrustOmitsInventory(t *testing.T) {
	teams := []sentry.Team{{ID: "1", MemberCount: 3}}

	m, _ := computeTeamMetrics(teams, LevelTrust, nil)

	if m.Inventory != nil {
		t.Errorf("expected nil inventory at trust, got %d items", len(m.Inventory))
	}
}

func TestComputeTeamMetrics_AuditIncludesInventory(t *testing.T) {
	teams := []sentry.Team{
		{
			ID: "t42", Slug: "backend", Name: "Backend Team",
			MemberCount: 6, ProjectCount: 3,
			DateCreated: "2024-01-10T09:00:00Z",
		},
	}

	m, _ := computeTeamMetrics(teams, LevelAudit, nil)

	if len(m.Inventory) != 1 {
		t.Fatalf("expected 1 inventory item, got %d", len(m.Inventory))
	}
	inv := m.Inventory[0]
	if inv.Slug != "backend" {
		t.Errorf("Slug = %q, want %q", inv.Slug, "backend")
	}
	if inv.MemberCount != 6 {
		t.Errorf("MemberCount = %d, want 6", inv.MemberCount)
	}
	if inv.ProjectCount != 3 {
		t.Errorf("ProjectCount = %d, want 3", inv.ProjectCount)
	}
}

func TestComputeTeamMetrics_InternalIncludesMembers(t *testing.T) {
	teams := []sentry.Team{
		{ID: "t1", Slug: "backend", Name: "Backend", MemberCount: 2},
	}
	teamMembers := TeamMembersMap{
		"backend": {
			{ID: "u1", Email: "jane@acme.com", Name: "Jane", TeamRole: "admin"},
			{ID: "u2", Email: "bob@acme.com", Name: "Bob", TeamRole: "contributor"},
		},
	}

	m, _ := computeTeamMetrics(teams, LevelInternal, teamMembers)

	if len(m.Inventory) != 1 {
		t.Fatalf("expected 1 inventory item, got %d", len(m.Inventory))
	}
	if len(m.Inventory[0].Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(m.Inventory[0].Members))
	}
	if m.Inventory[0].Members[0].Email != "jane@acme.com" {
		t.Errorf("Email = %q, want %q", m.Inventory[0].Members[0].Email, "jane@acme.com")
	}
	if m.Inventory[0].Members[1].TeamRole != "contributor" {
		t.Errorf("TeamRole = %q, want %q", m.Inventory[0].Members[1].TeamRole, "contributor")
	}
}

func TestComputeTeamMetrics_AuditOmitsMembers(t *testing.T) {
	teams := []sentry.Team{
		{ID: "t1", Slug: "backend", Name: "Backend", MemberCount: 2},
	}
	teamMembers := TeamMembersMap{
		"backend": {
			{ID: "u1", Email: "jane@acme.com", TeamRole: "admin"},
		},
	}

	m, _ := computeTeamMetrics(teams, LevelAudit, teamMembers)

	if m.Inventory[0].Members != nil {
		t.Errorf("expected nil members at audit level, got %v", m.Inventory[0].Members)
	}
}
