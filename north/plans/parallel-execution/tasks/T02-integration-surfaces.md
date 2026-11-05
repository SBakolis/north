---
type: North Task
north_schema: north-plan-v1
id: T02
task_status: done
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

Dispatch/session: `ses_e5272b7eeffe0li1t3NEXFk7uR`, completed.
Checkout/branch/base commit: Shared source `/Users/stamatis/Documents/projects/north` / `main` / `d32898f`; T01 accepted.
Changed files: Both tool core instructions, README.md, docs/architecture.md, docs/plan-format.md.
Validation: Worker diff check, 38 local reference checks, core-cue parity and ten unchanged native-header/permission checks passed. Primary reviewed full diff and clarified the cue to say workers cannot delegate further.
Review/integration: Accepted after primary diff review and staged whitespace check; committed on source main as `9f5f7ca`. No packaging changes; installer suite not rerun for wording-only edits.
Blockers: None known.

## History

Created during planning; no implementation started.
