---
type: North Task
north_schema: north-plan-v1
id: T02
task_status: pending
depends_on: [T01]
agent: north-worker
write_scope: [assets/opencode/instructions/core.md, assets/claude/instructions/core.md, README.md, docs/architecture.md, docs/plan-format.md]
---
# T02: Align tool instructions and documentation

Plan: [Parallel execution](../plan.md)
Prerequisites: [T01](T01-execution-guidance.md).
Research: [Integration surfaces](../research.md)

## Outcome

Route both host coordinators to the stronger execution preference and describe it consistently without duplicating the full policy.

## Acceptance

- Add a concise cue in both core instructions to look for compatible independent work and use the existing delegation owner; preserve scope/permission limits and standalone behavior when the pipeline is not selected.
- Align README/architecture/plan guide only where needed: default cap two, same-layer concurrency, safe refill and retained barriers.
- Audit both tools' worker/verifier definitions and command wrappers read-only. Confirm no nested delegation permission or implicit phase transition is introduced. Report contradictions outside scope before requesting expansion.
- No installer/dependency/native-frontmatter changes are needed. Validate relevant references and native metadata; use existing installation checks only if routing/packaging behavior changes, not wording-matching tests.
- Record exact static checks and limits for T03/T04.

## Execution

Dispatch/session: Not dispatched.
Checkout/branch/base commit: Not assigned.
Changed files: None.
Validation: None.
Review/integration: Not reviewed.
Blockers: None known.

## History

Created during planning; no implementation started.
