package sentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/locktivity/epack/componentsdk"
)

func testServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient(srv.Client(), srv.URL, "acme", "test-token")
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
	var next string
	srv, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		switch n {
		case 1:
			w.Header().Set("Link",
				`<`+next+`&cursor=0:0:1>; rel="previous"; results="false"; cursor="0:0:1", `+
					`<`+next+`&cursor=1608208573:0:0>; rel="next"; results="true"; cursor="1608208573:0:0"`)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]Monitor{{ID: "m1"}})
		case 2:
			if r.URL.Query().Get("cursor") != "1608208573:0:0" {
				t.Errorf("cursor = %q, want %q", r.URL.Query().Get("cursor"), "1608208573:0:0")
			}
			w.Header().Set("Link", `<`+next+`&cursor=1608208573:100:0>; rel="next"; results="false"; cursor="1608208573:100:0"`)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]Monitor{{ID: "m2"}})
		default:
			t.Fatalf("unexpected call %d", n)
		}
	})
	next = srv.URL + "/api/0/organizations/acme/monitors/?project=-1"

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
		w.WriteHeader(http.StatusOK)
		switch {
		case strings.Contains(r.URL.Path, "/detectors/"):
			_ = json.NewEncoder(w).Encode([]Detector{
				{
					ID: "d1", Name: "High Error Rate", Type: "metric_issue",
					ProjectID: "p1", Enabled: true,
					Config: DetectorConfig{DetectionType: "static"},
				},
			})
		case strings.Contains(r.URL.Path, "/workflows/"):
			_ = json.NewEncoder(w).Encode([]Workflow{})
		case strings.Contains(r.URL.Path, "/projects/"):
			_ = json.NewEncoder(w).Encode([]Project{
				{ID: "p1", Slug: "billing", Name: "Billing"},
			})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	})

	rules, err := c.ListAlertRules(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListAlertRules error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(rules))
	}
	if rules[0].ID != "d1" {
		t.Errorf("ID = %q, want %q", rules[0].ID, "d1")
	}
	if rules[0].Type != "alert_rule" {
		t.Errorf("Type = %q, want %q", rules[0].Type, "alert_rule")
	}
	if len(rules[0].Projects) != 1 || rules[0].Projects[0] != "billing" {
		t.Errorf("Projects = %v, want [billing]", rules[0].Projects)
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

// The team list has each team's projects as an array, and no project count
// field.
func TestClient_ListTeams_ReadsProjects(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"id": "48531", "slug": "ancient-gabelers", "name": "Ancient Gabelers", "memberCount": 2,
				"projects": [{"id": "4505", "slug": "billing", "name": "Billing"}, {"id": "4506", "slug": "api", "name": "API"}]},
			{"id": "48533", "slug": "unassigned", "name": "Unassigned", "memberCount": 0, "projects": []}
		]`))
	})

	teams, err := c.ListTeams(context.Background())
	if err != nil {
		t.Fatalf("ListTeams error: %v", err)
	}
	if len(teams) != 2 || len(teams[0].Projects) != 2 || len(teams[1].Projects) != 0 {
		t.Fatalf("teams = %+v, want 2 projects on the first team and none on the second", teams)
	}
	if teams[0].Projects[0].Slug != "billing" {
		t.Errorf("first project = %q, want billing", teams[0].Projects[0].Slug)
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

	_, err := c.ListAlertRules(context.Background(), nil)
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

func TestClient_ResponseWithoutAListIsInvalid(t *testing.T) {
	for _, body := range []string{"null", `{"detail":"ok"}`} {
		t.Run(body, func(t *testing.T) {
			_, c := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			})
			monitors, err := c.ListMonitors(context.Background(), nil, nil)
			if err == nil {
				t.Fatalf("got %v, want an error for a response with no list", monitors)
			}
			if code := componentsdk.Classify(err); code != componentsdk.CodeInvalidResponse {
				t.Errorf("Classify(err) = %s, want invalid_response", code)
			}
		})
	}
}

func TestClient_GetOrganization(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/0/organizations/acme/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"7","slug":"acme","name":"Acme","require2FA":false}`))
	})

	org, err := c.GetOrganization(context.Background())
	if err != nil {
		t.Fatalf("GetOrganization: %v", err)
	}
	if org.ID != "7" || org.Slug != "acme" || org.Require2FA == nil || *org.Require2FA {
		t.Errorf("org = %+v, want require2FA false", org)
	}
}

