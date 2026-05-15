# Feature Research: Open Routing v0.1 Catalog Foundation

**Domain:** Multi-channel ACD-style routing platform — catalog + agent state machine layer
**Researched:** 2026-05-15
**Milestone scope:** v0.1 only — catalog CRUD, agent state model, bulk import, MFE config UI
**Confidence:** HIGH for agent state model (vendor evidence); MEDIUM for MFE and bulk import patterns

---

## 1. Agent State Machine: Vendor Evidence

This is the most critical section. Findings are based on documentation from 8 vendors. States are grouped by who controls them.

### 1.1 Vendor Survey

#### Genesys Cloud CX

Genesys Cloud separates **presence** (what the user signals) from **routing status** (what the system sets when on-queue).

**Agent-toggleable (Presence layer):**
- `Available` — ready for ACD interactions; maps to "On Queue + Idle" routing status
- `Away` — logged in, not ready, no sub-reasons by default
- `Break` — logged in, on break
- `Meal` — logged in, at meal
- `Meeting` — logged in, in a meeting
- `Training` — logged in, in training
- `Out of Office` — logged in, OOO (sends calls to voicemail automatically)
- `Busy` — logged in, busy (sends calls to voicemail automatically)

Admins configure **secondary statuses** on top of primary statuses (up to 30 per primary). Secondary statuses are cosmetic/reporting labels; they do not independently control routing. Routing is governed by the presence + routing status combination.

**System-set (Routing Status layer — set while agent is On Queue):**
- `Idle` — on queue, no active interactions, can receive new ones
- `Interacting` — on queue, handling ACD interaction(s) and/or ACW; may receive additional interactions per utilization config
- `Communicating` — on queue, on a non-ACD call (counts against utilization)
- `Not Responding` — system-set when agent fails to accept an interaction within the queue's alerting timeout; no new interactions routed until cleared

**Channel states (conversation participant states, not top-level agent states):**
Genesys Cloud models channel engagement at the conversation/participant level, not the agent state level. A participant can be:
- `alerting` (ringing/incoming)
- `connected` (active engagement)
- `held` (on hold — voice only)
- `disconnected` (ended)

For voice: hold, conference, consult-transfer are conversation-level actions, not agent state changes. The agent's routing status stays `Interacting` throughout.

**Break/Not Ready terminology:** Genesys Cloud calls these "secondary statuses" layered on primary presences. The platform uses `Away`, `Break`, `Meal`, `Meeting`, `Training` as primary presences. No first-class "routable" flag on secondary statuses — routing is controlled by the primary presence + On Queue toggle.

