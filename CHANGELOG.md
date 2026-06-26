# Changelog

## v0.1.0

Initial release of the Sentry collector for epack.

### Collection levels

- **trust**: `monitoring_summary.alerts_or_monitors_enabled` boolean. No counts, inventories, or PII.
- **audit**: Aggregate counts and per-entity inventories for monitors, alert rules, users, and teams. Includes 2FA/SSO percentages.
- **internal**: Cross-entity detail: per-monitor open alert counts, user team memberships with roles, team member lists with roles, alert action integration IDs.

### Metrics emitted

| Surface | Aggregates | Inventory fields |
|---------|-----------|-----------------|
| Monitors | total, active, disabled, muted, with_alert_targets_pct | id, name, type, status, schedule, project, owner, connected_workflows, latest_group, environments, open_alert_count (internal) |
| Alert rules | total, metric_alerts, issue_alerts, with_actions_pct | id, name, type, status, aggregate, time_window, threshold_type, project, environment, owner, triggers with actions, integration_id (internal) |
| Users | total, active, pending, has_2fa_pct, sso_linked_pct | id, email, name, org_role, has_2fa, sso_linked, is_active, team_memberships (internal) |
| Teams | total, with_members | id, slug, name, member_count, project_count, members (internal) |

### Required Sentry scopes

| Scope | Levels |
|-------|--------|
| `org:read` | trust+ |
| `alerts:read` | trust+ |
| `member:read` | audit+ |
| `team:read` | audit+ |

### Redaction guarantees

- Trust level contains no PII, no counts, and no entity identifiers.
- Audit level contains email and display name but no cross-entity role mappings.
- Integration IDs appear only at internal level.
- Missing scopes produce diagnostic warnings, not failures.

### Configuration

- `organization` (required): Sentry organization slug.
- `sentry_url` (optional): Base URL, defaults to `https://sentry.io`.
- `projects` (optional): Filter to specific project slugs.
- `environments` (optional): Filter monitors to specific environments.
- `level` (optional): `trust`, `audit`, or `internal`. Defaults to `trust`.

### Platform support

linux/amd64, linux/arm64, darwin/amd64, darwin/arm64.