func TestClient_GetOrganization_WithoutRequire2FA(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"7","slug":"acme","name":"Acme"}`))
	})

	org, err := c.GetOrganization(context.Background())
	if err != nil {
		t.Fatalf("GetOrganization: %v", err)
	}
	if org.Require2FA != nil {
		t.Errorf("Require2FA = %v, want nil when the response leaves it out", *org.Require2FA)
	}
}

func TestClient_ListIntegrations_Pagination(t *testing.T) {
	var callCount atomic.Int32
	var next string
	srv, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/0/organizations/acme/integrations/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if q := r.URL.Query(); q.Get("includeConfig") != "0" || q.Get("per_page") != "100" {
			t.Errorf("query = %s, want includeConfig=0 and per_page=100", r.URL.RawQuery)
		}
		switch callCount.Add(1) {
		case 1:
			w.Header().Set("Link", `<`+next+`&cursor=100:1:0>; rel="next"; results="true"; cursor="100:1:0"`)
			_, _ = w.Write([]byte(`[{"id":"301","name":"Acme Workspace","provider":{"key":"slack"},"organizationIntegrationStatus":"active"}]`))
		case 2:
			if r.URL.Query().Get("cursor") != "100:1:0" {
				t.Errorf("cursor = %q, want 100:1:0", r.URL.Query().Get("cursor"))
			}
			w.Header().Set("Link", `<`+next+`&cursor=100:2:0>; rel="next"; results="false"; cursor="100:2:0"`)
			_, _ = w.Write([]byte(`[{"id":"303","name":"acme.atlassian.net","provider":{"key":"jira"},"organizationIntegrationStatus":"disabled"}]`))
		default:
			t.Fatal("unexpected third call")
		}
	})
	next = srv.URL + "/api/0/organizations/acme/integrations/?includeConfig=0&per_page=100"

	integrations, err := c.ListIntegrations(context.Background())
	if err != nil {
		t.Fatalf("ListIntegrations: %v", err)
	}
	want := []Integration{
		{ID: "301", Name: "Acme Workspace", Provider: IntegrationProvider{Key: "slack"}, OrganizationIntegrationStatus: "active"},
		{ID: "303", Name: "acme.atlassian.net", Provider: IntegrationProvider{Key: "jira"}, OrganizationIntegrationStatus: "disabled"},
	}
	if !reflect.DeepEqual(integrations, want) {
		t.Errorf("integrations = %+v, want %+v", integrations, want)
	}
}

func TestClient_ListSentryAppInstallations(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/0/organizations/acme/sentry-app-installations/" || r.URL.Query().Get("per_page") != "100" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		_, _ = w.Write([]byte(`[{"app":{"uuid":"a1","slug":"acme-deploy-bot","sentryAppId":81},"organization":{"slug":"acme","id":7},"uuid":"i1","status":"installed"}]`))
	})

	installations, err := c.ListSentryAppInstallations(context.Background())
	if err != nil {
		t.Fatalf("ListSentryAppInstallations: %v", err)
	}
	want := []SentryAppInstallation{{UUID: "i1", Status: "installed", App: SentryAppInstallationApp{Slug: "acme-deploy-bot"}}}
	if !reflect.DeepEqual(installations, want) {
		t.Errorf("installations = %+v, want %+v", installations, want)
	}
}

func TestClient_ListSentryAppInstallations_Forbidden(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"detail":"You do not have permission to perform this action."}`))
	})

	_, err := c.ListSentryAppInstallations(context.Background())
	if code := componentsdk.Classify(err); code != componentsdk.CodePermissionDenied {
		t.Errorf("Classify(%v) = %s, want permission_denied", err, code)
	}
}

func TestClient_GetOrganization_WithoutSlugIsInvalid(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})

	_, err := c.GetOrganization(context.Background())
	if code := componentsdk.Classify(err); code != componentsdk.CodeInvalidResponse {
		t.Errorf("Classify(%v) = %s, want invalid_response", err, code)
	}
}

