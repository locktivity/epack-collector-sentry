package sentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/locktivity/epack/componentsdk"
)

const (
	defaultBaseURL = "https://sentry.io"
	apiPrefix      = "/api/0"
)

type Client struct {
	httpClient   *http.Client
	baseURL      string
	organization string
	token        string
}

// NewClient returns a client for one organization. Every call goes through
// httpClient, which retries the calls it can.
func NewClient(httpClient *http.Client, baseURL, organization, token string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	return &Client{
		httpClient:   httpClient,
		baseURL:      baseURL + apiPrefix,
		organization: organization,
		token:        token,
	}
}

// Host returns the host a client for baseURL sends its calls, and its token,
// to.
func Host(baseURL string) string {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

type APIError struct {
	StatusCode int
	Body       string
	Header     http.Header
}

func (e *APIError) Error() string {
	return fmt.Sprintf("sentry API error: status %d: %s", e.StatusCode, e.Body)
}

// HTTPStatus, HTTPHeader, and VendorCode let componentsdk.Classify read the
// status and throttle headers, and give records Sentry's error code.
func (e *APIError) HTTPStatus() int         { return e.StatusCode }
func (e *APIError) HTTPHeader() http.Header { return e.Header }

// VendorCode returns the code some error details carry, such as
// {"detail":{"code":"sso-required",...}}, or "".
func (e *APIError) VendorCode() string {
	var body struct {
		Detail json.RawMessage `json:"detail"`
	}
	var detail struct {
		Code string `json:"code"`
	}
	if json.Unmarshal([]byte(e.Body), &body) != nil || json.Unmarshal(body.Detail, &detail) != nil {
		return ""
	}
	return detail.Code
}

// GetOrganization reads the organization the client is for. It is the
// collector's sign-in: it proves the token works and can read the
// organization. A 404 means Sentry has no organization by that slug, which
// is a configuration error.
func (c *Client) GetOrganization(ctx context.Context) (*Organization, error) {
	const op = "sentry.organization.get"
	var org Organization
	if _, err := c.getJSON(ctx, op, c.endpoint(fmt.Sprintf("/organizations/%s/", c.organization), nil), &org); err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return nil, componentsdk.CodeError(componentsdk.CodeConfigInvalid, fmt.Errorf("organization %q not found: %w", c.organization, err))
		}
		return nil, err
	}
	if org.ID == "" || org.Slug == "" {
		return nil, invalidResponse(op, "has no id or slug")
	}
	return &org, nil
}

func (c *Client) ListMonitors(ctx context.Context, projects []string, environments []string) ([]Monitor, error) {
	params := url.Values{}
	if len(projects) == 0 {
		params.Set("project", "-1")
	} else {
		for _, p := range projects {
			params.Add("project", p)
		}
	}
	for _, e := range environments {
		params.Add("environment", e)
	}
	return list[Monitor](ctx, c, "sentry.monitors.list", fmt.Sprintf("/organizations/%s/monitors/", c.organization), params)
}

// ListAlertRules reads detectors, the workflows that act on them, and the
// projects that name them, and fails when any of the three fails.
func (c *Client) ListAlertRules(ctx context.Context, projects []string) ([]AlertRule, error) {
	detectors, err := c.listDetectors(ctx, projects)
	if err != nil {
		return nil, err
	}

	workflows, err := c.listWorkflows(ctx)
	if err != nil {
		return nil, err
	}

	projectList, err := c.listProjects(ctx)
	if err != nil {
		return nil, err
	}

	return mapDetectorsToAlertRules(detectors, workflows, projectList), nil
}

func (c *Client) listDetectors(ctx context.Context, projects []string) ([]Detector, error) {
	params := url.Values{}
	params.Set("per_page", "100")
	for _, p := range projects {
		params.Add("project", p)
	}
	return list[Detector](ctx, c, "sentry.detectors.list", fmt.Sprintf("/organizations/%s/detectors/", c.organization), params)
}

func (c *Client) listWorkflows(ctx context.Context) ([]Workflow, error) {
	params := url.Values{}
	params.Set("per_page", "100")
	return list[Workflow](ctx, c, "sentry.workflows.list", fmt.Sprintf("/organizations/%s/workflows/", c.organization), params)
}

func (c *Client) listProjects(ctx context.Context) ([]Project, error) {
	params := url.Values{}
	params.Set("per_page", "100")
	return list[Project](ctx, c, "sentry.projects.list", fmt.Sprintf("/organizations/%s/projects/", c.organization), params)
}

func (c *Client) ListMembers(ctx context.Context) ([]Member, error) {
	return list[Member](ctx, c, "sentry.members.list", fmt.Sprintf("/organizations/%s/members/", c.organization), nil)
}