**Sources:** [Genesys routing status glossary](https://help.genesys.cloud/glossary/routing-status/), [Agent presence and activity indicators](https://help.genesys.cloud/articles/agent-presence-status-and-activity-indicators/), [Secondary status overview](https://help.genesys.cloud/articles/about-statuses/)

---

#### NICE CXone (inContact)

NICE CXone uses a two-value model at the API level, with reason codes layered on top.

**Agent-toggleable:**
- `Available` — the single routable state; agent can receive contacts
- `Unavailable` — the single non-routable state; accepts reason codes to categorize

**System-set:**
- The platform automatically sets state based on session events; the `AgentState` event includes `CurrentState`, `CurrentOutReason`, `IsAcw`, and `AcwTimeout` fields

**Unavailable reason codes (called "Unavailable Codes" in admin):**
- Configurable per org, assigned to teams
- One special property: `IsACW` flag — when true, the code is an After Contact Work code (supervisor-visible only, not agent-selectable; applied automatically post-contact)
- `Agent Timeout` — configurable duration (30–720 min) before automatic state change

The API's `AgentState` session event reports `CurrentState: "Available"` or `CurrentState: "Unavailable"` with `CurrentOutReason` carrying the reason code name.

**Engaged/contact states (separate from agent state):**
Contact types handled include: phone calls, chats, emails, voice mails, work items. Voice call handling includes hold, consult, conference, transfer — all as contact-level operations exposed through the Agent API session.

**Break/Not Ready terminology:** NICE CXone calls them "Unavailable Codes." No explicit "routable" flag on individual codes — all Unavailable Codes are non-routable by definition. The `IsACW` flag distinguishes ACW from other unavailable reasons.

**Sources:** [Set Agent State action](https://help.nice-incontact.com/content/studio/actions/setagentstate/setagentstate.htm), [Set Up Unavailable Codes](https://help.incontact.com/Content/ACD/UnavailableCodes/SetUpUnavailableCodes.htm), [Agent Session Events](https://developer.niceincontact.com/Documentation/AgentSessionEvents)

---

#### Amazon Connect

Amazon Connect uses a clean three-type model for agent statuses with explicit routing semantics.

**Agent-toggleable (set in CCP — Contact Control Panel):**
- `Available` — type `ROUTABLE`; agent can receive inbound contacts
- Custom statuses (e.g., `Break`, `Lunch`, `Training`) — type `CUSTOM`; non-routable; agents must select from org-configured list
- `Offline` — type `OFFLINE`; agent is not signed in

**API model for agent status:**
```
AgentStatus {
  Name: string        // "Available", "Break", "Lunch", etc.
  Type: ROUTABLE | CUSTOM | OFFLINE
  State: ENABLED | DISABLED
  DisplayOrder: number
}
```
`ROUTABLE` is the flag that controls whether contacts are routed. Only one built-in status is `ROUTABLE` ("Available"). All custom statuses created via admin are always `CUSTOM` (non-routable). This is the cleanest vendor model of a "routable" flag — it is explicit in the data model.

**System-set (Agent Activity State — shown in real-time metrics):**
- `Available` — ready for contacts
- `Incoming` — contact is ringing (alerting)
- `On contact` — actively handling at least one contact (Connected, On Hold, Paused, Outbound)
- `After contact work` — post-contact wrap-up
- `Error` — missed call or rejected chat/task
- `Missed` — did not answer in time
- `NPT (Non-Productive Time)` — in a custom status; agent is simultaneously NPT and may be On contact for outbound calls

**Contact states (channel-level, per contact in event stream):**
- `INCOMING` — queued callback being offered
- `PENDING` — queued callback pending
- `CONNECTING` — contact being offered (ringing), agent hasn't acted
- `CONNECTED` — agent accepted; customer in conversation
- `CONNECTED_ONHOLD` — agent put customer on hold (voice only)
- `PAUSED` — contact paused (tasks only)
- `ENDED` — conversation ended, agent in ACW
- `MISSED` — agent did not answer
- `REJECTED` — agent or customer abandoned during connect
- `ERROR` — error during outbound whisper

**Channel-specific differences:**
- **Voice:** CONNECTING → CONNECTED → CONNECTED_ONHOLD (hold) → ENDED; hold tracking
- **Chat:** no hold; CONNECTING → CONNECTED → ENDED; message metrics
- **Task:** CONNECTING → CONNECTED → PAUSED (unique to tasks) → ENDED
- **Email:** async, no "ringing"; CONNECTED → ENDED

**Break/Not Ready terminology:** Amazon Connect calls them "custom agent statuses" or simply "custom statuses." The `Type: ROUTABLE | CUSTOM | OFFLINE` enum is the explicit routable flag in the data model.

**Sources:** [Agent status in CCP](https://docs.aws.amazon.com/connect/latest/adminguide/metrics-agent-status.html), [Contact states](https://docs.aws.amazon.com/connect/latest/adminguide/about-contact-states.html), [CreateAgentStatus API](https://docs.aws.amazon.com/connect/latest/APIReference/API_CreateAgentStatus.html), [Metric definitions](https://docs.aws.amazon.com/connect/latest/adminguide/metrics-definitions.html)

---

#### Twilio Flex (TaskRouter)

Twilio Flex uses TaskRouter's Activity model, where every agent state is an "Activity" with an `available: boolean` property.

**Agent-toggleable Activities:**
- `Available` — `available: true`; agent receives tasks
- `Break` — `available: false`; default shipped activity
- `Offline` — `available: false`; default shipped activity
- `Unavailable` — `available: false`; default shipped activity
- Any custom Activity with `available: false` (up to 100 Activities per Workspace)

**System-set (injected by activity-reservation-handler feature):**
- `On a Task` — `available: true`; agent is available AND handling a task
- `On a Task, No ACD` — `available: false`; agent is handling a task but should not receive additional ACD tasks
- `Wrap Up` — `available: true`; agent in ACW, still eligible for routing
- `Wrap Up, No ACD` — `available: false`; agent in ACW, not eligible for routing
- `Extended Wrap Up` — triggered when agent requests extended wrap-up

The `available` property on each Activity is the canonical "routable" flag. It cannot be changed after the Activity is created.

**Task lifecycle states (system-set):**
- `pending` — task waiting for matching worker
- `reserved` — task offered to a worker (ringing/alerting)
- `assigned` — worker accepted; active work
- `wrapping` — post-work phase (multitasking workspaces)
- `completed` — terminal state
- `canceled` — task exceeded timeout or was cancelled

**Reservation states:** `pending` → `accepted` | `rejected` | `timeout` | `canceled` | `rescinded` | `wrapping` | `completed`

**Channel-specific engaged substates:** Twilio Flex does not model channel-specific substates at the agent level. Voice uses `reservation.conference` to connect the call; chat uses the `conversations` SDK. Both channel types go through the same `reserved` → `assigned` task state progression. Hold, conference, and transfer for voice are executed at the Programmable Voice/conference level, not as distinct agent state changes.

**Break/Not Ready terminology:** Twilio calls them "Activities." The `available: true/false` property on an Activity is the exact analog of a "routable" flag.

**Sources:** [TaskRouter Activity Resource](https://www.twilio.com/docs/taskrouter/api/activity), [Task state lifecycle](https://www.twilio.com/docs/taskrouter/lifecycle-task-state), [Activity Reservation Handler](https://twilio-professional-services.github.io/flex-project-template/feature-library/activity-reservation-handler)

---

#### Five9

Five9 uses a granular state model that explicitly separates call state from agent state.

**Agent-toggleable:**
- `Ready` — available to receive ACD calls
- `Not Ready` — unavailable; carries a configurable reason code

Five9's CRM SDK reports a `Presence` interface with:
- `isReady: boolean` — ready status
- `notReadyCodeId` — the current Not Ready reason code ID

**System-set (from CRM SDK):**
- `TALKING` — on an active call
- `ON_HOLD` — call is on hold
- `WRAP_UP` — after call work
- `FINISHED` — call ended (transitional)

Five9 calls its reason codes "Not Ready Reason Codes" or simply "Reason Codes." Examples in use: `Break`, `Lunch`, `Consulting Supervisor`, `Outbound Calling`, `End of Shift`, `Other`.

**Channel-specific engaged substates (from CRM SDK CallData):**
- `callState: "TALKING"` — active voice
- `callState: "ON_HOLD"` — voice on hold
- `callState: "WRAP_UP"` — in ACW for voice
- `callState: "FINISHED"` — voice call ended

**Break/Not Ready terminology:** Five9 calls them "Reason Codes" or "Not Ready Codes." No explicit "routable" flag on individual codes — all Not Ready codes are non-routable.

**Sources:** [Five9 CRM SDK global constants](https://cdn.prod.us.five9.net/stable/crm-sdk-lib/doc/global.html), [Five9 FAQ on Not Ready](https://www.five9.com/faq/what-is-not-ready)

---

#### Talkdesk

Talkdesk uses a simple named-state model with customization at the admin level.

**Agent-toggleable:**
- `Available` — logged in, can receive calls
- `Away` — logged in, away from computer
- `Busy` — logged in, cannot take calls (admin-assigned)
- Custom statuses configured by admin (e.g., `Lunch`, `Training`)

**System-set:**
- `On a Call` — actively speaking with customer; automatically set when call connects
- `After Call Work` — automatically set after call ends (5-minute default); agent cannot receive calls during ACW

**Offline:**
- `Offline` — not signed in

**Break/Not Ready terminology:** Talkdesk uses "Agent Status" as the canonical term. Custom statuses serve as break/not-ready analogs. No explicit "routable" flag on custom statuses — all custom statuses are non-routable by definition.

**Channel-specific substates:** Talkdesk was primarily voice-focused in its original model. Digital channels use the "Conversations" product with separate channel handling. Voice call actions (hold, transfer, conference) are call-level controls, not agent state changes.

**Sources:** [Talkdesk Agent Status and availability](https://support.talkdesk.com/hc/en-us/articles/200496719), [Custom status blog](https://www.talkdesk.com/blog/new-feature-customizable-call-center-agent-status-settings/)

---

#### 8x8 Contact Center

8x8 uses named states with status codes (reason codes) attached to state changes.

**Agent-toggleable:**
- `Available` — can receive interactions; must click "Available" in control panel
- `On Break` — unavailable; click "Take Break"
- `Working Offline` — unavailable but retains access to all features
- `Logged Out` — signed out

**System-set:**
- `Busy / In Progress` — automatically set when handling an interaction
- `Post-Processing` — automatically set after interaction ends; ACW equivalent

**Status codes (reason codes):**
- Required before completing state changes (break, work offline, logout, reject)
- Examples: `Lunch`, `Restroom`, `Attend Meeting`, `Training`, `Technical Issue`
- Used for supervisor reporting on how agents spend time
- No "routable" flag on individual status codes — all break/offline codes are non-routable

**Channel-specific substates:** 8x8 uses "Post-Processing" as a unified ACW state across channels. Channel-level substates (voice hold, chat handling) are not surfaced as named agent states.

**Sources:** [8x8 status codes overview](https://docs.8x8.com/8x8WebHelp/VCC/configuration-manager-vovcc/content/statuscodespageoverview.htm), [8x8 Agent Workspace status codes](https://docs.8x8.com/8x8WebHelp/contact-center/agent-workspace/Content/aw/status-codes.htm)

---

#### Cisco Finesse (UCCX/UCCE)

Cisco Finesse exposes a precise state machine with explicit system-set and agent-set states.

**Agent-toggleable (can directly set):**
- `READY` — available to receive ACD calls
- `NOT_READY` — unavailable; can carry a Not Ready Reason Code
- `LOGOUT` — agent logs out

**System-set (agent cannot directly set these):**
- `RESERVED` — system sets when call is ringing/being offered to agent
- `RESERVED_OUTBOUND` — system sets for outbound preview calls
- `RESERVED_OUTBOUND_PREVIEW` — system sets for outbound preview mode
- `TALKING` — system sets when agent answers/accepts call
- `HOLD` — system sets when agent places caller on hold
- `WORK` — system sets during wrap-up when agent will transition to NOT_READY after ACW
- `WORK_READY` — system sets during wrap-up when agent will transition to READY after ACW
- `UNKNOWN` — transitional/error state

Transition path for inbound call: `READY → RESERVED → TALKING → WORK/WORK_READY`

**Not Ready Reason Codes:** Configurable per org in Finesse admin. Codes carry names (e.g., `Break`, `Lunch`, `Training`). Cisco also supports "interruptible" reason codes in some deployments — when queue thresholds are breached, the system can interrupt agents in certain AUX codes and offer them calls.

**Avaya analog:** Avaya uses `AUX Work` (with reason codes) instead of `NOT_READY`, and `Auto-In` / `Manual-In` instead of `READY`. `ACW (After Call Work)` is Avaya's wrap-up state.

**Break/Not Ready terminology:** Cisco calls them "Not Ready Reason Codes"; Avaya calls them "AUX Reason Codes" or "AUX Work Codes." Cisco Finesse supports interruptible reason codes (routing can interrupt AUX) in some contact center configurations. This is the closest vendor analog to the `routable: bool` flag on break reasons in Open Routing.

**Sources:** [Cisco Finesse Change Agent State API](https://developer.cisco.com/docs/finesse/user%E2%80%94change-agent-state/), [Cisco UCCX Not Ready Reason Codes](https://comstice.com/blog/post/cisco-uccx-reason-codes), [Finesse Admin Guide: Manage Reasons](https://www.cisco.com/c/en/us/td/docs/voice_ip_comm/cust_contact/contact_center/finesse/finesse_1201/Admin/guide/cfin_b_1201-administration-guide-release-1201/cfin_b_1201-administration-guide-release-1201_chapter_0110.html)

---

### 1.2 Cross-Vendor State Name Mapping

| Concept | Genesys Cloud | NICE CXone | Amazon Connect | Twilio Flex | Five9 | Talkdesk | 8x8 | Cisco Finesse |
|---------|-------------|-----------|---------------|------------|-------|----------|-----|--------------|
| **Ready / Available** | Available (presence) | Available (state) | Available (ROUTABLE) | Available (activity, available=true) | Ready | Available | Available | READY |
| **Not Ready / Away** | Away / Break / Meal / Meeting / Training | Unavailable (+ reason code) | Custom status (CUSTOM) | Unavailable / Break / custom (available=false) | Not Ready (+ reason code) | Away / custom | On Break / Working Offline | NOT_READY (+ reason code) |
| **WrapUp / ACW** | System (routing: Interacting + ACW) | Unavailable + IsACW flag | After contact work (activity state) | Wrap Up activity (available=true) | WRAP_UP (call state) | After Call Work | Post-Processing | WORK / WORK_READY |
| **Engaged / Talking** | Routing: Interacting | (no top-level state; inferred from contact events) | On contact (activity state) | On a Task (activity) | TALKING | On a Call | Busy / In Progress | TALKING |
| **Ringing / Reserved** | Routing: Interacting (alerting particle in conversation) | (contact-level) | Incoming (activity state) | reserved (task state) | (call event) | (implicit) | (implicit) | RESERVED |
| **On Hold (voice)** | Conversation participant: held | (contact-level operation) | CONNECTED_ONHOLD (contact state) | (voice conference level) | ON_HOLD (call state) | (call control) | (call control) | HOLD |
| **Offline / LoggedOut** | Offline (presence) | LoggedOut (session event) | Offline (OFFLINE) | Offline (activity, available=false) | (session end) | Offline | Logged Out | LOGOUT |
| **Break Reason name** | Secondary status | Unavailable Code | Custom status name | Activity name | Reason Code | Custom status name | Status Code | Not Ready Reason Code |
| **Routable flag on break reason** | No (primary presence controls routing) | No (all Unavailable codes are non-routable) | No (all CUSTOM statuses are non-routable) | Yes — `available: bool` on each Activity | No | No | No | Partially (interruptible codes in some deployments) |

**Key finding on `routable: bool` per break reason:**
No vendor exposes a per-reason "routable" flag in a simple admin-configurable form. The closest analogs are:
1. **Twilio TaskRouter** — `available: bool` on each Activity; this is structurally the same as `routable: bool` per break reason, but applies to the whole Activity state (including non-break states)
2. **Amazon Connect** — `Type: ROUTABLE | CUSTOM | OFFLINE` per agent status; all custom statuses are non-routable by design (not per-reason)
3. **Cisco Finesse** — interruptible AUX codes (contact center can interrupt agents on specific AUX codes), but this is a routing engine behavior, not an explicit flag the admin sets per code

Open Routing's design of `routable: bool` on each `break_reason` is **more granular than any vendor currently exposes in a simple admin UI**. This is a differentiator: it allows orgs to say "Lunch break = non-routable, Quick Bio Break = routable (agent can receive high-priority calls)."

---

### 1.3 Channel-Specific Engaged Sub-States

Across all vendors surveyed, channel-specific substates during engagement are handled **at the contact/conversation level**, not at the agent state level. The agent state machine remains simple (Ready / NotReady / WrapUp / Engaged / Offline). Channel detail lives below that layer.

**Voice channel — standard sub-states (contact level):**
- `Alerting / Ringing / CONNECTING` — call being offered, agent hasn't answered
- `Talking / CONNECTED / TALKING` — active voice call
- `On Hold / CONNECTED_ONHOLD / HOLD` — agent placed caller on hold
- `Conference` — three-way or multi-party call (not a distinct agent state; modeled as a conference participant in most platforms)
- `Consult` — blind consult before transfer (not a distinct agent state)
- `Transfer in progress` — completing a transfer (not a distinct agent state)
- `After Call Work / ACW / WrapUp` — agent completing post-call work

**Chat/messaging channel — standard sub-states:**
- `Alerting / Incoming` — chat being offered
- `Handling / Active / CONNECTED` — agent actively handling chat
- `Idle / Waiting` — chat open but waiting for customer reply (some vendors track this, most don't surface it as an agent state)
- `After Chat Work / ACW` — post-chat wrap-up (some platforms, e.g., Amazon Connect, apply ACW to chat)

**Email channel — standard sub-states:**
- Amazon Connect: async; no "ringing" state; contact goes directly to CONNECTED when assigned
- Most platforms: email assignment is direct, no alerting phase
- Post-email ACW applies in Amazon Connect, Genesys Cloud

**Recommendation for Open Routing v0.1 Engaged sub-states:**

The research shows that tracking channel sub-states at the **agent state machine level** is not standard vendor practice. Vendors model it at the contact/conversation level. For v0.1:

- `Engaged` (top-level system-set state) is sufficient as a single bucket
- Sub-states like `voice:talking`, `voice:hold`, `voice:conference`, `chat:handling`, `email:composing` belong on the **interaction/contact object**, not on the agent state record
- The agent state machine transitions: `Ready → Engaged → WrapUp → Ready`
- The interaction layer carries its own state: `ringing → connected → held → connected → ended`

This means v0.1's agent state model is **correct as designed** without enumerating channel sub-states on the agent entity itself.

---

### 1.4 Recommended Agent State Model for v0.1

Based on the vendor survey:

**Agent-toggleable states (agent can set these):**
| State | Vendor Analogs | Routing behavior |
|-------|---------------|-----------------|
| `Ready` | Available (all vendors) | Routable |
| `NotReady` | Unavailable/Away (all vendors); no sub-reasons | Not routable |
| `Break` | On Break / AUX Work / custom status; has configurable sub-reasons | Per-reason `routable: bool` |

**System-set states (routing engine sets these):**
| State | Vendor Analogs | Notes |
|-------|---------------|-------|
| `WrapUp` | ACW / After Contact Work / Post-Processing | Settable: org configures whether WrapUp is routable or not |
| `Engaged` | On contact / TALKING / On a Task / In Progress | Set when interaction assigned; cleared when WrapUp begins |
| `Offline` | Offline / LoggedOut / LOGOUT | Agent not connected |

**Break reasons entity (break_reasons table, per org):**
| Field | Type | Notes |
|-------|------|-------|
| `id` | UUID | |
| `org_id` | UUID | org-scoped |
| `name` | string | e.g., "Lunch", "Training", "Bio Break" |
| `routable` | bool | If true: routing engine may route to agent even while on this break |
| `display_order` | int | Order shown in agent UI |
| `enabled` | bool | Can be soft-deleted |

This model is validated against all 8 vendors surveyed. The `routable: bool` on break reasons is more expressive than any vendor's current admin UI, making it a deliberate differentiator.

---

## 2. Catalog Entity Shapes

### 2.1 Agents

**Minimum v0.1 shape (what every vendor has):**
```
agents {
  id              UUID
  org_id          UUID
  external_id     string?     // for import/sync from HR/IdP
  name            string
  email           string
  username        string?
  status          enum        // current state (live, from projections)
  skills          []AgentSkill
  channels        []string    // which channels this agent handles
  default_language string?
  enabled         bool
  created_at      timestamp
  updated_at      timestamp
}
```

**Common variations across vendors:**
- Genesys Cloud: agents belong to divisions; have queues they're members of
- Amazon Connect: agents belong to routing profiles (which map to queues + channels)
- Five9: agents belong to skills groups and campaigns
- NICE CXone: agents belong to teams; teams own unavailable code assignments
- Twilio Flex: workers have a JSON attributes blob (no fixed schema)

**v0.1 minimal / v1 full:**
- v0.1: `id, org_id, external_id, name, email, enabled, created_at`
- v1 adds: `routing_profile_id`, `division_id`, `teams`, `time_zone`, `locale`

---

### 2.2 Skills

**Minimum v0.1 shape:**
```
skills {
  id              UUID
  org_id          UUID
  external_id     string?
  name            string      // "Spanish", "Billing", "VIP Support"
  description     string?
  skill_type      enum        // language | domain | technical | queue_access
  enabled         bool
  created_at      timestamp
}
```

**Skill level/proficiency format across vendors:**
- **NICE CXone:** 1–20 (integer), where 1 = highest proficiency
- **Bright Pattern:** 0–100 (integer); best practice is multiples of 25 (100, 75, 50, 25)
- **Genesys Cloud:** 1–10 (star rating with half stars); rule of thumb: use only 3–5 distinct levels
- **Webex Contact Center:** numeric proficiency score; higher = better match
- **RingCentral Engage Voice:** binary (skill assigned or not; no proficiency level)
- **Twilio TaskRouter:** arbitrary worker attributes (JSON); skills are custom attributes

**Recommendation for v0.1:**
Use integer `1–10` with `10 = expert`. This aligns with Genesys Cloud's user-facing model and is granular enough for routing without Bright Pattern's over-engineering. Map to vendor scales at adapter layer.

```
agent_skills {
  agent_id        UUID
  skill_id        UUID
  proficiency     int         // 1-10, default 5; 10 = expert
}
```

---

### 2.3 Queues

**Minimum v0.1 shape:**
```
queues {
  id              UUID
  org_id          UUID
  external_id     string?
  name            string
  description     string?
  channel_types   []string    // ["voice", "chat", "email"]
  priority        int         // default priority for interactions in this queue
  max_wait_sec    int?        // SLA target
  acw_sec         int?        // wrap-up time allocation
  enabled         bool
  created_at      timestamp
}
```

**Common vendor queue fields:**
- **Genesys Cloud:** mediaSettings per channel (alertingTimeoutSeconds, serviceLevel), ACW modes (5 options), routing method (Standard/Predictive/Bullseye/Preferred Agent/Conditional), max 5000 members
- **Amazon Connect:** routing profile maps queues + channel priorities; queue itself is simpler
- **NICE CXone:** skills are the primary routing entity; queues are skill groups
- **Twilio TaskRouter:** queues are TaskQueues with expression-based worker matching

**v0.1 minimal:** name, channel_types, priority, acw_sec
**v1 adds:** routing_method, acw_mode, service_level targets, max_members, wrapup_codes, in_queue_flow_id

---

### 2.4 Channels

**Minimum v0.1 shape:**
```
channels {
  id              UUID
  org_id          UUID
  external_id     string?
  name            string      // "Main Voice Line", "Live Chat Widget"
  channel_type    enum        // voice | chat | email | sms | social
  description     string?
  default_queue_id UUID?
  enabled         bool
  created_at      timestamp
}
```

Channel is a registry entry in v0.1 — a named inbound surface. No execution details.

---

### 2.5 Adapters

**Minimum v0.1 shape (registry only — no SDK execution in v0.1):**
```
adapters {
  id              UUID
  org_id          UUID
  name            string      // "FreeSWITCH Bridge", "LiveKit Voice"
  adapter_type    string      // "voice" | "chat" | "email"
  description     string?
  config          jsonb       // adapter-specific config blob (stored, not executed)
  enabled         bool
  created_at      timestamp
}
```

---

### 2.6 Break Reasons

```
break_reasons {
  id              UUID
  org_id          UUID
  name            string      // "Lunch", "Training", "Bio Break"
  routable        bool        // if true: routing may interrupt this break
  display_order   int
  enabled         bool
  created_at      timestamp
}
```

---

## 3. MFE Embedding Patterns for B2B Config UIs

### 3.1 Pattern Survey

**Stripe Connect Embedded Components**
- Technology: **Web Components** (custom HTML elements registered by Connect.js) with **iframe-based network isolation** at the CSP level
- Auth flow: host backend creates an `AccountSession` with `client_secret`; client passes `fetchClientSecret` to `loadConnectAndInitialize()`; the secret encodes delegated permissions per connected account
- Multi-org: different `client_secret` per user/account → scoped access
- Features enabled/disabled per session via the AccountSession components config
- Pattern: server-initiated session → client renders component → component fetches from Stripe directly
- CSP requirement: `frame-src https://connect-js.stripe.com https://js.stripe.com`

**Vercel Toolbar / Microfrontends**
- Vercel's own internal platform uses **vertical microfrontends** deployed as separate Next.js apps under a shared domain
- The Vercel Toolbar is an injected JavaScript overlay (not iframe); runs in page context
- For customer-facing microfrontend deployments: supports Module Federation (Webpack 5) and multi-zone routing
- Vercel's adoption improved preview build times by 40%+

**PostHog Toolbar**
- Technology: **JavaScript injection** into the host page (not iframe, not web component)
- Launches by injecting `#__posthog=JSON` token in the URL; toolbar runs in same page context
- Pattern: CDN-hosted script loaded at runtime; overlay UI rendered in host document
- Limitation: requires host app to not override the injected token

**Auth0 Universal Login**
- Universal Login (hosted): **redirect-based** — not embedded at all; browser navigates away
- Embedded Login (deprecated pattern): runs in host page using `auth0.js`; deprecated in favor of Universal Login
- Key lesson: Auth0 **explicitly discourages iframe-based auth** due to X-Frame-Options: deny headers
- Multi-domain SSO only works with Universal Login (redirect), not embedded login

### 3.2 Pattern Comparison for Embedded Config Surface

| Pattern | Isolation | Context passing | Auth | Routing | Best for |
|---------|-----------|-----------------|------|---------|----------|
| **iframe** | Strong (browser sandbox) | `postMessage` or URL params | Host-managed session cookie or token in URL | Host controls navigation | Untrusted or 3rd-party embeds; legacy compat |
| **Module Federation** | None (same JS runtime) | React context, props, Zustand | Shared from host | React Router | Same-team, same-framework micro frontends |
| **Web Components** | Shadow DOM (style only) | Attributes, custom events | Via `fetchClientSecret` pattern or JWT attribute | Internal routing | Cross-framework embed; Stripe Connect model |
| **Script injection** | None (same DOM) | Global variables or events | Host-managed | N/A | Toolbars, overlays (PostHog, Vercel toolbar) |

### 3.3 Recommendation for Open Routing MFE Config UI

For an **embedded read/write config surface with multi-page UX, multi-org context, and host-app auth trust**:

**Use Module Federation** for the primary MFE embed pattern.

Rationale:
1. Open Routing targets **product teams** (not external untrusted contexts) — they control the host app and share the same trust boundary
2. Module Federation enables native React integration: shared context (org_id, theme tokens, auth), React Router for multi-page flows, shared design system
3. The host passes `org_id` via a shared context/store; MFE reads it to scope all API calls
4. No CSP complexity vs iframe; no attribute-bridge complexity vs Web Components
5. Real-world use: RingCentral uses Module Federation for their contact center MFEs; Vercel uses it internally

**For teams needing iframe isolation** (untrusted host contexts, strict CSP environments):
- Maintain iframe support as the secondary embed path (per PROJECT.md requirement)
- iframe variant communicates via `postMessage` for org_id and auth token

**Auth context in embedded config:**
- v0.1: host passes `org_id` in a trusted header or context prop (stub auth)
- v1+: follow Stripe Connect model — host backend creates a scoped session token; MFE's API calls include it

**Theme tokens:** exposed as CSS custom properties; host can override them without MFE rebuild

**Sources:** [Stripe Connect embedded components](https://docs.stripe.com/connect/get-started-connect-embedded-components), [Vercel microfrontends](https://vercel.com/docs/microfrontends), [PostHog toolbar docs](https://posthog.com/docs/toolbar), [Module Federation patterns](https://blog.logrocket.com/solving-micro-frontend-challenges-module-federation/)

---

## 4. Bulk JSON/CSV Import Patterns

### 4.1 Validation and Error Feedback

**Standard patterns (from HubSpot, Salesforce, Intercom, open-source):**

1. **Parse → Validate → Import pipeline:** Split into explicit stages so errors are caught before writes occur
2. **Row-level error reporting:** Each failing row gets: row number, field name, error code, human-readable message
3. **Downloadable error file:** Return a CSV of failed rows with appended error columns (Salesforce Bulk API, HubSpot exports)
4. **Dry-run mode (validation-only):** Run the full parse + validate pipeline without committing any writes; return a summary of what would succeed/fail. Called "dry run" (Salesforce, some REST APIs) or "validate" mode

### 4.2 Partial Success vs All-or-Nothing

- **Bulk (partial success, 207 Multi-Status):** Process each record independently; log failures, continue with successes. Return `201 Created` if all succeed, `207 Multi-Status` if partial, `400` if all fail. Best for large imports where a few bad rows shouldn't block everything.
- **Batch (all-or-nothing, transactional):** All records in a single transaction; any failure rolls back all. Best for imports that must be consistent (e.g., flow configurations with inter-record references).

**Recommendation for Open Routing v0.1:** Use **partial success (bulk) model** for catalog imports. Agents, skills, queues, and break_reasons are independent records — no cross-entity dependency within a single import batch. Return per-row success/failure.

### 4.3 Async vs Sync

- **Sync (immediate response):** Acceptable for batches under ~1000 records with fast validation. Returns full results in the HTTP response.
- **Async (job-based):** Required for large files (>10K records) or when validation is expensive. Client gets a `job_id`; polls `GET /import-jobs/{id}` for status (`PENDING`, `PROCESSING`, `COMPLETE`, `FAILED`); downloads results when complete.

**HubSpot pattern:** `POST /crm/v3/exports/export/async` → returns `exportId`; poll `GET /crm/v3/exports/export/async/tasks/{exportId}/status`; download when `COMPLETE`.

**Recommendation for v0.1:** Sync for under 500 rows (reasonable for initial catalog seeding). Add async job support in v1 when orgs need to import thousands of agents from HR systems.

### 4.4 Idempotency and Deduplication

- Use `external_id` as the stable deduplication key (upsert semantics: create if not exists, update if exists)
- Support `Idempotency-Key` header for safe retry of the same import file
- No `external_id` = no reliable deduplication; treat as create-only

**Best practice flow:**
```
POST /orgs/{org_id}/imports
  Content-Type: multipart/form-data
  Idempotency-Key: {stable-key-for-this-file}
  Body: file (CSV or JSON)

Response (sync, <500 rows):
  207 Multi-Status
  {
    "summary": { "total": 150, "created": 148, "updated": 2, "failed": 0 },
    "rows": [
      { "row": 1, "status": "created", "id": "..." },
      ...
    ]
  }
```

**Sources:** [Bulk vs batch API design (Tyk)](https://tyk.io/blog/api-design-guidance-bulk-and-batch-import/), [Partial success in bulk APIs](https://oneuptime.com/blog/post/2026-02-02-rest-bulk-api-partial-success/view), [HubSpot async export](https://developers.hubspot.com/docs/api-reference/crm-exports-v3/guide)

---

## 5. Feature Landscape

### 5.1 Table Stakes (Must Exist in v0.1)

Features that make the catalog usable. Missing any of these makes v0.1 feel unfinished.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| CRUD for all 6 entities (agents, skills, queues, channels, adapters, break_reasons) | Every ACD product provides this; without it there's nothing to configure | LOW | Standard REST; no special patterns |
| Org-scoped isolation (org_id on all entities) | Multi-org is table stakes for embedded B2B SaaS | LOW | Already in data model; enforce at query layer |
| Agent status model: Ready / NotReady / Break / WrapUp / Engaged / Offline | Every ACD vendor has this; routing is impossible without it | MEDIUM | Break reasons with routable:bool is the v0.1 design |
| Configurable break reasons per org | All vendors (Genesys, NICE, Amazon Connect, Cisco) support org-configurable reason codes | LOW | break_reasons table; supported by all 8 vendors surveyed |
| Agent-skill association with proficiency | All ACD vendors associate agents to skills; proficiency is universal | LOW | agent_skills join table with proficiency 1-10 |
| Bulk CSV/JSON import | Product teams need to seed catalogs from existing HR/CRM data | MEDIUM | Upsert by external_id; row-level errors; sync for <500 rows |
| external_id field on all entities | Deduplication, sync from external systems | LOW | String field; unique per org |
| Enabled/disabled flag on all entities | Soft delete is universal in B2B SaaS catalogs | LOW | bool field; filter disabled in routing queries |
| REST management API | All vendors expose REST for catalog config | LOW | Standard REST CRUD |

### 5.2 Differentiators (Compelling but Optional for v0.1)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| `routable: bool` on break reasons | More granular than any vendor surveyed; allows "Bio Break = routable, Lunch = not" | LOW | Simple bool; logic in routing engine later |
| MFE-embeddable config UI | Vendors lock orgs into their own dashboards; Open Routing embeds in the host product | HIGH | Module Federation; org_id from host context |
| Import dry-run mode | Validate import without committing; rare but valued in enterprise | MEDIUM | Validation pipeline reused |
| CSV error file download | Download failed rows with error annotations; Salesforce/HubSpot standard | MEDIUM | CSV generation on backend |
| Audit log for catalog changes | Who changed what, when — expected in enterprise | MEDIUM | Append-only audit events; deferred to v1 |

### 5.3 Anti-Features (Things ACD Products Have That Open Routing Should NOT Build in v0.1)

These are common in ACD suites but violate Open Routing's "routing-only" boundary or create premature complexity.

| Anti-Feature | Why ACD Products Have It | Why Open Routing Should NOT Build It in v0.1 | Alternative |
|-------------|--------------------------|---------------------------------------------|-------------|
| Agent desktop UI (softphone, chat panel, email client) | ACD vendors own the agent experience | Violates routing-only boundary; PROJECT.md explicitly excludes agent desktop | Host product builds the desktop; Open Routing provides state model only |
| Live channel execution (holding calls, streaming chat) | ACD vendors are the media platform | Open Routing is routing-only; adapters own execution | Adapter SDK contract (v1); adapters handle media |
| WFM (scheduling, adherence, forecasting) | ACD vendors upsell this | Out of scope per PROJECT.md | External WFM system via adapter |
| CRM fields on agent entity (contact history, customer data) | ACD vendors bundle CRM | Creates CRM dependency; Open Routing's catalog is routing-relevant only | CRM stays external; enrichment via webhook in flows |
| Reporting and analytics dashboards | ACD vendors embed BI tools | Out of scope for v0.1; needs live runtime data | Org-facing audit (v1); external BI via data export |
| Real-time agent monitoring (barge, whisper, listen) | Supervisor tools in every ACD suite | Requires media access; violates boundary | Adapter SDK can implement; not catalog scope |
| SAML/SSO/RBAC per org | Enterprise ACD products require per-org SSO | Full auth is explicitly deferred to a later milestone per PROJECT.md | Stub auth (org_id header) in v0.1; full auth in v1 |
| Per-org database (tenant DB isolation) | Some enterprise platforms offer this | Adds massive operational complexity for unclear early benefit | Shared DB + org_id enforcement is correct for v0.1 |
| Presence sync with IdP (LDAP/AD) | Enterprise ACD syncs agent roster from HR | Premature; no external sync in v0.1 per PROJECT.md | JSON/CSV import is the v0.1 sync mechanism |

---

## 6. Feature Dependencies

```
break_reasons (configured per org)
    └──required by──> agent Break state (Break reasons are Break's sub-reasons)

agents
    └──requires──> org_id (multi-org isolation)
    └──requires──> skills (for skill-based routing, agents need assigned skills)
    └──requires──> break_reasons (to set Break state with a reason)

skills
    └──required by──> agent_skills (proficiency association)
    └──required by──> queues (queue targets skills in routing, later)

queues
    └──requires──> channels (queue handles interactions from a channel type)

channels
    └──optionally references──> adapters (channel has an adapter that bridges it)

MFE config UI
    └──requires──> REST CRUD API (UI reads/writes via REST)
    └──requires──> org_id context passing (from host app)

Bulk import
    └──requires──> external_id on entities (deduplication)
    └──requires──> entity CRUD (import calls create/update)
```

---

## 7. MVP Definition (v0.1)

### Launch With

- [ ] REST CRUD for all 6 entities: agents, skills, queues, channels, adapters, break_reasons
- [ ] org_id on all entities; enforced at app layer
- [ ] Agent status model: Ready, NotReady, Break (with break_reasons), WrapUp, Engaged, Offline
- [ ] break_reasons entity with `routable: bool`
- [ ] Agent-skill association with proficiency (1–10)
- [ ] `external_id` and `enabled` fields on all entities
- [ ] Bulk CSV/JSON import: upsert by external_id, row-level errors, sync for <500 rows, partial success (207)
- [ ] MFE-embeddable Catalog config UI (Module Federation; org_id from host context)

### Add After Validation (v1)

- [ ] Import dry-run/validation mode
- [ ] CSV error file download
- [ ] Async import jobs (for >500 rows)
- [ ] Audit log for catalog changes
- [ ] Org-facing audit UI
- [ ] Full auth/RBAC (deferred per PROJECT.md)
- [ ] Live external sync from IdP/HR

### Future (v2+)

- [ ] WFM integration
- [ ] Per-org SSO configuration
- [ ] Advanced skill proficiency tiers (with custom tier names)
- [ ] Adapter execution (real FreeSWITCH/LiveKit bridges)

---

## 8. Competitor Feature Analysis (Catalog + Agent State Layer Only)

| Feature | Genesys Cloud | NICE CXone | Amazon Connect | Twilio Flex | Open Routing Approach |
|---------|-------------|-----------|---------------|------------|----------------------|
| Agent state model | Presence + Routing Status (two layers) | Available/Unavailable + reason codes | ROUTABLE/CUSTOM/OFFLINE type on status | Activity with available:bool | Single state enum; break_reasons with routable:bool |
| Break reason "routable" flag | No (secondary statuses are cosmetic) | No (all Unavailable codes are non-routable) | No (all CUSTOM types are non-routable) | Yes (available:bool per Activity) | Yes — first-class `routable: bool` per break_reason |
| Skill proficiency | 1-10 stars | 1-20 (1=best) | Via routing profiles | Custom JSON attributes | 1-10 (10=expert); normalized |
| Bulk import | CSV via admin UI; no public row-level error API | CSV via admin | CSV via admin | No native bulk import | REST API with row-level errors and external_id upsert |
| Embeddable config UI | No (own dashboard only) | No | No | Flex UI is a full embedded product | Module Federation embed in host product |
| Multi-org isolation | Divisions (within one org) | Teams | AWS accounts | Workspaces | org_id on all entities; true multi-org |

---

## 9. Sources

- [Genesys Cloud routing status glossary](https://help.genesys.cloud/glossary/routing-status/)
- [Genesys Cloud agent presence and activity indicators](https://help.genesys.cloud/articles/agent-presence-status-and-activity-indicators/)
- [Genesys Cloud secondary status overview](https://help.genesys.cloud/articles/about-statuses/)
- [Genesys Cloud create queues](https://help.genesys.cloud/articles/create-queues/)
- [NICE CXone Set Agent State action](https://help.nice-incontact.com/content/studio/actions/setagentstate/setagentstate.htm)
- [NICE CXone Set Up Unavailable Codes](https://help.incontact.com/Content/ACD/UnavailableCodes/SetUpUnavailableCodes.htm)
- [NICE CXone Agent Session Events](https://developer.niceincontact.com/Documentation/AgentSessionEvents)
- [Amazon Connect agent status in CCP](https://docs.aws.amazon.com/connect/latest/adminguide/metrics-agent-status.html)
- [Amazon Connect about contact states](https://docs.aws.amazon.com/connect/latest/adminguide/about-contact-states.html)
- [Amazon Connect CreateAgentStatus API](https://docs.aws.amazon.com/connect/latest/APIReference/API_CreateAgentStatus.html)
- [Amazon Connect metric definitions](https://docs.aws.amazon.com/connect/latest/adminguide/metrics-definitions.html)
- [Twilio TaskRouter Activity Resource](https://www.twilio.com/docs/taskrouter/api/activity)
- [Twilio TaskRouter task state lifecycle](https://www.twilio.com/docs/taskrouter/lifecycle-task-state)
- [Twilio Flex Activity Reservation Handler](https://twilio-professional-services.github.io/flex-project-template/feature-library/activity-reservation-handler)
- [Five9 CRM SDK global constants](https://cdn.prod.us.five9.net/stable/crm-sdk-lib/doc/global.html)
- [Five9 FAQ: What is Not Ready](https://www.five9.com/faq/what-is-not-ready)
- [Talkdesk Agent Status and availability](https://support.talkdesk.com/hc/en-us/articles/200496719)
- [8x8 status codes overview](https://docs.8x8.com/8x8WebHelp/VCC/configuration-manager-vovcc/content/statuscodespageoverview.htm)
- [Cisco Finesse Change Agent State API](https://developer.cisco.com/docs/finesse/user%E2%80%94change-agent-state/)
- [Cisco UCCX Not Ready Reason Codes (Comstice)](https://comstice.com/blog/post/cisco-uccx-reason-codes)
- [Bright Pattern skill levels](https://help.brightpattern.com/5.19:Contact-center-administrator-guide/UsersandTeams/SkillLevels)
- [Stripe Connect embedded components](https://docs.stripe.com/connect/get-started-connect-embedded-components)
- [Vercel microfrontends](https://vercel.com/docs/microfrontends)
- [PostHog toolbar docs](https://posthog.com/docs/toolbar)
- [Module Federation challenges and solutions (LogRocket)](https://blog.logrocket.com/solving-micro-frontend-challenges-module-federation/)
- [Bulk vs batch API design (Tyk)](https://tyk.io/blog/api-design-guidance-bulk-and-batch-import/)
- [Partial success in bulk REST APIs](https://oneuptime.com/blog/post/2026-02-02-rest-bulk-api-partial-success/view)
- [HubSpot async export API](https://developers.hubspot.com/docs/api-reference/crm-exports-v3/guide)
- [RingCentral Engage Voice skill profile](https://developers.ringcentral.com/engage/voice/guide/users/agents/skill-profile)

---

*Feature research for: Open Routing v0.1 Catalog Foundation*
*Researched: 2026-05-15*
