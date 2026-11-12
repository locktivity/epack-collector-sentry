epack-collector-sentry is an epack collector built with the [Component SDK](https://github.com/locktivity/epack).

## What it does

Collects application monitoring posture evidence from a Sentry organization. It reads cron monitors, alert rules, users, and teams to produce evidence that production systems are monitored and anomalies trigger response.

Each run also writes `artifacts/sentry.access-posture.json` in the `evidencepack/access-posture@v1` format. It covers the organization's sign-in rules, members, admins, and connected apps. Admins are the members with the owner or manager role. With Require SSO off, members linked to SSO also count as signing in without SSO. The file has no `root` or `credentials` section. Sentry has no root account. Organization auth tokens can be listed only from a browser session or an OAuth app token, which the collector does not use.

## Not collected

Sentry offers the following through its API, and the collector does not read them.

- **Data scrubbing.** Sentry returns the data scrubbing settings of the organization and of each project, such as whether the default scrubbers run and whether IP addresses are stored.

## How it works

The collector authenticates with a personal auth token or internal integration token, then reads the Sentry API to gather monitoring configuration. It first reads the organization, which checks the token: only that read, or invalid configuration, can fail the run. A value it cannot read after that is left out of the artifact and listed, with the reason, in `artifacts/sentry.collector-result.json`. Data is gated by collection level: trust provides a single boolean indicating whether monitoring is configured, audit adds aggregate counts and per-entity inventories, and internal adds cross-entity detail like team memberships and integration IDs. In the access file, trust has the `*_present` flags and the percentages, and audit adds the counts behind them.
