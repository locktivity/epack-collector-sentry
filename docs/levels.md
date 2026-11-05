## Collection Levels

The Sentry collector supports three collection levels that control how much data is gathered. Set the level in your `epack.yaml` config:

```yaml
config:
  level: trust   # or audit, internal
```

### trust (default)

Minimal data for establishing whether monitoring exists and how authentication is enforced.

| Section | Fields |
|---------|--------|
| `auth` | `sso_configured`, `sso_provider`, `sso_required`, `scim_enabled` |
| `monitoring_summary` | `alerts_or_monitors_enabled` (boolean) |

`auth` reports the org's SSO posture: whether an auth provider is configured, which one (`okta`, `saml2`, `google`, ...), whether members must sign in through it (`sso_required`, Sentry's "Require SSO" setting), and whether SCIM provisioning is enabled. The fields are null when the lookup fails, so "unknown" is distinguishable from "not configured"; a diagnostic warning is recorded in that case. `auth` appears at every level.

No inventories, counts, user data, or team data are collected at trust.

### audit

Aggregated counts and per-item inventories for compliance review.

| Section | Fields |
|---------|--------|
| `monitoring_summary` | `alerts_or_monitors_enabled`, `total_monitoring_rules`, `cron_monitors`, `alert_rules`, `with_actions_pct` |
| `monitors` | `total_count`, `active_count`, `active_pct`, `with_alert_targets_pct`, inventory (id, name, status, schedule, project, owner, connected workflows) |
| `alert_rules` | `total_count`, `metric_alerts`, `issue_alerts`, `with_actions_pct`, inventory (id, name, type, aggregate, projects, environment, owner, triggers) |
| `users` | `total_count`, `active_count`, `with_2fa_pct`, `with_sso_pct`, `admin_count`, inventory (id, email, role, active, 2fa, sso) |
| `teams` | `total_count`, inventory (slug, name, member_count, project_count) |

Each inventory holds at most 10,000 entries. When a list is longer, the counts still cover every entry, and the section has `inventory_truncated: true`.

### internal

Everything from audit, plus cross-entity detail for internal analysis.

| Additional fields | Description |
|-------------------|-------------|
| `monitors[].open_alert_count` | Count of unresolved incidents per monitor |
| `alert_rules[].triggers[].actions[].integration_id` | Integration identifiers on alert actions |
| `users[].team_memberships` | Which teams each user belongs to and their role |
| `teams[].members` | Full member list per team with roles |

### The access file

`artifacts/sentry.access-posture.json` follows `evidencepack/access-posture@v1` at every level. Trust has the sign-in rules, the `*_present` flags, and the percentages. Audit adds the counts behind them, such as `members.member_count` and `admins.local_admin_count`. Internal adds nothing more.

Local members are the members who can sign in without SSO. A member is counted as local when they are not linked to the organization's SSO provider. With Require SSO off, members linked to SSO are counted as local too, since they can still sign in with a password. If the SSO settings can't be read, the local values that depend on a linked member are left out with a gap.

### Required Sentry scopes by level

| Scope | Levels | Surface |
|-------|--------|---------|
| `org:read` | trust+ | Organization metadata, SSO posture, projects, integrations, and Sentry app installations. From audit, teams. |
| `alerts:read` | trust+ | Monitors and alert rules |
| `member:read` | trust+ | Organization members. A trust run reads them for the access file only. |
| `team:read` | internal | Team members |

A missing scope other than `org:read` leaves out the values it covers instead of failing the run. See [configuration](configuration.md).
