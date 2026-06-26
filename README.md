# epack-collector-sentry

An epack collector that gathers Sentry application monitoring posture evidence, built with the [Component SDK](https://github.com/locktivity/epack).

See [docs/](docs/) for usage documentation.

## What it collects

- **Cron monitors:** scheduled job monitoring configuration, status, and alert targets
- **Alert rules:** metric and issue alert rules with triggers and notification actions
- **Users:** organization members with 2FA, SSO, and role information (audit+)
- **Teams:** team structure with member counts and project assignments (audit+)
- **Monitoring summary:** combined posture answer across monitors and alert rules

## Collection levels

| Level | What's included |
|-------|----------------|
| `trust` | `monitoring_summary.alerts_or_monitors_enabled` (boolean) |
| `audit` | Aggregate counts, per-entity inventories, user/team metrics |
| `internal` | Cross-entity detail: team memberships, integration IDs, open alert counts |

## Development

```bash
# Build
make build

# Run tests
make test

# Lint
go vet ./...

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
