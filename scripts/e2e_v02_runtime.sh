#!/usr/bin/env bash
# v0.2 runtime e2e smoke: drives a published flow through the runtime end to end —
# create draft → validate → publish → simulate (interpolated side-effect + a
# scripted captured-input branch) → live route → submit input → trace.
# Hermetic: needs no pre-seeded agents. Exits non-zero on the first failed
# assertion. Uses python3 (no jq dependency).
set -euo pipefail

API_URL="${API_URL:-http://10.69.69.169:30080}"
ORG_ID="${ORG_ID:-01920000-0000-7000-8000-000000000001}"
H="X-Org-Id: $ORG_ID"
SUF="${RANDOM}${RANDOM}"
CODE="e2e_rt_$SUF"
ENTRY="e2e$SUF"
FID=""

# j '<python expr over d>' — reads JSON on stdin, prints eval(expr).
j() { python3 -c 'import sys,json; d=json.load(sys.stdin); print(eval(sys.argv[1]))' "$1"; }
fail() { echo "FAIL: $1"; exit 1; }
cleanup() { [ -n "$FID" ] && curl -s -X DELETE "$API_URL/v1/orgs/$ORG_ID/flows/$FID" -H "$H" -o /dev/null || true; }
trap cleanup EXIT

echo "--- 1. create draft ($CODE) ---"
GRAPH='{"nodes":[
  {"id":"t","type":"trigger","config":{}},
  {"id":"c","type":"if_else","config":{"expr":"customer.tier == \"gold\""}},
  {"id":"msg","type":"send_message","config":{"text":"Hi ${customer.name}"}},
  {"id":"p","type":"prompt_text","config":{"save_as":"answer","timeout_sec":600}},
  {"id":"eg","type":"end","config":{"outcome":"gold"}},
  {"id":"ec","type":"end","config":{"outcome":"captured"}},
  {"id":"et","type":"end","config":{"outcome":"timeout"}}
],"edges":[
  {"from":"t","to":"c"},
  {"from":"c","to":"msg","label":"true"},
  {"from":"c","to":"p","label":"false"},
  {"from":"msg","to":"eg"},
  {"from":"p","to":"ec","label":"captured"},
  {"from":"p","to":"et","label":"timeout"}
]}'
CR=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/flows" -H "$H" -H 'Content-Type: application/json' \
  -d "{\"code\":\"$CODE\",\"name\":\"e2e rt\",\"graph\":$GRAPH}")
FID=$(echo "$CR" | j 'd.get("id","null")')
[ "$FID" != "null" ] && [ -n "$FID" ] || fail "create draft: $CR"
echo "  flow=$FID"

echo "--- 2. validate (expect valid) ---"
VAL=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/flows/$FID/validate" -H "$H" -H 'Content-Type: application/json' -d '{}')
[ "$(echo "$VAL" | j 'd.get("valid")')" == "True" ] || fail "validate: $VAL"

echo "--- 3. publish ---"
PUB=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/flows/$FID/publish" -H "$H" -H 'Content-Type: application/json' \
  -d "{\"channel\":\"voice\",\"entry_code\":\"$ENTRY\",\"version\":1}")
[ "$(echo "$PUB" | j 'd.get("binding",{}).get("active")')" == "True" ] || fail "publish: $PUB"

echo "--- 4. simulate gold branch (interpolated send_message) ---"
SIM=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/flows/$FID/simulate" -H "$H" -H 'Content-Type: application/json' \
  -d '{"interaction_input":{"customer":{"tier":"gold","name":"Lan"}}}')
[ "$(echo "$SIM" | j 'd["trace"]["outcome"]')" == "completed" ] || fail "simulate outcome: $SIM"
MSG=$(echo "$SIM" | j '[s for s in d["trace"]["steps"] if s["node_kind"]=="send_message"][0]["output"]["text"]')
[ "$MSG" == "Hi Lan" ] || fail "send_message interpolation: got '$MSG' want 'Hi Lan'"

echo "--- 5. simulate captured-input branch (scripted_effect_outputs) ---"
SIM2=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/flows/$FID/simulate" -H "$H" -H 'Content-Type: application/json' \
  -d '{"interaction_input":{"customer":{"tier":"silver"}},"scripted_effect_outputs":{"p":"yes please"}}')
PPORT=$(echo "$SIM2" | j '[s for s in d["trace"]["steps"] if s["node_id"]=="p"][0]["port"]')
[ "$PPORT" == "captured" ] || fail "input scripted branch: got '$PPORT' want 'captured'"

echo "--- 6. live route → waiting at input ---"
RR=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/route-requests" -H "$H" -H 'Content-Type: application/json' \
  -d "{\"channel\":\"voice\",\"entry_code\":\"$ENTRY\",\"interaction_input\":{\"customer\":{\"tier\":\"silver\"}}}")
RID=$(echo "$RR" | j 'd["id"]')
[ "$(echo "$RR" | j 'd["status"]')" == "waiting" ] || fail "live route status (want waiting): $RR"

echo "--- 7. submit input → completed ---"
SUB=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/route-requests/$RID/input" -H "$H" -H 'Content-Type: application/json' -d '{"value":"hi"}')
[ "$(echo "$SUB" | j 'd["status"]')" == "completed" ] || fail "submit input: $SUB"

echo "--- 8. trace shows captured branch ---"
TR=$(curl -s "$API_URL/v1/orgs/$ORG_ID/route-requests/$RID/trace" -H "$H")
[ "$(echo "$TR" | j 'd["outcome"]')" == "completed" ] || fail "trace outcome: $TR"
TCAP=$(echo "$TR" | j '[s for s in d["steps"] if s["node_id"]=="p"][0]["output"]["captured"]')
[ "$TCAP" == "hi" ] || fail "trace captured value: got '$TCAP' want 'hi'"

echo ""
echo "PASS: v0.2 runtime e2e (validate, publish, simulate x2, live execute, input submit, trace)"
