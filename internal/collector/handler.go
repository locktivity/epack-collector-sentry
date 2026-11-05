package collector

import (
	"time"

	"github.com/locktivity/epack-collector-sentry/docs/schema"
	"github.com/locktivity/epack-collector-sentry/internal/sentry"
	"github.com/locktivity/epack/componentsdk"
)

// artifactPath is where the collector's detailed artifact sits in the pack.
// componentsdk.ArtifactPath gives this path for a file with no schema ID.
const artifactPath = "artifacts/sentry.details.json"

// Spec describes the collector to componentsdk.RunCollector.
func Spec(version, commit string) componentsdk.CollectorSpec {
	return componentsdk.CollectorSpec{
		Name:        "sentry",
		Version:     version,
		Commit:      commit,
		Description: "Collects Sentry application monitoring posture",
		Timeout:     5 * time.Minute,
		Schemas:     map[string][]byte{artifactPath: schema.Monitoring},
		Methods:     schema.Methods(),
		TraceMap:    schema.TraceMap,
		Writes:      []string{accessPostureSchema},
	}
}

// Run is the collector's handler. It fails only for invalid configuration or
// a failed sign-in, and otherwise emits what it collected with a gap recorded
// for each value it could not read.
func Run(ctx componentsdk.CollectorContext) error {
	cfg, err := ParseConfig(ctx.Config())
	if err != nil {
		return componentsdk.NewConfigError("%s", err)
	}

	token := ctx.Secret("SENTRY_AUTH_TOKEN")
	if token == "" {
		return componentsdk.NewConfigError("SENTRY_AUTH_TOKEN is required")
	}

	httpClient := componentsdk.NewHTTPClient(ctx, componentsdk.HTTPOptions{
		CredentialHosts: []string{sentry.Host(cfg.SentryURL)},
	})
	client := sentry.NewClient(httpClient, cfg.SentryURL, cfg.Organization, token)

	c := New(cfg, client, ctx.Level(), runClock{ctx: ctx}, componentsdk.Records(ctx))
	c.collectAccess = componentsdk.Selected(ctx, componentsdk.Location{Schema: accessPostureSchema, Path: accessPosturePath})
	result, err := c.Collect(ctx.Context())
	if err != nil {
		return err
	}

	artifacts := []componentsdk.CollectedArtifact{
		componentsdk.JSONArtifact(artifactPath, result, componentsdk.ArtifactMeta{}),
	}
	if access := c.AccessPosture(); access != nil {
		artifacts = append(artifacts, componentsdk.JSONArtifact(accessPosturePath, access, componentsdk.ArtifactMeta{Schema: accessPostureSchema}))
	}
	return ctx.Emit(artifacts)
}

// runClock reads the run's clock, which tests pin.
type runClock struct {
	ctx componentsdk.CollectorContext
}

func (c runClock) Now() time.Time { return componentsdk.Now(c.ctx) }
