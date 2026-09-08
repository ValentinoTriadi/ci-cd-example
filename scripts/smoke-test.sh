#!/usr/bin/env bash
#
# Verify a running instance answers its probes correctly.
#
# Usage: scripts/smoke-test.sh [base-url]     (default: http://localhost:8080)
# Env:   SMOKE_RETRIES (default 30), SMOKE_DELAY seconds (default 1)
set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
RETRIES="${SMOKE_RETRIES:-30}"
DELAY="${SMOKE_DELAY:-1}"

fail() { echo "::error::$*" >&2; exit 1; }

echo "Smoke-testing ${BASE_URL}"

# 1. Wait for the service to accept connections at all.
for ((attempt = 1; attempt <= RETRIES; attempt++)); do
  if curl -fsS --max-time 5 "${BASE_URL}/healthz" >/dev/null 2>&1; then
    echo "  ✓ /healthz responded after ${attempt} attempt(s)"
    break
  fi
  if ((attempt == RETRIES)); then
    fail "service never became healthy after ${RETRIES} attempts"
  fi
  sleep "$DELAY"
done

# 2. Readiness must report ready, not draining.
READY="$(curl -fsS --max-time 5 "${BASE_URL}/readyz")"
grep -q '"ready"' <<<"$READY" || fail "/readyz did not report ready: ${READY}"
echo "  ✓ /readyz reports ready"

# 3. Build metadata must be present.
VERSION_JSON="$(curl -fsS --max-time 5 "${BASE_URL}/version")"
grep -q '"version"' <<<"$VERSION_JSON" || fail "/version returned no version field: ${VERSION_JSON}"
echo "  ✓ /version -> ${VERSION_JSON}"

# 4. Exercise a real write path, not just the probes.
CREATED="$(curl -fsS --max-time 5 -X POST "${BASE_URL}/api/todos" \
  -H 'Content-Type: application/json' \
  -d '{"title":"smoke test"}')"
grep -q '"smoke test"' <<<"$CREATED" || fail "POST /api/todos failed: ${CREATED}"
echo "  ✓ POST /api/todos created a todo"

LISTED="$(curl -fsS --max-time 5 "${BASE_URL}/api/todos")"
grep -q '"smoke test"' <<<"$LISTED" || fail "GET /api/todos did not return the new todo: ${LISTED}"
echo "  ✓ GET /api/todos returns it"

STATS="$(curl -fsS --max-time 5 "${BASE_URL}/api/todos/stats")"
grep -q '"total":1' <<<"$STATS" || fail "GET /api/todos/stats did not count the new todo: ${STATS}"
echo "  ✓ GET /api/todos/stats -> ${STATS}"

# 5. A bad request must still be rejected correctly.
STATUS="$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "${BASE_URL}/api/todos/999")"
[[ "$STATUS" == "404" ]] || fail "expected 404 for a missing todo, got ${STATUS}"
echo "  ✓ missing todo returns 404"

echo "Smoke test passed against ${BASE_URL}"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "### ✅ Smoke test passed"
    echo ""
    echo "Target: \`${BASE_URL}\`"
    echo ""
    echo '```json'
    echo "${VERSION_JSON}"
    echo '```'
  } >>"$GITHUB_STEP_SUMMARY"
fi
