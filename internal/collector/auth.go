package collector

import "github.com/locktivity/epack-collector-sentry/internal/sentry"

// AuthSummary reports the org's SSO posture at every level. Pointer fields are
// nil when the auth-provider lookup failed, so "unknown" is distinguishable
// from "SSO not configured".
type AuthSummary struct {
	SSOConfigured *bool  `json:"sso_configured"`
	SSOProvider   string `json:"sso_provider,omitempty"`
	SSORequired   *bool  `json:"sso_required"`
	SCIMEnabled   *bool  `json:"scim_enabled"`
}

func computeAuthSummary(provider *sentry.AuthProvider, err error) AuthSummary {
	if err != nil {
		return AuthSummary{}
	}

	configured := provider != nil
	s := AuthSummary{SSOConfigured: &configured}
	if provider == nil {
		f := false
		s.SSORequired = &f
		s.SCIMEnabled = &f
		return s
	}

	s.SSOProvider = provider.ProviderName
	s.SSORequired = &provider.RequireLink
	s.SCIMEnabled = &provider.ScimEnabled
	return s
}
