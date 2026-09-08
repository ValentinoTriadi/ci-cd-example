#!/usr/bin/env bash
#
# Enforce a minimum total statement coverage.
#
# Usage: scripts/coverage.sh [profile]        (default: coverage.out)
# Env:   COVERAGE_MIN  minimum percentage, default 80
set -euo pipefail

PROFILE="${1:-coverage.out}"
MIN="${COVERAGE_MIN:-80}"

if [[ ! -f "$PROFILE" ]]; then
  echo "::error::coverage profile '$PROFILE' not found; run the tests first" >&2
  exit 1
fi

TOTAL="$(go tool cover -func="$PROFILE" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')"

if [[ -z "$TOTAL" ]]; then
  echo "::error::could not parse a total from '$PROFILE'" >&2
  exit 1
fi

# awk rather than bash arithmetic, because these are floats.
PASSED="$(awk -v total="$TOTAL" -v min="$MIN" 'BEGIN { print (total >= min) ? "yes" : "no" }')"

report() {
  printf '%s\n' "$1"
  if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
    printf '%s\n' "$1" >>"$GITHUB_STEP_SUMMARY"
  fi
}

if [[ "$PASSED" == "yes" ]]; then
  report "### ✅ Coverage ${TOTAL}% (minimum ${MIN}%)"
  exit 0
fi

report "### ❌ Coverage ${TOTAL}% is below the ${MIN}% minimum"
echo "::error::coverage ${TOTAL}% < ${MIN}%" >&2

# Show the least-covered functions so the failure is actionable.
echo "Least-covered functions:" >&2
go tool cover -func="$PROFILE" | grep -v '^total:' | sort -t$'\t' -k3 -n | head -10 >&2

exit 1
