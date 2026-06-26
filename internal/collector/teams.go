package collector

import (
	"fmt"

	"github.com/locktivity/epack-collector-sentry/internal/sentry"
)

type TeamMetrics struct {
	TotalCount       int              `json:"total_count"`
	WithMembersCount int              `json:"with_members_count"`
	Inventory        []TeamInventory  `json:"inventory,omitempty"`
}

type TeamInventory struct {
	ID           string              `json:"id"`
	Slug         string              `json:"slug"`
	Name         string              `json:"name"`
	MemberCount  int                 `json:"member_count"`
	ProjectCount int                 `json:"project_count"`
	DateCreated  string              `json:"date_created,omitempty"`
	Members      []TeamMemberDetail  `json:"members,omitempty"`
}

type TeamMemberDetail struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	TeamRole string `json:"team_role"`
}

type TeamMembersMap map[string][]sentry.TeamMember

func computeTeamMetrics(teams []sentry.Team, level Level, teamMembers TeamMembersMap) (TeamMetrics, *string) {
	m := TeamMetrics{
		TotalCount: len(teams),
	}

	for _, t := range teams {
		if t.MemberCount > 0 {
			m.WithMembersCount++
		}
	}

	var truncationMsg *string
	if level.AtLeast(LevelAudit) {
		items := teams
		if len(items) > maxInventorySize {
			msg := fmt.Sprintf("teams truncated from %d to %d", len(items), maxInventorySize)
			truncationMsg = &msg
			items = items[:maxInventorySize]
		}
		inv := make([]TeamInventory, 0, len(items))
		for _, t := range items {
			ti := TeamInventory{
				ID:           t.ID,
				Slug:         t.Slug,
				Name:         t.Name,
				MemberCount:  t.MemberCount,
				ProjectCount: t.ProjectCount,
				DateCreated:  t.DateCreated,
			}
			if level.AtLeast(LevelInternal) && teamMembers != nil {
				if members, ok := teamMembers[t.Slug]; ok {
					details := make([]TeamMemberDetail, 0, len(members))
					for _, tm := range members {
						details = append(details, TeamMemberDetail{
							UserID:   tm.ID,
							Email:    tm.Email,
							TeamRole: tm.TeamRole,
						})
					}
					ti.Members = details
				}
			}
			inv = append(inv, ti)
		}
		m.Inventory = inv
	}

	return m, truncationMsg
}
