package collector

import "testing"

func TestParseConfig_Valid(t *testing.T) {
	raw := map[string]any{
		"organization": "acme-corp",
		"sentry_url":   "https://de.sentry.io",
		"projects":     []any{"billing", "auth"},
		"environments": []any{"production"},
	}

	cfg, err := ParseConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Organization != "acme-corp" {
		t.Errorf("Organization = %q, want %q", cfg.Organization, "acme-corp")
	}
	if cfg.SentryURL != "https://de.sentry.io" {
		t.Errorf("SentryURL = %q, want %q", cfg.SentryURL, "https://de.sentry.io")
	}
	if len(cfg.Projects) != 2 || cfg.Projects[0] != "billing" {
		t.Errorf("Projects = %v, want [billing auth]", cfg.Projects)
	}
	if len(cfg.Environments) != 1 || cfg.Environments[0] != "production" {
		t.Errorf("Environments = %v, want [production]", cfg.Environments)
	}
}

func TestParseConfig_MinimalValid(t *testing.T) {
	raw := map[string]any{"organization": "acme"}

	cfg, err := ParseConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Organization != "acme" {
		t.Errorf("Organization = %q, want %q", cfg.Organization, "acme")
	}
	if cfg.SentryURL != "" {
		t.Errorf("SentryURL = %q, want empty", cfg.SentryURL)
	}
}

func TestParseConfig_MissingOrganization(t *testing.T) {
	raw := map[string]any{}

	_, err := ParseConfig(raw)
	if err == nil {
		t.Fatal("expected error for missing organization")
	}
}

func TestParseConfig_EmptyOrganization(t *testing.T) {
	raw := map[string]any{"organization": ""}

	_, err := ParseConfig(raw)
	if err == nil {
		t.Fatal("expected error for empty organization")
	}
}

func TestParseConfig_NilConfig(t *testing.T) {
	_, err := ParseConfig(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestParseConfig_SentryURLRejectsHTTP(t *testing.T) {
	raw := map[string]any{
		"organization": "acme",
		"sentry_url":   "http://sentry.example.com",
	}
	_, err := ParseConfig(raw)
	if err == nil {
		t.Fatal("expected error for http:// sentry_url")
	}
}

func TestParseConfig_SentryURLAcceptsHTTPS(t *testing.T) {
	raw := map[string]any{
		"organization": "acme",
		"sentry_url":   "https://sentry.example.com",
	}
	cfg, err := ParseConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SentryURL != "https://sentry.example.com" {
		t.Errorf("SentryURL = %q, want %q", cfg.SentryURL, "https://sentry.example.com")
	}
}

