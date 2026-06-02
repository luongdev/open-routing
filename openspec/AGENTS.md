# OpenSpec Guidance

## Source of Truth

Use `openspec/specs/` for current behavior. Use `openspec/changes/` for proposed or active changes. `.planning/` remains available as historical context from the pre-OpenSpec workflow, but new planning should be written in OpenSpec form.

## Change Workflow

1. Read `openspec/project.md` and the relevant capability specs.
2. For behavior changes, create or update one folder under `openspec/changes/<change-name>/`.
3. Keep `proposal.md`, `tasks.md`, optional `design.md`, and delta specs together.
4. Delta specs use `## ADDED Requirements`, `## MODIFIED Requirements`, and `## REMOVED Requirements`.
5. When a change ships, merge the resulting behavior into `openspec/specs/` and move the change to `openspec/changes/archive/YYYY-MM-DD-<change-name>/`.

## Project Rules

Follow the repository `AGENTS.md` rules. In particular, do not over-comment code. Comments are rare and must explain rationale, constraints, invariants, trade-offs, or decision references.
