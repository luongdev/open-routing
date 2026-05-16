# AGENTS.md — Open Routing project rules (Codex, Cursor, Aider, etc.)

## Comments — HARD RULE

Don't over-comment code. Default: **no comments**.

If a comment is necessary, it MUST explain **WHY** (rationale, constraint,
invariant, gotcha, trade-off) — never WHAT or HOW. The code already shows
what and how; comments that restate it are noise.

**Bad — DELETE:**
```go
// Loop through items and sum
for _, x := range items { total += x }

// Return 404 if not found
if errors.Is(err, pgx.ErrNoRows) { return 404 }
```

**Good — KEEP:**
```go
// Use BeginTx so agent UPDATE + skills replace share one tx — mid-INSERT
// failure must roll back the agent row too (Codex C4 iter 3).
tx, err := h.deps.OrgDB.BeginTx(ctx)

// 23503 → 422 invalid_reference: defense-in-depth for races where the
// referenced row is deleted between handler probe and INSERT.
case "23503":
```

**When a comment is allowed (rare):**
- Hidden constraint, subtle invariant, bug workaround, trade-off taken,
  or a pointer to a decision (D-NN in CONTEXT.md, ROADMAP CRIT, PR).

**Prefer over comments:** self-documenting names, small intention-revealing
functions, tests as living docs, decision logs in `.planning/`.

**Never write:**
- Docstrings restating the signature.
- "Removed X" / "Added Y" / "Used by Z" notes — git history shows that.
- TODOs without owner + tracking issue.
- Multi-paragraph headers above trivial helpers.

Comment line count should be a small fraction of code line count. If a
function needs a paragraph of comments to be understandable, refactor it.

## Cross-AI Peer Review (carried in via ~/.codex/AGENTS.md)

After completing any non-trivial work (≥3 commits or ≥5 files), invoke
peer review on the diff before declaring done. Codex → Gemini. See
user-global `~/.codex/AGENTS.md` for the invocation pattern.
