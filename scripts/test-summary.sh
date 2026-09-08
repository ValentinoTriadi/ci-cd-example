#!/usr/bin/env bash
#
# Turn `go test -json` output into a readable job summary.
#
# Usage: scripts/test-summary.sh <test-output.json> [label]
#
# jq is preinstalled on GitHub-hosted runners; locally, install it or skip this.
set -euo pipefail

INPUT="${1:?usage: test-summary.sh <test-output.json> [label]}"
LABEL="${2:-Tests}"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq not found; skipping test summary" >&2
  exit 0
fi

[[ -f "$INPUT" ]] || { echo "::error::$INPUT not found" >&2; exit 1; }

count_action() {
  jq -r --arg action "$1" \
    'select(.Test != null and .Action == $action) | .Test' "$INPUT" | wc -l | tr -d ' '
}

PASSED="$(count_action pass)"
FAILED="$(count_action fail)"
SKIPPED="$(count_action skip)"
ELAPSED="$(jq -r 'select(.Test == null and .Elapsed != null) | .Elapsed' "$INPUT" |
  awk '{sum += $1} END {printf "%.1f", sum + 0}')"

emit() {
  printf '%s\n' "$@"
  if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
    printf '%s\n' "$@" >>"$GITHUB_STEP_SUMMARY"
  fi
}

icon="✅"
[[ "$FAILED" != "0" ]] && icon="❌"

emit "### ${icon} ${LABEL}" \
  "" \
  "| Passed | Failed | Skipped | Package time |" \
  "| ---: | ---: | ---: | ---: |" \
  "| ${PASSED} | ${FAILED} | ${SKIPPED} | ${ELAPSED}s |"

if [[ "$FAILED" != "0" ]]; then
  emit "" "<details><summary>Failed tests</summary>" ""
  while IFS= read -r line; do
    emit "- \`${line}\`"
  done < <(jq -r 'select(.Test != null and .Action == "fail") | "\(.Package)  \(.Test)"' "$INPUT")
  emit "" "</details>"
fi
