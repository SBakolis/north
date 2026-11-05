---
type: North Task
north_schema: north-plan-v1
id: T04
task_status: pending
depends_on: [T01, T02, T03]
agent: north-verifier
write_scope: []
---
# T04: Verify integrated execution guidance

Plan: [Parallel execution](../plan.md)
Prerequisites: [T01](T01-execution-guidance.md), [T02](T02-integration-surfaces.md), [T03](T03-behavioral-evaluation.md).
Research: [Findings and evidence limits](../research.md)

## Outcome

Read-only acceptance review of the combined instructions, documentation and observed evaluation evidence.

## Acceptance

- Confirm the change adds specific launch/refill/overlap behavior rather than only stronger adjectives, and remains consistent across shared skills and both tool cores.
- Confirm default cap, explicit limits, layer barriers, review-before-done, primary-owned integration/records, no worker nesting, and legacy/OKF behavior remain intact.
- Inspect actual positive-case launch ordering and negative-case outcomes. Static examples or worker summaries alone do not prove concurrency; request missing execution evidence from the primary.
- Confirm no broad method-skill rewrite, new runtime/permission, cap increase or automatic phase invocation entered the change.
- Report actionable findings and bounded coverage. Primary performs required checks, accepts repairs and updates the plan; the verifier does not mutate files or run prohibited commands.

## Execution

Dispatch/session: Not dispatched.
Checkout/branch/base commit: Not assigned.
Changed files: None.
Validation: None.
Review/integration: Not reviewed.
Blockers: None known.

## History

Created during planning; no implementation started.
