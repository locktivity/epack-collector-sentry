//go:build e2e

package sentry

import (
	"context"
	"os"
	"testing"
)

func e2eClient(t *testing.T) *Client {
	t.Helper()
	token := os.Getenv("SENTRY_AUTH_TOKEN")
	if token == "" {
		t.Skip("SENTRY_AUTH_TOKEN not set; skipping e2e test")
	}
	org := os.Getenv("SENTRY_ORG")
	if org == "" {
		t.Skip("SENTRY_ORG not set; skipping e2e test")
	}
	baseURL := os.Getenv("SENTRY_URL")
	return NewClient(baseURL, org, token)
}

func TestE2E_ListMonitors(t *testing.T) {
	c := e2eClient(t)
	monitors, err := c.ListMonitors(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("ListMonitors: %v", err)
	}
	t.Logf("Found %d monitors", len(monitors))
}

func TestE2E_ListAlertRules(t *testing.T) {
	c := e2eClient(t)
	rules, err := c.ListAlertRules(context.Background())
	if err != nil {
		t.Fatalf("ListAlertRules: %v", err)
	}
	t.Logf("Found %d alert rules", len(rules))
}

func TestE2E_ListMembers(t *testing.T) {
	c := e2eClient(t)
	members, err := c.ListMembers(context.Background())
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	t.Logf("Found %d members", len(members))
	for _, m := range members {
		if m.Email == "" {
			t.Error("member missing email")
		}
	}
}

func TestE2E_ListTeams(t *testing.T) {
	c := e2eClient(t)
	teams, err := c.ListTeams(context.Background())
	if err != nil {
		t.Fatalf("ListTeams: %v", err)
	}
	t.Logf("Found %d teams", len(teams))

	if len(teams) > 0 {
		members, err := c.ListTeamMembers(context.Background(), teams[0].Slug)
		if err != nil {
			t.Fatalf("ListTeamMembers(%s): %v", teams[0].Slug, err)
		}
		t.Logf("Team %q has %d members", teams[0].Slug, len(members))
	}
}
