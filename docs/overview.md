epack-collector-sentry is an epack collector built with the [Component SDK](https://github.com/locktivity/epack).

## What it does

Collects application monitoring posture evidence from a Sentry organization. It reads cron monitors, alert rules, users, and teams to produce evidence that production systems are monitored and anomalies trigger response.

## How it works

The collector authenticates with a personal auth token or internal integration token, then reads the Sentry API to gather monitoring configuration. Data is gated by collection level: trust provides a single boolean indicating whether monitoring is configured, audit adds aggregate counts and per-entity inventories, and internal adds cross-entity detail like team memberships and integration IDs.
