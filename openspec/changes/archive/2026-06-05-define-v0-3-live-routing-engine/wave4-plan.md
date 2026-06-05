# Wave 4 — The matcher (the engine) — implementation plan, rev 2

> rev 2 folds the codex + gemini plan review (3 BLOCK, 7 HIGH, 6 MED). The plan
> had real stuck-route + stale-worker-overwrite holes; the fix is a **match-offer
> token** fencing model + an explicit state machine + three sweeps + SQL-computed
> ranking. Deltas from rev1 marked **[Rxx]**.

Goal: turn the engine from **offer-now-or-fail** into a **queue + bidirectional
matcher**, on the W3 substrate (capacity slots, presence lease, route run-lock,
durable continuation worker). A route with no available agent **waits in a queue**
and is pulled onto an agent the moment one becomes Ready / frees capacity.

## State machine (live routes) — explicit, token-fenced

Statuses: `pending`→`running` (executor active, run-lock) →`waiting` (suspended at
a reservation, offer outstanding, or a wait node) → terminal. **NEW**:
`waiting_match` (queued, no outstanding offer, matcher-owned) and `offering`
(matcher claimed it, building an offer — transient).

Fencing columns on `route_requests` **[R-token]** (codex BLOCK ×3): `match_offer_token
uuid NULL`, `match_attempt_seq int NOT NULL DEFAULT 0`, `offering_started_at
timestamptz NULL`. EVERY claim/offer-commit/requeue/recover is guarded
`WHERE ... AND match_offer_token = $token` so a stalled worker that wakes after a
sweeper requeued + another worker re-offered finds its token superseded → 0 rows →
it rolls back and does nothing. Transitions (all token-fenced except enqueue):

- **enqueue** (interaction side, Stage 2): `running → waiting_match`, set queue_id,
  priority, waiting_since=now(), next_match_at=now() **[R-nextmatch NOT NULL]**,
  required_skills, match_deadline, keep excluded_agent_ids.
- **claim** (matcher): one statement
  `UPDATE route_requests SET status='offering', match_offer_token=gen_random_uuid(),
   match_attempt_seq=match_attempt_seq+1, offering_started_at=now()
   FROM (SELECT id FROM route_requests WHERE ... ORDER BY <score> FOR UPDATE OF
   route_requests SKIP LOCKED LIMIT 1) c WHERE route_requests.id=c.id RETURNING *`
   (single claiming statement — codex LOW; lock ONLY rr — agy).
- **offer-commit** (separate SHORT tx): acquire slot (SKIP LOCKED) + insert
  reservation(lease_token, agent_session_id) + `UPDATE route SET status='waiting',
  active_reservation_id=$res WHERE id=$ AND match_offer_token=$token AND
  status='offering'` (0 rows → token superseded → roll back, release slot) + arm
  reservation_timeout continuation clamped to `match_deadline` **[R-clamp]** + a
  route_decisions row + agent_outbox offer frame.
- **offer-failure** (slot/lease/route lost): guarded recovery in its OWN tx (NOT
  the rolled-back one — codex MED) `UPDATE route SET status='waiting_match',
  next_match_at=now() WHERE id=$ AND match_offer_token=$token AND status='offering'`
  + a route_decisions row recording the outcome.
