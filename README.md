# epack-collector-sentry

An epack collector that gathers Sentry application monitoring posture evidence, built with the [Component SDK](https://github.com/locktivity/epack).

See [docs/](docs/) for usage documentation.

## What it collects

- **Auth posture:** whether SSO is configured, which provider, whether it is required for sign-in, and SCIM status
- **Cron monitors:** scheduled job monitoring configuration, status, and alert targets
- **Alert rules:** metric and issue alert rules with triggers and notification actions
- **Users:** organization members with 2FA, SSO, and role information (audit+)
- **Teams:** team structure with member counts and project assignments (audit+)
- **Monitoring summary:** combined posture answer across monitors and alert rules
- **Access:** sign-in rules, member and admin 2FA, inactive members, and connected apps, in `artifacts/sentry.access-posture.json`

## Collection levels

| Level | What's included |
|-------|----------------|
| `trust` | `auth` SSO posture, `monitoring_summary.alerts_or_monitors_enabled`, and the access file's flags and percentages |
| `audit` | Aggregate counts, per-entity inventories, user/team metrics |
| `internal` | Cross-entity detail: team memberships, integration IDs, open alert counts |

## Partial collection

The collector first reads the organization, which checks the token. Only that read, or invalid configuration, can fail the run:

| Exit | When |
|------|------|
| `2` | The configuration is invalid, Sentry has no organization by the configured slug, or `SENTRY_AUTH_TOKEN` is not set |
| `3` | Sentry rejects the token, or the token lacks `org:read` |
| `4` | Sentry keeps failing or cannot be reached during the sign-in |

After the sign-in, a value the collector cannot read is left out of `artifacts/sentry.details.json` or `artifacts/sentry.access-posture.json` rather than written as zero or false, and the run exits 0. Each value left out is listed in `artifacts/sentry.collector-result.json` with a code that says why, such as `permission_denied` for a missing scope. That file also records the calls the collector made. From the audit level, it records the organization the collector signed in to.

## Development

```bash
# Build
make build

# Run tests
make test

# Lint
go vet ./...

# Rewrite docs/schema/trace-map.json after changing the collector
make trace-map

# Run locally (with epack SDK)
epack sdk run ./epack-collector-sentry

# Watch mode: auto-rebuild on file changes
epack sdk run --watch .
```

### Manual Testing

```bash
# Test capabilities
./epack-collector-sentry --capabilities

# Run conformance tests
go install -tags conformance github.com/locktivity/epack/cmd/epack-conformance@latest
$(go env GOPATH)/bin/epack-conformance collector ./epack-collector-sentry
```

## Testing

```bash
go test ./...
```

## Release

Tag a version to trigger the release workflow:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The GitHub Action will:
1. Run tests and conformance checks
2. Build multi-platform binaries (linux/darwin, amd64/arm64)
3. Generate [SLSA Level 3](https://slsa.dev/spec/v1.0/levels#build-l3) provenance attestations
4. Publish to GitHub Releases
