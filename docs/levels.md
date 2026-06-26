## Collection Levels

The Sentry collector supports three collection levels that control how much data is gathered. Set the level in your `epack.yaml` config:

```yaml
config:
  level: trust   # or audit, internal
```

### trust (default)

Minimal data for establishing whether monitoring exists.

| Section | Fields |
|---------|--------|
| `monitoring_summary` | `alerts_or_monitors_enabled` (boolean) |

No inventories, counts, user data, or team data are collected.

### audit

Aggregated counts and per-item inventories for compliance review.

| Section | Fields |
|---------|--------|
| `monitoring_summary` | `alerts_or_monitors_enabled`, `total_monitoring_rules`, `cron_monitors`, `alert_rules`, `with_actions_pct` |
| `monitors` | `total_count`, `active_count`, `active_pct`, `with_alert_targets_pct`, inventory (id, name, status, schedule, project, owner, connected workflows) |
| `alert_rules` | `total_count`, `metric_alerts`, `issue_alerts`, `with_actions_pct`, inventory (id, name, type, aggregate, projects, environment, owner, triggers) |
| `users` | `total_count`, `active_count`, `with_2fa_pct`, `with_sso_pct`, `admin_count`, inventory (id, email, role, active, 2fa, sso) |
| `teams` | `total_count`, inventory (slug, name, member_count, project_count) |

### internal

Everything from audit, plus cross-entity detail for internal analysis.

| Additional fields | Description |
|-------------------|-------------|
| `monitors[].open_alert_count` | Count of unresolved incidents per monitor |
| `alert_rules[].triggers[].actions[].integration_id` | Integration identifiers on alert actions |
| `users[].team_memberships` | Which teams each user belongs to and their role |
| `teams[].members` | Full member list per team with roles |

### Required Sentry scopes by level

| Scope | Levels | Surface |
|-------|--------|---------|
| `org:read` | trust+ | Organization metadata |
| `alerts:read` | trust+ | Monitors and alert rules |
| `member:read` | audit+ | Organization members |
| `team:read` | audit+ | Teams and team members |
