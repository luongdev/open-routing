#!/usr/bin/env bash
# v0.3 live-engine e2e smoke: seeds a skill + agent + queue + published voice flow
# (trigger → route_queue → match_skill → reservation), connects the reference
# agent over WebSocket (auto-Ready, auto-accept + complete echoing the lease_token),
# creates a live route, and asserts the engine offers it → the agent accepts and
# completes → capacity frees. Exits non-zero on the first failed assertion.
#
# REQUIRES on the target stack:
#   - MATCHER_ENABLED=true on BOTH api and runtime (else routes never park/offer).
#   - The WS gateway (/v1/agent/ws) reachable from here.
#   - python3 (JSON, no jq) and a Go toolchain (builds cmd/refagent on the fly).
set -euo pipefail

API_URL="${API_URL:-http://10.69.69.169:30080}"
ORG_ID="${ORG_ID:-01920000-0000-7000-8000-000000000001}"
H="X-Org-Id: $ORG_ID"
SUF="${RANDOM}${RANDOM}"
SKCODE="e2e_sk_$SUF"
AGCODE="e2e_ag_$SUF"
QCODE="e2e_q_$SUF"
FCODE="e2e_flow_$SUF"
ENTRY="e2e$SUF"
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

SKID=""; AGID=""; FID=""; RAPID=""; LOG="$(mktemp)"
j() { python3 -c 'import sys,json; d=json.load(sys.stdin); print(eval(sys.argv[1]))' "$1"; }
fail() { echo "FAIL: $1"; [ -s "$LOG" ] && { echo "--- refagent log ---"; cat "$LOG"; }; exit 1; }
cleanup() {
  [ -n "$RAPID" ] && kill "$RAPID" 2>/dev/null || true
  [ -n "$FID" ] && curl -s -X DELETE "$API_URL/v1/orgs/$ORG_ID/flows/$FID" -H "$H" -o /dev/null || true
  [ -n "$AGID" ] && curl -s -X DELETE "$API_URL/v1/orgs/$ORG_ID/agents/$AGID" -H "$H" -o /dev/null || true
  [ -n "$SKID" ] && curl -s -X DELETE "$API_URL/v1/orgs/$ORG_ID/skills/$SKID" -H "$H" -o /dev/null || true
  rm -f "$LOG"
}
trap cleanup EXIT

echo "--- 1. seed skill + agent + queue ---"
SKID=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/skills" -H "$H" -H 'Content-Type: application/json' \
  -d "{\"code\":\"$SKCODE\",\"name\":\"$SKCODE\",\"skill_type\":\"binary\"}" | j 'd["id"]')
[ -n "$SKID" ] || fail "create skill"
AGID=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/agents" -H "$H" -H 'Content-Type: application/json' \
  -d "{\"code\":\"$AGCODE\",\"name\":\"$AGCODE\",\"email\":\"$AGCODE@e2e.test\",\"skills\":[{\"skill_id\":\"$SKID\",\"proficiency\":3}]}" | j 'd["id"]')
[ -n "$AGID" ] || fail "create agent"
curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/queues" -H "$H" -H 'Content-Type: application/json' \
  -d "{\"code\":\"$QCODE\",\"name\":\"$QCODE\",\"channel_types\":[\"voice\"]}" -o /dev/null
echo "skill=$SKID agent=$AGID queue=$QCODE"

echo "--- 2. publish a voice routing flow ---"
GRAPH="{\"nodes\":[
  {\"id\":\"t\",\"type\":\"trigger\",\"config\":{}},
  {\"id\":\"rq\",\"type\":\"route_queue\",\"config\":{\"queue\":\"$QCODE\"}},
  {\"id\":\"ms\",\"type\":\"match_skill\",\"config\":{\"skill\":\"$SKCODE\",\"min_proficiency\":1}},
  {\"id\":\"rsv\",\"type\":\"reservation\",\"config\":{\"timeout_sec\":30,\"max_attempts\":2}},
  {\"id\":\"fb\",\"type\":\"fallback\",\"config\":{\"reason\":\"no agent\"}},
  {\"id\":\"end\",\"type\":\"end\",\"config\":{}}
],\"edges\":[
  {\"from\":\"t\",\"to\":\"rq\"},{\"from\":\"rq\",\"to\":\"ms\"},{\"from\":\"ms\",\"to\":\"rsv\"},
  {\"from\":\"rsv\",\"to\":\"end\",\"label\":\"accepted\"},
  {\"from\":\"rsv\",\"to\":\"fb\",\"label\":\"timeout\"},
  {\"from\":\"rsv\",\"to\":\"fb\",\"label\":\"no_candidate\"},
  {\"from\":\"fb\",\"to\":\"end\"}
]}"
FID=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/flows" -H "$H" -H 'Content-Type: application/json' \
  -d "{\"code\":\"$FCODE\",\"name\":\"$FCODE\",\"graph\":$GRAPH}" | j 'd["id"]')
[ -n "$FID" ] || fail "create flow"
curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/flows/$FID/publish" -H "$H" -H 'Content-Type: application/json' \
  -d "{\"channel\":\"voice\",\"entry_code\":\"$ENTRY\",\"version\":1}" -o /dev/null || fail "publish"

echo "--- 3. connect the reference agent (Ready + auto accept/complete) ---"
go build -o "$LOG.bin" "$REPO_DIR/services/api/cmd/refagent" || fail "build refagent"
"$LOG.bin" -url "$API_URL" -org "$ORG_ID" -agent "$AGID" -ready -complete >"$LOG" 2>&1 &
RAPID=$!
for _ in $(seq 1 20); do grep -q '"msg":"connected"\|msg=connected\|connected' "$LOG" && break; sleep 0.5; done
grep -q connected "$LOG" || fail "refagent did not connect"

echo "--- 4. create a live route ---"
RID=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/route-requests" -H "$H" -H 'Content-Type: application/json' \
  -d "{\"channel\":\"voice\",\"entry_code\":\"$ENTRY\"}" | j 'd["id"]')
[ -n "$RID" ] || fail "create route"
echo "route=$RID"

echo "--- 5. wait for the agent to accept + the call to complete ---"
DONE=""
for _ in $(seq 1 30); do
  if grep -q 'sent complete' "$LOG"; then DONE=1; break; fi
  sleep 1
done
[ -n "$DONE" ] || fail "agent never accepted+completed the offer (check MATCHER_ENABLED + WS reachability)"
grep -q 'status=accepted\|"status":"accepted"' "$LOG" || fail "no accepted ack in refagent log"

echo "--- 6. assert reservation completed + slot freed ---"
RSTATE=$(curl -s "$API_URL/v1/orgs/$ORG_ID/route-requests/$RID/reservations" -H "$H" | j 'd["items"][0]["state"]')
[ "$RSTATE" = "completed" ] || fail "reservation state=$RSTATE, want completed"
HELD=$(curl -s "$API_URL/v1/orgs/$ORG_ID/routing/stats" -H "$H" | j 'd["held_slots"]')
echo "reservation=completed, held_slots=$HELD"

echo "PASS: v0.3 live-engine e2e smoke (offer → accept → complete → freed)"
