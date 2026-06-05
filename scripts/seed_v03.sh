#!/usr/bin/env bash
# Seed a demo fixture set for the v0.3 live engine: skills, queues, Ready agents,
# a published voice routing flow, and a few in-flight route requests so the Live
# Ops view + Route Tester have something to show. Idempotent-ish on a fresh DB
# (fixed codes). python3 for JSON, no jq.
set -euo pipefail
URL="${API_URL:-http://10.69.69.169:30080}"
ORG="${ORG_ID:-01920000-0000-7000-8000-000000000001}"
H="X-Org-Id: $ORG"
j() { python3 -c 'import sys,json; d=json.load(sys.stdin); print(eval(sys.argv[1]))' "$1"; }
post() { curl -s -X POST "$URL$1" -H "$H" -H 'Content-Type: application/json' -d "$2"; }

echo "--- skills ---"
SALES=$(post "/v1/orgs/$ORG/skills" '{"code":"sales","name":"Sales","skill_type":"binary"}' | j 'd.get("id","")')
SUP=$(post "/v1/orgs/$ORG/skills" '{"code":"support","name":"Support","skill_type":"binary"}' | j 'd.get("id","")')
echo "sales=$SALES support=$SUP"

echo "--- queues ---"
post "/v1/orgs/$ORG/queues" '{"code":"vip","name":"VIP","channel_types":["voice"]}' >/dev/null
post "/v1/orgs/$ORG/queues" '{"code":"general","name":"General","channel_types":["voice"]}' >/dev/null

echo "--- agents (Ready) ---"
mkagent() { # code name skill_id...
  local code="$1" name="$2"; shift 2
  local skills=""
  for sid in "$@"; do [ -n "$sid" ] && skills="$skills{\"skill_id\":\"$sid\",\"proficiency\":3},"; done
  skills="[${skills%,}]"
  local aid
  aid=$(post "/v1/orgs/$ORG/agents" "{\"code\":\"$code\",\"name\":\"$name\",\"email\":\"$code@demo.test\",\"skills\":$skills}" | j 'd.get("id","")')
  [ -n "$aid" ] && curl -s -X PATCH "$URL/v1/orgs/$ORG/agents/$aid/status" -H "$H" -H 'Content-Type: application/json' -d '{"to":"Ready","force":true}' >/dev/null
  echo "  $name ($code) = $aid"
}
mkagent alice "Alice Nguyen" "$SALES"
mkagent bob "Bob Tran" "$SUP"
mkagent carol "Carol Le" "$SALES" "$SUP"

echo "--- voice flow (route_queue → match_skill → reservation), published voice/main ---"
GRAPH="{\"nodes\":[
  {\"id\":\"t\",\"type\":\"trigger\",\"config\":{}},
  {\"id\":\"rq\",\"type\":\"route_queue\",\"config\":{\"queue\":\"vip\"}},
  {\"id\":\"ms\",\"type\":\"match_skill\",\"config\":{\"skill\":\"sales\",\"min_proficiency\":1}},
  {\"id\":\"rsv\",\"type\":\"reservation\",\"config\":{\"timeout_sec\":30,\"max_attempts\":3}},
  {\"id\":\"fb\",\"type\":\"fallback\",\"config\":{\"reason\":\"no agent\"}},
  {\"id\":\"end\",\"type\":\"end\",\"config\":{}}
],\"edges\":[
  {\"from\":\"t\",\"to\":\"rq\"},{\"from\":\"rq\",\"to\":\"ms\"},{\"from\":\"ms\",\"to\":\"rsv\"},
  {\"from\":\"rsv\",\"to\":\"end\",\"label\":\"accepted\"},
  {\"from\":\"rsv\",\"to\":\"fb\",\"label\":\"timeout\"},
  {\"from\":\"rsv\",\"to\":\"fb\",\"label\":\"no_candidate\"},
  {\"from\":\"fb\",\"to\":\"end\"}
]}"
FID=$(post "/v1/orgs/$ORG/flows" "{\"code\":\"inbound\",\"name\":\"Inbound Voice\",\"graph\":$GRAPH}" | j 'd.get("id","")')
[ -n "$FID" ] || { echo "flow create failed"; exit 1; }
post "/v1/orgs/$ORG/flows/$FID/publish" '{"channel":"voice","entry_code":"main","version":1}' >/dev/null
echo "flow=$FID published voice/main"

echo "--- a few live route requests (park waiting_match — no WS agent connected) ---"
for i in 1 2 3; do
  post "/v1/orgs/$ORG/route-requests" '{"channel":"voice","entry_code":"main"}' | j 'd.get("status","?")'
done

echo "--- routing stats now ---"
curl -s "$URL/v1/orgs/$ORG/routing/stats" -H "$H"; echo
echo "PASS: seeded. Open Live Ops to watch the queue."
