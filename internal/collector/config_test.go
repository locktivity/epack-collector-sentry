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

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]any
		want Level
	}{
		{"missing key", map[string]any{}, LevelTrust},
		{"trust", map[string]any{"level": "trust"}, LevelTrust},
		{"audit", map[string]any{"level": "audit"}, LevelAudit},
		{"internal", map[string]any{"level": "internal"}, LevelInternal},
		{"unknown downgrades", map[string]any{"level": "secret"}, LevelTrust},
		{"non-string", map[string]any{"level": 42}, LevelTrust},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLevel(tt.cfg)
			if got != tt.want {
				t.Errorf("ParseLevel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLevel_AtLeast(t *testing.T) {
	tests := []struct {
		level Level
		other Level
		want  bool
	}{
		{LevelTrust, LevelTrust, true},
		{LevelTrust, LevelAudit, false},
		{LevelTrust, LevelInternal, false},
		{LevelAudit, LevelTrust, true},
		{LevelAudit, LevelAudit, true},
		{LevelAudit, LevelInternal, false},
		{LevelInternal, LevelTrust, true},
		{LevelInternal, LevelAudit, true},
		{LevelInternal, LevelInternal, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.level)+"_atleast_"+string(tt.other), func(t *testing.T) {
			if got := tt.level.AtLeast(tt.other); got != tt.want {
				t.Errorf("%s.AtLeast(%s) = %v, want %v", tt.level, tt.other, got, tt.want)
			}
		})
	}
}
