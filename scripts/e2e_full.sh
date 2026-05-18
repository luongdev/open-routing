#!/usr/bin/env bash
set -euo pipefail

API_URL="http://127.0.0.1:8080"
ORG_ID="01934567-0000-7000-8000-000000000001"
HEADER="X-Org-Id: $ORG_ID"

# 1. Create a Skill
echo "--- Step 1: Create Skill ---"
SKILL_RESP=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/skills" \
  -H "$HEADER" \
  -H "Content-Type: application/json" \
  -d '{"code": "skill_e2e_1", "external_id": "SKILL-E2E-1", "name": "E2E Skill", "skill_type": "technical"}')
SKILL_ID=$(echo "$SKILL_RESP" | jq -r '.id')
if [ "$SKILL_ID" == "null" ]; then echo "FAIL: Create Skill - $SKILL_RESP"; exit 1; fi
echo "Skill Created: $SKILL_ID"

# 2. Create a Break Reason
echo "--- Step 2: Create Break Reason ---"
BR_RESP=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/break-reasons" \
  -H "$HEADER" \
  -H "Content-Type: application/json" \
  -d '{"code": "break_lunch", "name": "E2E Lunch", "routable": false, "display_order": 1}')
BR_ID=$(echo "$BR_RESP" | jq -r '.id')
if [ "$BR_ID" == "null" ]; then echo "FAIL: Create Break Reason - $BR_RESP"; exit 1; fi
echo "Break Reason Created: $BR_ID"

# 3. Create an Agent with Skill and Code
echo "--- Step 3: Create Agent ---"
AGENT_RESP=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/agents" \
  -H "$HEADER" \
  -H "Content-Type: application/json" \
  -d "{
    \"code\": \"agent_e2e_1\",
    \"external_id\": \"AGENT-E2E-1\",
    \"name\": \"E2E Agent\",
    \"email\": \"e2e@example.com\",
    \"skills\": [{\"skill_id\": \"$SKILL_ID\", \"proficiency\": 5}]
  }")
AGENT_ID=$(echo "$AGENT_RESP" | jq -r '.id')
if [ "$AGENT_ID" == "null" ]; then echo "FAIL: Create Agent - $AGENT_RESP"; exit 1; fi
echo "Agent Created: $AGENT_ID"

# 4. Check Initial Status (should be Offline)
echo "--- Step 4: Check Initial Status ---"
STATUS_RESP=$(curl -s -H "$HEADER" "$API_URL/v1/orgs/$ORG_ID/agents/$AGENT_ID/status")
INITIAL_STATUS=$(echo "$STATUS_RESP" | jq -r '.status')
echo "Initial Status: $INITIAL_STATUS"

# 5. Transition to Ready (using force=true to bypass Offline->Ready restriction)
echo "--- Step 5: Transition to Ready (Force) ---"
READY_RESP=$(curl -s -X PATCH "$API_URL/v1/orgs/$ORG_ID/agents/$AGENT_ID/status" \
  -H "$HEADER" \
  -H "Content-Type: application/json" \
  -d '{"to": "Ready", "force": true}')
NEW_STATUS=$(echo "$READY_RESP" | jq -r '.status')
if [ "$NEW_STATUS" != "Ready" ]; then echo "FAIL: Transition to Ready - $READY_RESP"; exit 1; fi
echo "New Status: $NEW_STATUS"

# 6. Transition to Break
echo "--- Step 6: Transition to Break ---"
BREAK_RESP=$(curl -s -X PATCH "$API_URL/v1/orgs/$ORG_ID/agents/$AGENT_ID/status" \
  -H "$HEADER" \
  -H "Content-Type: application/json" \
  -d "{\"to\": \"Break\", \"break_reason_id\": \"$BR_ID\"}")
BREAK_STATUS=$(echo "$BREAK_RESP" | jq -r '.status')
if [ "$BREAK_STATUS" != "Break" ]; then echo "FAIL: Transition to Break - $BREAK_RESP"; exit 1; fi
echo "New Status: $BREAK_STATUS, Reason: $(echo "$BREAK_RESP" | jq -r '.break_reason_id')"

