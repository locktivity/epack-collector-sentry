# Changelog

## Unreleased

- The collector is built on Go 1.26.11.

## v0.3.1

- The collector is built on epack v0.10.5.
- The collector result's `targets` says what the run was pointed at: the organization, chosen by `organization`, at `organization` in `artifacts/sentry.details.json`.

## v0.3.0

- Each run writes `artifacts/sentry.access-posture.json`, in the `evidencepack/access-posture@v1` format, and `collect` lists it as `access-posture`. It has the organization's sign-in rules, members, admins, and connected apps. Its `sign_in.sso_configured`, `sso_required`, and `scim_enabled` copy the `auth` values. `sign_in.mfa_required` is the organization's require-2FA setting.
- In the access file, pending invitations are not counted as members. Admins are the members with the owner or manager role. A member is inactive when their Sentry account is deactivated, or when they were last active more than 90 days before the run.
- In the access file, a member not linked to the SSO provider counts as signing in without SSO. With Require SSO off, members linked to SSO count too, since they can still sign in with a password. When the SSO settings can't be read, the `local_*` values that depend on a linked member are left out with the read's gap.
- The access file's `connected_apps` counts the organization's active integrations and its installed Sentry apps, including internal integrations. The collector reads them with `org:read`.
- The access file has no `root` section and no external member values. Each is left out with an `unsupported` gap, since Sentry has no root account and no external member type.
- The access file has no `credentials` section. It is left out with a `not_read` gap. Sentry lists organization auth tokens only for a browser session or an OAuth app token with `org:tokens`, and the collector signs in with a personal or internal integration token.
- A trust run now reads the organization's members, integrations, and Sentry app installations for the access file, so it needs `member:read`. A trust run with `collect: [details]` makes none of these calls.
- A gap left by a call that answered with an error status, or with a body that does not decode, now names that call. A gap left by a call the token lacks a scope for names that scope as the missing permission, such as `member:read` for the members list. Each scope is the one Sentry's API docs list for the call.
- The teams list needs `org:read`, as Sentry's API docs say. `team:read` is needed only at internal, for each team's members. When the teams list is forbidden, the diagnostic warning now names `org:read` instead of `team:read`.
- The detailed artifact moves from `artifacts/sentry.monitoring.json` to `artifacts/sentry.details.json`, and `collect` lists it as `details`. A run configured under another name in `epack.yaml`, such as `sentry-eu`, writes its files under that name, such as `artifacts/sentry-eu.details.json`.
- New `auth` section, emitted at every level: `sso_configured`, `sso_provider`, `sso_required` (Sentry's "Require SSO" setting), and `scim_enabled`. Fields are null with a diagnostic warning when the lookup fails, so unknown is distinguishable from not configured. Uses the existing `org:read` scope.
- Built on the epack SDK's HTTP client and test kit. Calls are retried on the SDK's policy, and each run adds `artifacts/sentry.collector-result.json`, which lists the values left out with a code, the calls made, and the organization signed in to. The one second pause when Sentry reported few remaining requests is gone.
- The run first reads the organization to check the token, and fails only for invalid configuration or a failed sign-in: exit 2 for invalid configuration, now including a missing `SENTRY_AUTH_TOKEN` (previously exit 3) and an organization slug Sentry does not know (previously exit 4), and exit 3 for a rejected token or a token without `org:read`. A failed read after the sign-in no longer fails the run: the values it would have given are left out and listed in the collector result.
- `monitoring_summary.alerts_or_monitors_enabled` is null when neither the monitors nor the alert rules showed an entry and one of them could not be read.
- At audit, `monitoring_summary.with_actions_pct` is null when there are no monitoring rules, and an empty inventory is `[]`. At internal, a team with no members and a user in no team have `[]`.
- An inventory cut to its first 10,000 entries now has `inventory_truncated: true` beside it. The counts still cover every entry, and `diagnostics.truncation` still describes the cut.
- A response without its list, or an auth-provider response without its fields, is recorded as `invalid_response` instead of being read as empty or false. Alert rules no longer show project IDs in place of slugs when the projects list cannot be read; they are left out instead.
- `scripts/check-forbidden-data.sh` runs without arguments, against the trust output the test fixtures produce.
- The schema gives each field a label and a summary for readers of the pack, as `x-epack-label` and `x-epack-summary`. Each value a trust run writes also has an `x-epack-method` sentence on how it is measured.
- Each percentage in `monitors`, `alert_rules` and `users` points to its count with `x-epack-numerator`. Every percentage is marked `x-epack-rounding: floor`, since each one rounds down.
- Each pack holds the collector's build file, `artifacts/sentry.collector-build.json`, with its schema and its trace map. The trace map in `docs/schema/trace-map.json` links each field to the calls and code behind it. `make trace-map` rewrites it.
- With epack v0.10.0, the collector result below the audit level has no organization and no call counts. At every level, its operations have `retried`, `status_codes` and `error_codes`.
- Alert rules no longer include the error monitor and issue stream monitor that every project has by default. One of these monitors counts as an issue alert only when an alert is connected to it. Uptime monitors and other monitor types are no longer counted as alert rules. For an organization with only these monitors, `monitoring_summary.alerts_or_monitors_enabled` is now false.
- An alert rule's `time_window_minutes` is now in minutes. It used to hold the value in seconds, so a 4 hour window showed as 14400.
- A static or change metric alert that fires below its threshold now has `threshold_type: below`. Every static alert used to show `above`.
- An action's `target_type` is now the recipient type set in Sentry. An email action has `user`, `team` or `issue_owners`, and an integration action has `specific`. Every action used to show `specific`.
- An issue alert's `environment` now comes from the alerts connected to it in Sentry. When those alerts name different environments, it is left out.
- `organization` is now the organization's slug when the configuration gives its ID. It used to be the configured ID.
- A team's `project_count` is now the number of projects the team is assigned to. It used to be 0 for every team.
- At internal, a team member's `user_id` is now the id of the member's Sentry user. It used to hold the organization membership id, which the users inventory lists as `id`. It is null for a member who was invited and has not joined yet.

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
