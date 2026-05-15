#!/usr/bin/env bash
# Phase 1 / Plan 06 Task 5 — local smoke for cmd/api end-to-end.
#
# Usage:
#   bash services/api/scripts/smoke.sh
#
# Prerequisites (operator must run first; NOT enforced by this script):
#   1. docker compose up -d postgres redis    # script assumes both are healthy
#   2. task migrate-up                        # script assumes schema is current
#
# Exit codes:
#   0 — every assertion passed
#   1 — first failed assertion (prints "FAIL: ..." and dumps /tmp/api.log)
#
# Why no `set -e`: we want per-assertion FAIL messages and a final summary,
# not a silent exit on the first nonzero command. The cleanup trap handles
# the API process lifecycle regardless of where execution stops.
#
# Why a real readiness loop (B-4 fix): an inline `( ... & )` subshell loses
# the API_PID — the parent shell sees the subshell's PID, not the actual
# binary's PID, so cleanup can't reach the API. We launch the binary directly
# with `&` and capture $!.
#
# Why `grep -qE PATTERN file` (W-3 fix): piping the unsilenced form into a
# pager swallows the grep exit code so a missing match falsely succeeds.
# Each log assertion below uses `grep -qE file` directly so a missing match
# returns nonzero immediately.
set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$REPO_ROOT"

cleanup() {
    if [ -n "${API_PID:-}" ]; then
        kill "$API_PID" 2>/dev/null || true
        wait "$API_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

# ---- Build the API binary ----
(cd services/api && go build -o /tmp/open-routing-api ./cmd/api) || { echo "FAIL: build"; exit 1; }

# ---- Launch in background ----
DATABASE_URL="postgres://openrouting:openrouting@localhost:5432/openrouting?sslmode=disable" \
REDIS_URL="redis://localhost:6379/0" \
OTEL_EXPORTER=stdout \
LISTEN_ADDR=:8080 \
ORGDB_VALIDATION_MODE=panic \
/tmp/open-routing-api > /tmp/api.log 2>&1 &
API_PID=$!

# ---- Readiness loop: up to 30s ----
for i in $(seq 1 30); do
    if curl -fsS http://localhost:8080/healthz > /dev/null 2>&1; then
        break
    fi
    sleep 1
done

# ---- Determine ORG (fresh UUIDv7) ----
# Try a tiny Go helper, then python's uuid7 (only available in 3.13+),
# else fall back to a canned RFC 9562 §5.7-compliant string. The canned
# string passes uuid.Parse + Version()==7 + Variant()==VariantRFC4122 so
# the API's OrgContext middleware accepts it for the smoke.
ORG=""
if command -v go > /dev/null 2>&1; then
    ORG=$(printf 'package main\nimport ("fmt"; "github.com/google/uuid")\nfunc main(){ id := uuid.Must(uuid.NewV7()); fmt.Print(id.String()) }\n' \
          | (cd services/api && go run /dev/stdin) 2>/dev/null || true)
fi
if [ -z "$ORG" ]; then
    ORG=$(python3 -c "import uuid; print(uuid.uuid7())" 2>/dev/null || true)
fi
if [ -z "$ORG" ]; then
    # Canned UUIDv7: version nibble = 7 (3rd group starts with 7), variant
    # nibble = 8 (4th group starts with 8). RFC 9562 §5.7-conformant.
    ORG="01934567-0000-7000-8000-000000000001"
fi

# ---- HTTP smoke (each assertion exits on failure) ----
HEALTH=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/healthz)
[ "$HEALTH" = "200" ] || { echo "FAIL: /healthz returned $HEALTH"; cat /tmp/api.log; exit 1; }

READY_BODY=$(curl -s http://localhost:8080/readyz)
# /readyz may be 503 if redis/db not running; check the JSON status field
# rather than the HTTP code so missing-infrastructure surfaces as a clear
# WARN rather than a hard FAIL — the smoke proves D-21 (no org-gate) and
# FOUND-10 (single-command bring-up), not docker-compose presence.
case "$READY_BODY" in
    *'"status":"ok"'*)        : ;;
    *'"status":"degraded"'*)  echo "WARN: /readyz=degraded body=$READY_BODY (acceptable when DB/Redis are absent)";;
    *)                        echo "FAIL: /readyz unexpected body $READY_BODY"; cat /tmp/api.log; exit 1;;
esac

SCAFFOLD_MISSING=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/v1/orgs/$ORG/_scaffold")
[ "$SCAFFOLD_MISSING" = "400" ] || { echo "FAIL: missing X-Org-Id returned $SCAFFOLD_MISSING (expected 400)"; cat /tmp/api.log; exit 1; }

SCAFFOLD_BAD=$(curl -s -o /dev/null -w "%{http_code}" -H "X-Org-Id: not-a-uuid" "http://localhost:8080/v1/orgs/$ORG/_scaffold")
[ "$SCAFFOLD_BAD" = "400" ] || { echo "FAIL: malformed X-Org-Id returned $SCAFFOLD_BAD (expected 400)"; cat /tmp/api.log; exit 1; }

# ---- Log assertions: each grep must succeed on its own (W-3 fix) ----
grep -qE '"trace_id":"[0-9a-f]{32}"' /tmp/api.log || { echo "FAIL: no trace_id in /tmp/api.log"; cat /tmp/api.log; exit 1; }
grep -qE '"span_id":"[0-9a-f]{16}"' /tmp/api.log || { echo "FAIL: no span_id in /tmp/api.log"; cat /tmp/api.log; exit 1; }

echo "smoke ok: HEALTH=$HEALTH READY=$READY_BODY MISSING=$SCAFFOLD_MISSING BAD=$SCAFFOLD_BAD"
exit 0
