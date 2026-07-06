package sentry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL    = "https://sentry.io"
	apiPrefix         = "/api/0"
	requestTimeout    = 30 * time.Second
	rateLimitSlowDown = 10
)

type Client struct {
	baseURL      string
	organization string
	token        string
	httpClient   *http.Client
}

func NewClient(baseURL, organization, token string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	return &Client{
		baseURL:      baseURL + apiPrefix,
		organization: organization,
		token:        token,
		httpClient:   &http.Client{Timeout: requestTimeout},
	}
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("sentry API error: status %d: %s", e.StatusCode, e.Body)
}

func (c *Client) ListMonitors(ctx context.Context, projects []string, environments []string) ([]Monitor, error) {
	var all []Monitor
	cursor := ""

	for {
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
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		path := fmt.Sprintf("/organizations/%s/monitors/", c.organization)
		var page []Monitor
		resp, err := c.doPagedRequest(ctx, path, params, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)

		info := parseLinkHeader(resp)
		if !info.HasNext {
			break
		}
		cursor = info.NextCursor
	}

	return all, nil
}

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
		if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == 403 {
			projectList = nil
		} else {
			return nil, err
		}
	}

	return mapDetectorsToAlertRules(detectors, workflows, projectList), nil
}

func (c *Client) listDetectors(ctx context.Context, projects []string) ([]Detector, error) {
	var all []Detector
	cursor := ""

	for {
		params := url.Values{}
		params.Set("per_page", "100")
		if len(projects) > 0 {
			for _, p := range projects {
				params.Add("project", p)
			}
		}
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		path := fmt.Sprintf("/organizations/%s/detectors/", c.organization)
		var page []Detector
		resp, err := c.doPagedRequest(ctx, path, params, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)

		info := parseLinkHeader(resp)
		if !info.HasNext {
			break
		}
		cursor = info.NextCursor
	}

	return all, nil
}

func (c *Client) listWorkflows(ctx context.Context) ([]Workflow, error) {
	var all []Workflow
	cursor := ""

	for {
		params := url.Values{}
		params.Set("per_page", "100")
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		path := fmt.Sprintf("/organizations/%s/workflows/", c.organization)
		var page []Workflow
		resp, err := c.doPagedRequest(ctx, path, params, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)

		info := parseLinkHeader(resp)
		if !info.HasNext {
			break
		}
		cursor = info.NextCursor
	}

	return all, nil
}

func (c *Client) listProjects(ctx context.Context) ([]Project, error) {
	var all []Project
	cursor := ""

	for {
		params := url.Values{}
		params.Set("per_page", "100")
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		path := fmt.Sprintf("/organizations/%s/projects/", c.organization)
		var page []Project
		resp, err := c.doPagedRequest(ctx, path, params, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)

		info := parseLinkHeader(resp)
		if !info.HasNext {
			break
		}
		cursor = info.NextCursor
	}

	return all, nil
}

func (c *Client) ListMembers(ctx context.Context) ([]Member, error) {
	var all []Member
	cursor := ""

	for {
		params := url.Values{}
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		path := fmt.Sprintf("/organizations/%s/members/", c.organization)
		var page []Member
		resp, err := c.doPagedRequest(ctx, path, params, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)

		info := parseLinkHeader(resp)
		if !info.HasNext {
			break
		}
		cursor = info.NextCursor
	}

	return all, nil
}

func (c *Client) ListTeams(ctx context.Context) ([]Team, error) {
	var all []Team
	cursor := ""

	for {
		params := url.Values{}
		params.Set("per_page", "100")
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		path := fmt.Sprintf("/organizations/%s/teams/", c.organization)
		var page []Team
		resp, err := c.doPagedRequest(ctx, path, params, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)

		info := parseLinkHeader(resp)
		if !info.HasNext {
			break
		}
		cursor = info.NextCursor
	}

	return all, nil
}

func (c *Client) ListTeamMembers(ctx context.Context, teamSlug string) ([]TeamMember, error) {
	var all []TeamMember
	cursor := ""

	for {
		params := url.Values{}
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		path := fmt.Sprintf("/teams/%s/%s/members/", c.organization, teamSlug)
		var page []TeamMember
		resp, err := c.doPagedRequest(ctx, path, params, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)

		info := parseLinkHeader(resp)
		if !info.HasNext {
			break
		}
		cursor = info.NextCursor
	}

	return all, nil
}

func (c *Client) doPagedRequest(ctx context.Context, path string, params url.Values, target any) (*http.Response, error) {
	var lastErr error

	for attempt := range maxAttempts {
		resp, err := c.doRequest(ctx, path, params)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusOK {
			defer func() { _ = resp.Body.Close() }()
			if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
				return nil, fmt.Errorf("decoding response: %w", err)
			}
			c.respectRateLimit(resp)
			return resp, nil
		}

		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, &APIError{StatusCode: resp.StatusCode, Body: string(body)}
		}

		if !isRetryable(resp.StatusCode) {
			return nil, &APIError{StatusCode: resp.StatusCode, Body: string(body)}
		}

		lastErr = &APIError{StatusCode: resp.StatusCode, Body: string(body)}
		if err := sleep(ctx, retryDelay(resp, attempt)); err != nil {
			return nil, err
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *Client) doRequest(ctx context.Context, path string, params url.Values) (*http.Response, error) {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	return c.httpClient.Do(req)
}

func (c *Client) respectRateLimit(resp *http.Response) {
	remaining := resp.Header.Get("X-Sentry-Rate-Limit-Remaining")
	if remaining == "" {
		return
	}
	n, err := strconv.Atoi(remaining)
	if err != nil {
		return
	}
	if n < rateLimitSlowDown {
		time.Sleep(1 * time.Second)
	}
}
