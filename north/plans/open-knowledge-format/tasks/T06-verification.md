---
id: T06
status: done
depends_on: [T01, T02, T03, T04, T05]
agent: north-verifier
write_scope: []
---

# T06: Verify integrated change

Plan: [Open Knowledge Format](../index.md)
Prerequisites: [T01](T01-contract.md), [T02](T02-pipeline.md), [T03](T03-supporting-skills.md), [T04](T04-integration-surfaces.md), [T05](T05-evaluation.md).
Research: [Scope and evidence](../research.md)

## Outcome

Read-only acceptance review of the integrated change against the confirmed user scope, pinned OKF specification, compatibility requirements, and actual validation evidence.

## Acceptance

- Confirm all sixteen identified skills have appropriate adaptations and no writer/reader contradicts the shared contract.
- Confirm task execution lifecycle, evidence ownership, phase boundaries, legacy selection, source provenance and generated-skill behavior satisfy the plan's concrete examples.
- Check both tool surfaces, installer dependency availability, user documentation and all new local reference targets.
- Review actual T04 checks and T05 observed outputs; distinguish conformance, North workflow validation, static inspection and unavailable cross-tool runs. Request missing checks from the primary; do not run prohibited commands or edit plans.
- Confirm no unrequested bulk migration, native-format conversion, runtime installation, or fabricated verification has been introduced.
- Return actionable findings with locations and acceptance coverage. The primary repairs through owning tasks, rechecks affected evidence, and updates statuses; completion requires reviewed integrated outcomes rather than a worker report alone.

## Execution

Dispatch/session: `ses_e573ba391ffenaJ3XLeI8DJH0K`, first review completed with one P2 finding.
Checkout/branch/base commit: Integration checkout / `feature/okf-integration` / `ae5cadd`.
Changed files: No verifier edits; primary repaired T01/T02 and added T05 regression evidence in `25bbcdb`.
Validation: Verifier inspected baseline and focused repair without mutation. Final primary checks on main: cargo test (45 passed), baseline artifact replay (249 assertions, zero failures), focused legacy-extension replay (passed).
Review/integration: Same verifier session re-reviewed `25bbcdb`: no remaining actionable findings, bounded acceptance supported. Primary accepted and fast-forwarded implementation to main; six clean merged worktrees/branches removed.
Blockers: None. Explicit coverage limits retained in evaluation report and index; no cross-runtime or exhaustive conformance claim.

## History

Created during planning; no implementation started.

First review found legacy task-extension ambiguity; T01/T02 reopened and T05 supplemented. Second review accepted the repair and native regression fixture. Final source-tree checks passed; all task outcomes integrated.
