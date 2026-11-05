## Configuration

Add to your `epack.yaml`:

```yaml
collectors:
  sentry:
    source: locktivity/epack-collector-sentry@v1
    config:
      organization: acme-corp
    secrets:
      - SENTRY_AUTH_TOKEN
```

## Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `organization` | string | Yes | Sentry organization slug |
| `sentry_url` | string | No | Base URL (defaults to `https://sentry.io`). Set to `https://de.sentry.io` for EU or a self-hosted URL. |
| `projects` | string[] | No | Filter to these project slugs. Omit for all projects. |
| `environments` | string[] | No | Filter monitors to these environments. Omit for all. |
| `level` | string | No | Collection level: `trust`, `audit`, or `internal`. Defaults to `trust`. |

## Secrets

| Variable | Description |
|----------|-------------|
| `SENTRY_AUTH_TOKEN` | Personal auth token or internal integration token. Required: the run exits 2 without it. |

## Required Sentry scopes

| Scope | Levels | Surface |
|-------|--------|---------|
| `org:read` | trust+ | Organization metadata, SSO settings, projects, integrations, and Sentry app installations. From audit, teams. |
| `alerts:read` | trust+ | Monitors and alert rules |
| `member:read` | trust+ | Organization members. A trust run reads them for `artifacts/sentry.access-posture.json` only. |
| `team:read` | internal | Team members |

The collector checks `org:read` when it signs in, and the run fails without it. A token without one of the other scopes still collects: the values that scope covers are left out and listed in `artifacts/sentry.collector-result.json` with the code `permission_denied`.

To skip the member, integration, and Sentry app reads at trust, leave `access-posture` out of `collect`, as in `collect: [details]`.

`artifacts/sentry.access-posture.json` has no `credentials` section, and its gap has the code `not_read`. Sentry lists organization auth tokens at `GET /api/0/organizations/{org}/org-auth-tokens/` only for a browser session or an OAuth app token with the `org:tokens` scope. A personal or internal integration token cannot list them with any scope.