func TestClient_GetOrganization_NotFoundIsConfigInvalid(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"The requested resource does not exist"}`))
	})

	_, err := c.GetOrganization(context.Background())
	if code := componentsdk.Classify(err); code != componentsdk.CodeConfigInvalid {
		t.Errorf("Classify(%v) = %s, want config_invalid", err, code)
	}
}

// opRecorder records the operation each request names, with the permission
// it declares.
type opRecorder struct {
	mu   sync.Mutex
	ops  map[string]bool
	perm map[string]string
	base http.RoundTripper
}

func (r *opRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	op, _ := componentsdk.OperationOf(req.Context())
	r.mu.Lock()
	r.ops[op.Name] = true
	if op.Permission != "" {
		r.perm[op.Name] = op.Permission
	}
	r.mu.Unlock()
	return r.base.RoundTrip(req)
}

func TestClient_NamesEachCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/organizations/acme/"):
			_, _ = w.Write([]byte(`{"id":"1","slug":"acme"}`))
		case strings.HasSuffix(r.URL.Path, "/auth-provider/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(srv.Close)
	rec := &opRecorder{ops: map[string]bool{}, perm: map[string]string{}, base: srv.Client().Transport}
	c := NewClient(&http.Client{Transport: rec}, srv.URL, "acme", "test-token")

	ctx := context.Background()
	if _, err := c.GetOrganization(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListMonitors(ctx, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListAlertRules(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetAuthProvider(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListMembers(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListTeams(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListTeamMembers(ctx, "backend"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListIntegrations(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListSentryAppInstallations(ctx); err != nil {
		t.Fatal(err)
	}

	for _, op := range []string{
		"sentry.organization.get", "sentry.monitors.list", "sentry.detectors.list", "sentry.workflows.list",
		"sentry.projects.list", "sentry.auth_provider.get", "sentry.members.list", "sentry.teams.list",
		"sentry.teams.members.list", "sentry.integrations.list", "sentry.sentry_app_installations.list",
	} {
		if !rec.ops[op] {
			t.Errorf("no request named %s; got %v", op, rec.ops)
		}
	}
	if len(rec.ops) != 11 {
		t.Errorf("operations = %v, want exactly the eleven", rec.ops)
	}
	wantPerm := map[string]string{
		"sentry.organization.get":              "org:read",
		"sentry.monitors.list":                 "alerts:read",
		"sentry.detectors.list":                "alerts:read",
		"sentry.workflows.list":                "alerts:read",
		"sentry.projects.list":                 "org:read",
		"sentry.auth_provider.get":             "org:read",
		"sentry.members.list":                  "member:read",
		"sentry.teams.list":                    "org:read",
		"sentry.teams.members.list":            "team:read",
		"sentry.integrations.list":             "org:read",
		"sentry.sentry_app_installations.list": "org:read",
	}
	if !reflect.DeepEqual(rec.perm, wantPerm) {
		t.Errorf("permissions = %v, want %v", rec.perm, wantPerm)
	}
}

func TestAPIError_VendorCode(t *testing.T) {
	cases := map[string]string{
		`{"detail":{"code":"sso-required","message":"Must login via SSO"}}`: "sso-required",
		`{"detail":"Invalid token"}`:                                        "",
		`not json`:                                                          "",
	}
	for body, want := range cases {
		if got := (&APIError{StatusCode: 401, Body: body}).VendorCode(); got != want {
			t.Errorf("VendorCode(%s) = %q, want %q", body, got, want)
		}
	}
}

func TestHost(t *testing.T) {
	cases := map[string]string{
		"":                                   "sentry.io",
		"https://de.sentry.io":               "de.sentry.io",
		"https://Sentry.Example.com/sentry/": "sentry.example.com",
	}
	for baseURL, want := range cases {
		if got := Host(baseURL); got != want {
			t.Errorf("Host(%q) = %q, want %q", baseURL, got, want)
		}
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

func TestClient_GetAuthProvider_Configured(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/0/organizations/acme/auth-provider/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"1","provider_name":"okta","pending_links_count":2,"login_url":"https://acme.sentry.io/auth/login/acme/","default_role":"member","require_link":true,"scim_enabled":false}`))
	})

	provider, err := c.GetAuthProvider(context.Background())
	if err != nil {
		t.Fatalf("GetAuthProvider: %v", err)
	}
	if provider == nil || provider.ProviderName != "okta" || !provider.RequireLink || provider.ScimEnabled {
		t.Errorf("provider = %+v, want okta with require_link=true scim_enabled=false", provider)
	}
}

func TestClient_GetAuthProvider_NotConfigured(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	provider, err := c.GetAuthProvider(context.Background())
	if err != nil {
		t.Fatalf("GetAuthProvider: %v", err)
	}
	if provider != nil {
		t.Errorf("provider = %+v, want nil for 204", provider)
	}
}

func TestClient_GetAuthProvider_WithoutItsFieldsIsInvalid(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"1","provider_name":"okta"}`))
	})

	provider, err := c.GetAuthProvider(context.Background())
	if provider != nil {
		t.Fatalf("provider = %+v, want nil when require_link and scim_enabled are missing", provider)
	}
	if code := componentsdk.Classify(err); code != componentsdk.CodeInvalidResponse {
		t.Errorf("Classify(%v) = %s, want invalid_response", err, code)
	}
}

func TestClient_GetAuthProvider_Forbidden(t *testing.T) {
	_, c := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"detail":"You do not have permission to perform this action."}`))
	})

	provider, err := c.GetAuthProvider(context.Background())
	if provider != nil || err == nil {
		t.Fatalf("provider = %+v, err = %v; want nil provider and an error", provider, err)
	}
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("err = %v, want APIError with status 403", err)
	}
}
