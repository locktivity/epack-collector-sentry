package collector

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/locktivity/epack-collector-sentry/internal/sentry"
)

func TestComputeAuthSummary(t *testing.T) {
	sum := computeAuthSummary(nil, &sentry.APIError{StatusCode: http.StatusForbidden})
	if sum.SSOConfigured != nil || sum.SSORequired != nil || sum.SCIMEnabled != nil || sum.SSOProvider != "" {
		t.Errorf("lookup failure should leave all fields unknown: %+v", sum)
	}

	sum = computeAuthSummary(nil, nil)
	if sum.SSOConfigured == nil || *sum.SSOConfigured || sum.SSORequired == nil || *sum.SSORequired || sum.SCIMEnabled == nil || *sum.SCIMEnabled {
		t.Errorf("no provider should report configured/required/scim false: %+v", sum)
	}

	sum = computeAuthSummary(&sentry.AuthProvider{ProviderName: "okta", RequireLink: true, ScimEnabled: false}, nil)
	if sum.SSOConfigured == nil || !*sum.SSOConfigured || sum.SSOProvider != "okta" ||
		sum.SSORequired == nil || !*sum.SSORequired || sum.SCIMEnabled == nil || *sum.SCIMEnabled {
		t.Errorf("provider fields wrong: %+v", sum)
	}
}

func TestCollect_AuthLookupFailureWarnsAtTrust(t *testing.T) {
	fake := richFake()
	fake.authProvider = nil
	fake.authProviderErr = &sentry.APIError{StatusCode: http.StatusForbidden}

	c := testCollector(fake, LevelTrust)
	out, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if out.Auth.SSOConfigured != nil || out.Auth.SSORequired != nil {
		t.Errorf("auth should be unknown on lookup failure: %+v", out.Auth)
	}

	found := false
	for _, w := range out.Diagnostics.Warnings {
		if strings.Contains(w, "auth-provider") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected auth-provider warning at trust, got %v", out.Diagnostics.Warnings)
	}
}
