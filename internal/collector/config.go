package collector

import "fmt"

type Config struct {
	Organization string
	SentryURL    string
	Projects     []string
	Environments []string
}

func ParseConfig(raw map[string]any) (Config, error) {
	var cfg Config

	if raw == nil {
		return cfg, fmt.Errorf("required config key 'organization' is missing")
	}

	org, ok := raw["organization"]
	if !ok {
		return cfg, fmt.Errorf("required config key 'organization' is missing")
	}
	orgStr, ok := org.(string)
	if !ok || orgStr == "" {
		return cfg, fmt.Errorf("config key 'organization' must be a non-empty string")
	}
	cfg.Organization = orgStr

	if v, ok := raw["sentry_url"]; ok {
		s, ok := v.(string)
		if !ok {
			return cfg, fmt.Errorf("config key 'sentry_url' must be a string")
		}
		cfg.SentryURL = s
	}

	if v, ok := raw["projects"]; ok {
		cfg.Projects = toStringSlice(v)
	}

	if v, ok := raw["environments"]; ok {
		cfg.Environments = toStringSlice(v)
	}

	return cfg, nil
}

func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