// ListIntegrations reads the integrations installed in the organization,
// without each one's settings.
func (c *Client) ListIntegrations(ctx context.Context) ([]Integration, error) {
	params := url.Values{}
	params.Set("includeConfig", "0")
	params.Set("per_page", "100")
	return list[Integration](ctx, c, "sentry.integrations.list", fmt.Sprintf("/organizations/%s/integrations/", c.organization), params)
}

// ListSentryAppInstallations reads the Sentry apps installed in the
// organization, including its internal integrations.
func (c *Client) ListSentryAppInstallations(ctx context.Context) ([]SentryAppInstallation, error) {
	params := url.Values{}
	params.Set("per_page", "100")
	return list[SentryAppInstallation](ctx, c, "sentry.sentry_app_installations.list", fmt.Sprintf("/organizations/%s/sentry-app-installations/", c.organization), params)
}

// GetAuthProvider fetches the org's SSO configuration. Requires org:read.
// Returns nil (no error) when no provider is configured (Sentry responds 204).
// The endpoint is marked private in Sentry's API (the web app consumes it), so
// callers should degrade to "unknown" on failure rather than treat it as fatal.
func (c *Client) GetAuthProvider(ctx context.Context) (*AuthProvider, error) {
	const op = "sentry.auth_provider.get"
	path := fmt.Sprintf("/organizations/%s/auth-provider/", c.organization)

	resp, err := c.do(ctx, op, c.endpoint(path, nil))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		var provider struct {
			ProviderName *string `json:"provider_name"`
			RequireLink  *bool   `json:"require_link"`
			ScimEnabled  *bool   `json:"scim_enabled"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&provider); err != nil {
			return nil, componentsdk.ResponseError(resp, fmt.Errorf("decoding response: %w", err))
		}
		if provider.ProviderName == nil || *provider.ProviderName == "" || provider.RequireLink == nil || provider.ScimEnabled == nil {
			return nil, invalidResponse(op, "lacks provider_name, require_link, or scim_enabled")
		}
		return &AuthProvider{ProviderName: *provider.ProviderName, RequireLink: *provider.RequireLink, ScimEnabled: *provider.ScimEnabled}, nil
	case http.StatusNoContent:
		return nil, nil
	}
	return nil, apiError(resp)
}

func (c *Client) ListTeams(ctx context.Context) ([]Team, error) {
	params := url.Values{}
	params.Set("per_page", "100")
	return list[Team](ctx, c, "sentry.teams.list", fmt.Sprintf("/organizations/%s/teams/", c.organization), params)
}

func (c *Client) ListTeamMembers(ctx context.Context, teamSlug string) ([]TeamMember, error) {
	return list[TeamMember](ctx, c, "sentry.teams.members.list", fmt.Sprintf("/teams/%s/%s/members/", c.organization, teamSlug), nil)
}

// list reads every page of a list endpoint, following the next link of each
// page. A page that is not a list fails the read, so a missing list never
// reads as an empty one.
func list[T any](ctx context.Context, c *Client, op, path string, params url.Values) ([]T, error) {
	all := []T{}
	u := c.endpoint(path, params)
	for {
		var page *[]T
		resp, err := c.getJSON(ctx, op, u, &page)
		if err != nil {
			return nil, err
		}
		if page == nil {
			return nil, invalidResponse(op, "has no list")
		}
		all = append(all, *page...)

		next, ok := nextPage(resp)
		if !ok {
			return all, nil
		}
		u = next.String()
	}
}

// endpoint returns the URL of path under the API, with params as its query.
func (c *Client) endpoint(path string, params url.Values) string {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	return u
}

// getJSON makes one call named op to u and decodes its 200 response into
// target. The response it returns has its body read and closed.
func (c *Client) getJSON(ctx context.Context, op, u string, target any) (*http.Response, error) {
	resp, err := c.do(ctx, op, u)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return nil, componentsdk.ResponseError(resp, fmt.Errorf("decoding response: %w", err))
	}
	return resp, nil
}

// permissions holds the scope each call needs. A permission_denied gap from
// the call names it. Sentry's API docs list the scopes each endpoint accepts,
// and the map holds the read scope from that list. The auth-provider
// endpoint is not in the docs. Sentry's source lists org:read for its GET.
var permissions = map[string]string{
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

// do sends a GET named op to u. The caller closes the response body.
func (c *Client) do(ctx context.Context, op, u string) (*http.Response, error) {
	ctx = componentsdk.WithOperation(ctx, componentsdk.Operation{Name: op, Permission: permissions[op]})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	return c.httpClient.Do(req)
}

// apiError reads resp into an APIError. It wraps the error with the call
// that returned resp, so a gap recorded from it names that call and the
// call's permission.
func apiError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return componentsdk.ResponseError(resp, &APIError{StatusCode: resp.StatusCode, Body: string(body), Header: resp.Header})
}

// invalidResponse reports a 200 response that lacks what the collector reads
// from it.
func invalidResponse(op, problem string) error {
	return componentsdk.CodeError(componentsdk.CodeInvalidResponse, fmt.Errorf("the response to %s %s", op, problem))
}
