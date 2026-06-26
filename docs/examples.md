## Basic Usage

```yaml
collectors:
  sentry:
    source: locktivity/epack-collector-sentry@v1
    config:
      organization: acme-corp
    secrets:
      - SENTRY_AUTH_TOKEN
```

## With Project Filtering

```yaml
collectors:
  sentry:
    source: locktivity/epack-collector-sentry@v1
    config:
      organization: acme-corp
      projects:
        - billing-service
        - api-gateway
      environments:
        - production
    secrets:
      - SENTRY_AUTH_TOKEN
```

## Audit Level

```yaml
collectors:
  sentry:
    source: locktivity/epack-collector-sentry@v1
    config:
      organization: acme-corp
      level: audit
    secrets:
      - SENTRY_AUTH_TOKEN
```

## Self-Hosted Sentry

```yaml
collectors:
  sentry:
    source: locktivity/epack-collector-sentry@v1
    config:
      organization: my-org
      sentry_url: https://sentry.internal.company.com
    secrets:
      - SENTRY_AUTH_TOKEN
```

## Example Output (trust)

```json
{
  "schema_version": "1.0.0",
  "collected_at": "2026-06-26T14:00:00Z",
  "collected_at_level": "trust",
  "organization": "acme-corp",
  "monitoring_summary": {
    "alerts_or_monitors_enabled": true
  },
  "diagnostics": {
    "errors": [],
    "warnings": [],
    "truncation": {}
  }
}
```
