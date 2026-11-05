#!/usr/bin/env bash
#
# Lint: verify that trust-level output contains only the expected fields.
# Usage: scripts/check-forbidden-data.sh [trust-output.json]
#
# Without a file, it checks the trust-level output the collector produces
# from the test suite's canned Sentry responses.
#
# The trust level should only contain:
#   schema_version, collected_at, collected_at_level, organization, auth,
#   monitoring_summary (with only alerts_or_monitors_enabled), diagnostics
#
# Any monitors, alert_rules, users, or teams sections indicate a data leak.

set -euo pipefail

if [ $# -ge 1 ]; then
  FILE="$1"
else
  cd "$(dirname "$0")/.."
  FILE="$(mktemp)"
  trap 'rm -f "$FILE"' EXIT
  SENTRY_TRUST_OUTPUT="$FILE" go test ./internal/collector -run '^TestTrustOutputFile$' -count=1 > /dev/null
fi

if [ ! -f "$FILE" ]; then
  echo "Error: file not found: $FILE"
  exit 1
fi

ERRORS=0

check_absent() {
  local field="$1"
  if jq -e ".$field" "$FILE" > /dev/null 2>&1; then
    echo "FAIL: trust output contains forbidden field '$field'"
    ERRORS=$((ERRORS + 1))
  fi
}

check_absent "monitors"
check_absent "alert_rules"
check_absent "users"
check_absent "teams"
check_absent "monitoring_summary.total_monitoring_rules"
check_absent "monitoring_summary.cron_monitors"
check_absent "monitoring_summary.alert_rules"
check_absent "monitoring_summary.with_actions_pct"

if [ "$ERRORS" -gt 0 ]; then
  echo ""
  echo "$ERRORS forbidden field(s) found in trust-level output."
  exit 1
fi

echo "OK: trust-level output contains no forbidden fields."
