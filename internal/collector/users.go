package collector

import (
	"fmt"

	"github.com/locktivity/epack-collector-sentry/internal/sentry"
)

type UserMetrics struct {
	TotalCount           int              `json:"total_count"`
	ActiveCount          int              `json:"active_count"`
	PendingCount         int              `json:"pending_count"`
	Has2FACount          int              `json:"has_2fa_count"`
	Has2FAPct            *int             `json:"has_2fa_pct"`
	SSOLinkedCount       int              `json:"sso_linked_count"`
	SSOLinkedPct         *int             `json:"sso_linked_pct"`
	DenominatorActiveUsers int            `json:"denominator_active_users"`
	Inventory            []UserInventory  `json:"inventory,omitempty"`
}

type UserInventory struct {
	ID               string            `json:"id"`
	Email            string            `json:"email"`
	Name             string            `json:"name"`
	OrgRole          string            `json:"org_role"`
	Has2FA           bool              `json:"has_2fa"`
	SSOLinked        bool              `json:"sso_linked"`
	IsActive         bool              `json:"is_active"`
	DateCreated      string            `json:"date_created,omitempty"`
	LastActive       string            `json:"last_active,omitempty"`
	TeamMemberships  []TeamMembership  `json:"team_memberships,omitempty"`
}

type TeamMembership struct {
	TeamSlug string `json:"team_slug"`
	TeamRole string `json:"team_role"`
}

type TeamMembershipMap map[string][]TeamMembership

func computeUserMetrics(members []sentry.Member, level Level, teamMemberships TeamMembershipMap) (UserMetrics, *string) {
	m := UserMetrics{
		TotalCount:             len(members),
		DenominatorActiveUsers: len(members),
	}

	for _, mem := range members {
		if mem.Pending {
			m.PendingCount++
		} else if mem.User != nil && mem.User.IsActive {
			m.ActiveCount++
		}

		if mem.User != nil && mem.User.Has2fa {
			m.Has2FACount++
		}

		if mem.Flags.SSOLinked {
			m.SSOLinkedCount++
		}
	}

	if m.TotalCount > 0 {
		pct2fa := (m.Has2FACount * 100) / m.TotalCount
		m.Has2FAPct = &pct2fa
		pctSSO := (m.SSOLinkedCount * 100) / m.TotalCount
		m.SSOLinkedPct = &pctSSO
	}

	var truncationMsg *string
	if level.AtLeast(LevelAudit) {
		items := members
		if len(items) > maxInventorySize {
			msg := fmt.Sprintf("users truncated from %d to %d", len(items), maxInventorySize)
			truncationMsg = &msg
			items = items[:maxInventorySize]
		}
		inv := make([]UserInventory, 0, len(items))
		for _, mem := range items {
			item := buildUserInventory(mem)
			if level.AtLeast(LevelInternal) && teamMemberships != nil {
				if memberships, ok := teamMemberships[mem.ID]; ok {
					item.TeamMemberships = memberships
				}
			}
			inv = append(inv, item)
		}
		m.Inventory = inv
	}

	return m, truncationMsg
}

func buildUserInventory(mem sentry.Member) UserInventory {
	item := UserInventory{
		ID:          mem.ID,
		Email:       mem.Email,
		Name:        mem.Name,
		OrgRole:     mem.OrgRole,
		SSOLinked:   mem.Flags.SSOLinked,
		DateCreated: mem.DateCreated,
	}

	if mem.User != nil {
		item.Has2FA = mem.User.Has2fa
		item.IsActive = mem.User.IsActive
		item.LastActive = mem.User.LastActive
	}

	return item
}