# 7. Phase 5: Bulk Import Agents (JSON)
echo "--- Step 7: Bulk Import Agents (JSON) ---"
IMPORT_RESP=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/catalog/import?entity=agents" \
  -H "$HEADER" \
  -H "Content-Type: application/json" \
  -d '[
    {"code": "agent_bulk_1", "name": "Bulk Agent 1", "email": "bulk1@example.com"},
    {"code": "agent_bulk_2", "name": "Bulk Agent 2", "email": "bulk2@example.com"}
  ]')
SUCCEEDED_COUNT=$(echo "$IMPORT_RESP" | jq '.succeeded | length')
if [ "$SUCCEEDED_COUNT" != "2" ]; then echo "FAIL: Bulk Import JSON - $IMPORT_RESP"; exit 1; fi
echo "Bulk Import JSON Succeeded: $SUCCEEDED_COUNT agents"

# 8. Phase 5: Bulk Import Agents (CSV)
echo "--- Step 8: Bulk Import Agents (CSV) ---"
CSV_BODY="code,name,email
agent_csv_1,CSV Agent 1,csv1@example.com
agent_csv_2,CSV Agent 2,csv2@example.com"
IMPORT_CSV_RESP=$(curl -s -X POST "$API_URL/v1/orgs/$ORG_ID/catalog/import?entity=agents&schema_version=v0.1" \
  -H "$HEADER" \
  -H "Content-Type: text/csv" \
  --data-binary "$CSV_BODY")
SUCCEEDED_CSV_COUNT=$(echo "$IMPORT_CSV_RESP" | jq '.succeeded | length')
if [ "$SUCCEEDED_CSV_COUNT" != "2" ]; then echo "FAIL: Bulk Import CSV - $IMPORT_CSV_RESP"; exit 1; fi
echo "Bulk Import CSV Succeeded: $SUCCEEDED_CSV_COUNT agents"

# 9. Verify Imported Agents
echo "--- Step 9: Verify All Imported Agents ---"
LIST_RESP=$(curl -s -H "$HEADER" "$API_URL/v1/orgs/$ORG_ID/agents")
TOTAL_COUNT=$(echo "$LIST_RESP" | jq '.items | length')
# 1 (e2e_1) + 2 (bulk) + 2 (csv) = 5
if [ "$TOTAL_COUNT" -lt 5 ]; then echo "FAIL: Not all agents found in list - $LIST_RESP"; exit 1; fi
echo "All 5 agents verified in list."

# 10. Soft Delete Agent
echo "--- Step 10: Soft Delete Agent ---"
curl -s -X DELETE "$API_URL/v1/orgs/$ORG_ID/agents/$AGENT_ID" -H "$HEADER"
LIST_AFTER_DEL=$(curl -s -H "$HEADER" "$API_URL/v1/orgs/$ORG_ID/agents")
FOUND=$(echo "$LIST_AFTER_DEL" | jq -r ".items[] | select(.id == \"$AGENT_ID\") | .id")
if [ -z "$FOUND" ]; then
    echo "Agent successfully soft-deleted (hidden from default list)."
else
    echo "FAIL: Agent still in default list after delete."
    exit 1
fi

# 11. Verify Isolation
echo "--- Step 11: Verify Isolation ---"
OTHER_ORG="01934567-0000-7000-8000-000000000002"
ISO_RESP=$(curl -s -H "X-Org-Id: $OTHER_ORG" "$API_URL/v1/orgs/$OTHER_ORG/agents")
ISO_FOUND=$(echo "$ISO_RESP" | jq -r ".items[] | select(.id == \"$AGENT_ID\") | .id")
if [ -z "$ISO_FOUND" ]; then
    echo "Isolation OK: Agent not visible to another org."
else
    echo "FAIL: Isolation breach! Agent visible to another org."
    exit 1
fi

echo "E2E Test Success!"