- **stale-offering sweep**: `status='offering' AND offering_started_at < now()-Δ`
  → requeue with a NEW token (so the original worker can't later commit) →
  waiting_match, next_match_at=now() **[R-stale]** (codex BLOCK).
- **deadline sweep (SLA)**: `status='waiting_match' AND match_deadline <= now()`
  → CAS-acquire the run-lock + resume the route with the `no_candidate` port to
  fallback **[R-deadline]** (codex BLOCK: waiting_match must not wait forever). It
  targets ONLY waiting_match; an `offering`/`waiting`(offered) route's
  reservation_timeout wins (precedence — codex/agy HIGH).
- **accept/reject/complete**: existing run-lock CAS + lease fencing below.

`AcquireRouteForRun` legal source statuses become `pending|waiting|waiting_match`
→`running` is wrong for the matcher — the matcher does NOT call resumeRoute inside
its claim/offer tx (that would self-block on the run-lock — agy BLOCK). Instead the
route is already suspended AT the reservation cursor (set when it parked
waiting_match); offer-commit just attaches the offer + flips to `waiting`. The
existing accept/timeout resume (run-lock) drives the flow from the cursor. So the
matcher never holds the route lock across a resume **[R-noresume]**.

## Stage 1 — Data model (fold into 000001)

`route_requests` +: `queue_id uuid`, `priority int NOT NULL DEFAULT 0`,
`waiting_since timestamptz`, `next_match_at timestamptz`, `required_skills text[]
NOT NULL DEFAULT '{}'`, `match_deadline timestamptz`, `match_attempt_seq int NOT
NULL DEFAULT 0`, `match_offer_token uuid`, `offering_started_at timestamptz`,
`active_reservation_id uuid`, `excluded_agent_ids uuid[] NOT NULL DEFAULT '{}'`
**[R-excluded]** (codex/agy HIGH: distinct-agent RONA). Status gains
`waiting_match`,`offering`.

Indexes **[R-index]** (codex MED): partial pull index
`(org_id, queue_id, next_match_at, priority DESC, waiting_since ASC) WHERE
status='waiting_match'`; `(org_id, match_deadline) WHERE status='waiting_match'`
(SLA sweep); `(org_id, offering_started_at) WHERE status='offering'` (stale sweep);
GIN on `required_skills`.

`reservations` +: `lease_token uuid`, `agent_session_id uuid` (D5 fencing).

`route_decisions` (audit D9): id, org_id, route_request_id, decision_type
(interaction_offer|availability_pull|retry|sweep), decision_version,
matcher_instance, channel, queue_id, selected_agent_id, selected_slot_no, outcome
(offered|no_candidate|capacity_lost|lease_lost|route_lost), reason, created_at +
JSONB eligibility/candidates/excluded/ranking. Index (org_id, route_request_id,
created_at DESC). Add to SQLChecker tenantTables.

`agents` +: `routing_state text NOT NULL DEFAULT 'routable'`,
`routing_state_expires_at timestamptz`, `last_ready_at timestamptz` (RONA fence).

## Stage 2 — Enqueue (interaction-driven side)

`liveReservation` tries the live pool (existing offer-now). On empty/at-capacity,
park `waiting_match` (NOT fallback) capturing queue_id, priority, required_skills
(from the compiled match_skill node feeding the reservation), waiting_since,
next_match_at=now(), match_deadline. Gate to LIVE mode only — sim/replay keeps the
deterministic fallback **[R-determinism]** (codex MED). Stable tie-break `id ASC`.

## Stage 3 — The matcher (availability-driven pull), cmd/runtime ~2s tick

Primary trigger = the durable sweep (NOTIFY is only a wake hint — D8). Per org,
bounded batch (≤100 agents/≤100 routes). For each **available agent** (Ready +
leased-connected + under-capacity + routable: `routing_state='routable' OR
routing_state_expires_at <= now()` **[R-ronaexpiry]** codex MED):

1. **claim** the best eligible waiting route (single fenced UPDATE above). Eligible:
   `status='waiting_match' AND next_match_at<=now() AND required_skills <@
   $agentSkills AND $agentId <> ALL(excluded_agent_ids) AND queue served`.
   **Ranking computed in SQL** **[R-score]** (codex/agy BLOCK — bounded prefix
   breaks cross-band aging): `ORDER BY (priority*W_p + queue_weight*10 +
   EXTRACT(EPOCH FROM now()-waiting_since)*W_age) DESC, id ASC LIMIT 1` over the
   agent's eligible subset (narrowed by skills/queue, bounded). W_age chosen so a
   sufficiently-aged low band overtakes (no starvation).
2. **offer-commit** (short tx): re-confirm lease (Connected + token) → acquire slot
   → insert reservation(lease_token, agent_session_id) → guarded route flip →
   continuation (clamped) → route_decisions(offered) → agent_outbox. Lock order
   GLOBAL **[R-lockorder]**: route-claim-token first, then slot (SKIP LOCKED), then
   reservation — same order as the inline + accept paths (agy BLOCK / codex MED).
3. On any pre-offer failure: guarded requeue (own tx) + route_decisions(outcome).

`max_attempts` = count of ACTUAL offered reservations to DISTINCT agents (derive
from reservations / excluded_agent_ids), NOT match_attempt_seq **[R-attempts]**
(codex HIGH). Exhaustion → fallback.

## Stage 4 — Lease fencing + RONA

- **Lease fencing** (codex HIGH): accept/reject/complete (command.go + HTTP) match
  reservation id + `lease_token` + `agent_session_id` + reservation state + `route.
  active_reservation_id = reservation.id`, all in one tx — a stale command for a
  superseded reservation fails closed **[R-fence]**.
- **RONA** (codex HIGH): offer timeout appends the agent to the route's
  `excluded_agent_ids` (the distinct-agent guarantee) AND sets the agent
  `routing_state='missed'`/expiry — but ONLY if `last_ready_at` has NOT advanced
  past the offer time (so a Ready that arrived after the ring isn't clobbered)
  **[R-readyfence]**. Expired `missed` is treated routable (no stuck filter).

## Stage 5 — Wire + tests

cmd/runtime tick runs: claim+offer per available agent, the deadline sweep, the
stale-offering sweep (alongside the existing continuation worker + capacity sweep).
Postgres `now()` only (never gateway clocks). Tests (testcontainer, `-race`):
enqueue parks waiting_match; availability pull offers + holds slot + suspends;
fairness incl. **a winner OUTSIDE the priority prefix** (aged low-priority
overtakes — codex HIGH test); 2 matchers + 1 route + 1 agent → exactly one offer;
N agents × M routes no double-assign; stale-offering recovery requeues a crashed
claim WITHOUT clobbering a fresh offer (token fence); SLA deadline → fallback;
RONA distinct-agent (same route never re-offered to a missed agent) + Ready-race
fence; lease-token mismatch ⇒ conflict; route_decisions row per decision;
sim/replay determinism unchanged.

## Sequencing
1. Migration + sqlc + SQLChecker. 2. Enqueue + state machine + tests. 3. Matcher
claim/offer/ranking + route_decisions + tests. 4. Lease fencing + RONA + tests.
5. cmd/runtime wiring + the two new sweeps. Cross-AI review of the implementation
before declaring W4 done. Each stage build+test+lint green and committed.
