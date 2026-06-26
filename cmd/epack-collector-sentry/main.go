package main

import (
	"time"

	"github.com/locktivity/epack/componentsdk"
	"github.com/locktivity/epack-collector-sentry/internal/collector"
	"github.com/locktivity/epack-collector-sentry/internal/sentry"
)

var (
	Version = "dev"
	Commit  = "unknown"
)

func main() {
	componentsdk.RunCollector(componentsdk.CollectorSpec{
		Name:        "sentry",
		Version:     Version,
		Description: "Collects Sentry application monitoring posture",
		Timeout:     5 * time.Minute,
	}, run)
}

func run(ctx componentsdk.CollectorContext) error {
	cfg, err := collector.ParseConfig(ctx.Config())
	if err != nil {
		return componentsdk.NewConfigError("%s", err)
	}

	token := ctx.Secret("SENTRY_AUTH_TOKEN")
	if token == "" {
		return componentsdk.NewAuthError("SENTRY_AUTH_TOKEN is required")
	}

	client := sentry.NewClient(cfg.SentryURL, cfg.Organization, token)

	level := collector.ParseLevel(ctx.Config())
	c := collector.New(cfg, client, level)
	result, err := c.Collect(ctx.Context())
	if err != nil {
		return err
	}

	return ctx.Emit(result)
}
