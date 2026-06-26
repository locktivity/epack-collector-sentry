package sentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func testServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "acme", "test-token")
	return srv, c
}

func TestClient_AuthHeader(t *testing.T) {
	var gotAuth string
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]Monitor{})
	})

	_, _ = c.ListMonitors(context.Background(), nil, nil)
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-token")
	}
}

func TestClient_ListMonitors_SinglePage(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/organizations/acme/monitors/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("project") != "-1" {
			t.Errorf("expected project=-1 for empty project list, got %q", r.URL.Query().Get("project"))
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]Monitor{
			{ID: "m1", Name: "daily-sync", Status: "active"},
		})
	})

	monitors, err := c.ListMonitors(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("ListMonitors error: %v", err)
	}
	if len(monitors) != 1 {
		t.Fatalf("got %d monitors, want 1", len(monitors))
	}
	if monitors[0].ID != "m1" {
		t.Errorf("monitor ID = %q, want %q", monitors[0].ID, "m1")
	}
}

func TestClient_ListMonitors_ProjectAndEnvParams(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		projects := r.URL.Query()["project"]
		envs := r.URL.Query()["environment"]
		if len(projects) != 2 || projects[0] != "billing" || projects[1] != "api" {
			t.Errorf("projects = %v, want [billing, api]", projects)
		}
		if len(envs) != 1 || envs[0] != "production" {
			t.Errorf("environments = %v, want [production]", envs)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]Monitor{})
	})

	_, err := c.ListMonitors(context.Background(), []string{"billing", "api"}, []string{"production"})
	if err != nil {
		t.Fatalf("ListMonitors error: %v", err)
	}
}

func TestClient_ListMonitors_Pagination(t *testing.T) {
	var callCount atomic.Int32
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		switch n {
		case 1:
			w.Header().Set("Link",
				`<http://x>; rel="previous"; results="false"; cursor="0:0:1", `+
					`<http://x>; rel="next"; results="true"; cursor="1608208573:0:0"`)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]Monitor{{ID: "m1"}})
		case 2:
			if r.URL.Query().Get("cursor") != "1608208573:0:0" {
				t.Errorf("cursor = %q, want %q", r.URL.Query().Get("cursor"), "1608208573:0:0")
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]Monitor{{ID: "m2"}})
		default:
			t.Fatalf("unexpected call %d", n)
		}
	})

	monitors, err := c.ListMonitors(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("ListMonitors error: %v", err)
	}
	if len(monitors) != 2 {
		t.Fatalf("got %d monitors, want 2", len(monitors))
	}
	if monitors[0].ID != "m1" || monitors[1].ID != "m2" {
		t.Errorf("monitor IDs = [%s, %s], want [m1, m2]", monitors[0].ID, monitors[1].ID)
	}
}

func TestClient_ListAlertRules_SinglePage(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/organizations/acme/combined-rules/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]AlertRule{
			{ID: "ar1", Name: "High Error Rate", Type: "alert_rule"},
		})
	})

	rules, err := c.ListAlertRules(context.Background())
	if err != nil {
		t.Fatalf("ListAlertRules error: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != "ar1" {
		t.Errorf("got %v, want 1 rule with ID ar1", rules)
	}
}

func TestClient_ListMembers_SinglePage(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/organizations/acme/members/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]Member{
			{ID: "u1", Email: "admin@acme.com", OrgRole: "admin"},
		})
	})

	members, err := c.ListMembers(context.Background())
	if err != nil {
		t.Fatalf("ListMembers error: %v", err)
	}
	if len(members) != 1 || members[0].Email != "admin@acme.com" {
		t.Errorf("got %v, want 1 member", members)
	}
}

func TestClient_ListTeams_SinglePage(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/organizations/acme/teams/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]Team{
			{ID: "t1", Slug: "backend", Name: "Backend"},
		})
	})

	teams, err := c.ListTeams(context.Background())
	if err != nil {
		t.Fatalf("ListTeams error: %v", err)
	}
	if len(teams) != 1 || teams[0].Slug != "backend" {
		t.Errorf("got %v, want 1 team", teams)
	}
}

func TestClient_ListTeamMembers_SinglePage(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/teams/acme/backend/members/") {
			t.Errorf("unexpected path: %s, want /teams/acme/backend/members/", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]TeamMember{
			{ID: "u1", Email: "admin@acme.com", TeamRole: "admin"},
		})
	})

	members, err := c.ListTeamMembers(context.Background(), "backend")
	if err != nil {
		t.Fatalf("ListTeamMembers error: %v", err)
	}
	if len(members) != 1 || members[0].TeamRole != "admin" {
		t.Errorf("got %v, want 1 member", members)
	}
}

func TestClient_Unauthorized(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid token"))
	})

	_, err := c.ListMonitors(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error for 401")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
	}
	if apiErr.Body != "invalid token" {
		t.Errorf("Body = %q, want %q", apiErr.Body, "invalid token")
	}
}

func TestClient_Forbidden(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("missing scope"))
	})

	_, err := c.ListMembers(context.Background())
	if err == nil {
		t.Fatal("expected error for 403")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", apiErr.StatusCode)
	}
}

func TestClient_NonRetryableError(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	})

	_, err := c.ListAlertRules(context.Background())
	if err == nil {
		t.Fatal("expected error for 400")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", apiErr.StatusCode)
	}
}

func TestClient_RetryableError_EventualSuccess(t *testing.T) {
	var callCount atomic.Int32
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("rate limited"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]Monitor{{ID: "m1"}})
	})

	monitors, err := c.ListMonitors(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("ListMonitors error: %v", err)
	}
	if len(monitors) != 1 {
		t.Errorf("got %d monitors, want 1", len(monitors))
	}
	if callCount.Load() != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", callCount.Load())
	}
}

func TestClient_RetryAfterHeader(t *testing.T) {
	var callCount atomic.Int32
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("rate limited"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]Monitor{{ID: "m1"}})
	})

	monitors, err := c.ListMonitors(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("ListMonitors error: %v", err)
	}
	if len(monitors) != 1 {
		t.Errorf("got %d monitors, want 1", len(monitors))
	}
	if callCount.Load() != 2 {
		t.Errorf("expected 2 calls (1 retry after Retry-After), got %d", callCount.Load())
	}
}

func TestClient_ContextCanceled(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]Monitor{})
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.ListMonitors(ctx, nil, nil)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestClient_CustomBaseURL(t *testing.T) {
	var gotPath string
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]Monitor{})
	})

	_, _ = c.ListMonitors(context.Background(), nil, nil)
	if !strings.HasPrefix(gotPath, "/api/0/") {
		t.Errorf("path = %q, expected /api/0/ prefix", gotPath)
	}
}
