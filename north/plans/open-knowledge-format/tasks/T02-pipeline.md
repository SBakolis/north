---
id: T02
status: done
depends_on: [T01]
agent: north-worker
write_scope: [assets/skills/north-plan/SKILL.md, assets/skills/north-explore/SKILL.md, assets/skills/north-execute/SKILL.md, assets/skills/north-save/SKILL.md, assets/skills/invoke-memory/SKILL.md, assets/skills/subagent-usage/SKILL.md]
---

# T02: Adapt planning, execution, and memory skills

Plan: [Open Knowledge Format](../index.md)
Prerequisites: [T01](T01-contract.md).
Research: [Pipeline impact](../research.md)

## Outcome

Make all six pipeline/delegation consumers use the shared profile for writing, selecting, updating, linking and reading new or legacy records.

## Acceptance

- New plans use the new navigation/overview split and typed research/tasks. Exploration returns provenance sufficient for the primary writer.
- Executor and delegation guidance distinguish task progress from lifecycle and carry the correct authoritative plan pointer; layer barriers and primary-only progress updates remain intact.
- Memory template is a typed concept with evidence-backed provenance and coverage; indexes are listings, and both reserved filenames are protected.
- Memory retrieval accepts legacy and new records, unknown types/extensions, absent optional trust data and both link forms while checking claims against source evidence.
- Meaningful updates preserve unknown keys and existing history; no `done` status automatically grants `verified` or human review.
- Walk through new plan -> execute -> save -> read and legacy resume cases; retain actual behavioral validation for T05. Do not convert this active bootstrap plan as a worker side effect.

## Execution

Dispatch/session: `ses_e574bc87dffeyWrsiYjEpbgb6P`, completed.
Checkout/branch/base commit: `north-okf-t02` / `feature/okf-t02` / `dc54069` (temporary parent in plan).
Changed files: All six scoped SKILL.md files.
Validation: Worker static header/link checks and new/legacy walkthrough passed; primary reviewed full diff and independently ran git diff --check (passed). Behavioral checks assigned to T05.
Review/integration: Accepted; `d5bf356` integrated, combined tree `7d721b2`.
Blockers: None known.

## History

Created during planning; no implementation started.

T06 legacy-extension finding invalidated unconditional new-task metadata guidance. Primary aligned north-plan with the repaired contract; focused regression pending.

Repair accepted in `25bbcdb`: added legacy tasks use legacy status, graph remains selectable; primary checked emitted metadata and replay passed.
