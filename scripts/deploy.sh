#!/usr/bin/env bash
#
# DEPLOY STUB.
#
# This prints the deployment it *would* perform and exits 0. It exists so the
# whole pipeline is green without any cloud credentials. Everything around it
# — environments, approvals, concurrency, image digests — is real.
#
# To make it real, replace the "execute" section below with your own tooling
# (kubectl / helm / flyctl / aws ecs / terraform). The interface stays the same.
#
# Usage:
#   scripts/deploy.sh --env staging --image ghcr.io/owner/app@sha256:... [--ref main] [--dry-run]
set -euo pipefail

ENVIRONMENT=""
IMAGE=""
REF="${GITHUB_SHA:-local}"
DRY_RUN="false"

VALID_ENVIRONMENTS=("development" "staging" "production")

usage() {
  cat <<'USAGE'
Usage: deploy.sh --env <environment> --image <image-ref> [--ref <git-ref>] [--dry-run]

  --env     Target environment: development | staging | production
  --image   Fully qualified image reference (tag or, preferably, @sha256 digest)
  --ref     Git ref being deployed (defaults to $GITHUB_SHA)
  --dry-run Print the plan without the (simulated) apply
USAGE
}

fail() {
  echo "::error::$*" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env)     ENVIRONMENT="${2:-}"; shift 2 ;;
    --image)   IMAGE="${2:-}";       shift 2 ;;
    --ref)     REF="${2:-}";         shift 2 ;;
    --dry-run) DRY_RUN="true";       shift   ;;
    -h|--help) usage; exit 0 ;;
    *)         usage >&2; fail "unknown argument: $1" ;;
  esac
done

# --- validate -------------------------------------------------------------
# Bad input exits non-zero on purpose, so the "deploy failed" path in the
# pipeline is demonstrable rather than theoretical.

[[ -n "$ENVIRONMENT" ]] || { usage >&2; fail "--env is required"; }
[[ -n "$IMAGE" ]]       || { usage >&2; fail "--image is required"; }

valid="false"
for candidate in "${VALID_ENVIRONMENTS[@]}"; do
  [[ "$ENVIRONMENT" == "$candidate" ]] && valid="true"
done
[[ "$valid" == "true" ]] || fail "invalid environment '${ENVIRONMENT}' (want one of: ${VALID_ENVIRONMENTS[*]})"

[[ "$IMAGE" == *:* || "$IMAGE" == *@* ]] || fail "image '${IMAGE}' has no tag or digest"

# Production must be deployed from an immutable digest, never a moving tag.
if [[ "$ENVIRONMENT" == "production" && "$IMAGE" != *@sha256:* ]]; then
  echo "::warning::deploying production from a mutable tag; prefer an @sha256 digest"
fi

case "$ENVIRONMENT" in
  development) REPLICAS=1; HOST="dev.example.internal" ;;
  staging)     REPLICAS=2; HOST="staging.example.internal" ;;
  production)  REPLICAS=4; HOST="app.example.com" ;;
esac

# --- plan -----------------------------------------------------------------

cat <<PLAN
────────────────────────────────────────────────────────────
 Deployment plan
────────────────────────────────────────────────────────────
 Environment : ${ENVIRONMENT}
 Image       : ${IMAGE}
 Git ref     : ${REF}
 Replicas    : ${REPLICAS}
 Hostname    : ${HOST}
 Actor       : ${GITHUB_ACTOR:-$(whoami)}
 Dry run     : ${DRY_RUN}
────────────────────────────────────────────────────────────
PLAN

steps=(
  "pull ${IMAGE}"
  "render manifests for ${ENVIRONMENT} (replicas=${REPLICAS}, host=${HOST})"
  "apply manifests"
  "wait for rollout to become healthy"
  "run post-deploy smoke test against https://${HOST}"
)

if [[ "$DRY_RUN" == "true" ]]; then
  echo "Dry run requested; stopping before apply."
  exit 0
fi

# --- execute (simulated) --------------------------------------------------
# Replace this loop with your real deployment commands.

for i in "${!steps[@]}"; do
  printf '  [%d/%d] %s\n' "$((i + 1))" "${#steps[@]}" "${steps[$i]}"
done

echo "Simulated deployment to ${ENVIRONMENT} completed."

# --- report ---------------------------------------------------------------

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "### 🚀 Deployed to \`${ENVIRONMENT}\`"
    echo ""
    echo "| Field | Value |"
    echo "| --- | --- |"
    echo "| Image | \`${IMAGE}\` |"
    echo "| Ref | \`${REF}\` |"
    echo "| Replicas | ${REPLICAS} |"
    echo "| URL | https://${HOST} |"
    echo ""
    echo "> This is a simulated deploy. See \`scripts/deploy.sh\` to wire in real tooling."
  } >>"$GITHUB_STEP_SUMMARY"
fi

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  {
    echo "environment=${ENVIRONMENT}"
    echo "image=${IMAGE}"
    echo "url=https://${HOST}"
  } >>"$GITHUB_OUTPUT"
fi
