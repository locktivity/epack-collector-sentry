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
| `SENTRY_AUTH_TOKEN` | Personal auth token or internal integration token |

## Required Sentry scopes

| Scope | Levels | Surface |
|-------|--------|---------|
| `org:read` | trust+ | Organization metadata |
| `alerts:read` | trust+ | Monitors and alert rules |
| `member:read` | audit+ | Organization members |
| `team:read` | audit+ | Teams and team members |
