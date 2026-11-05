---
type: North Task
north_schema: north-plan-v1
id: T03
task_status: pending
depends_on: [T01, T02]
agent: north-worker
write_scope: [north/evaluations/parallel-execution/]
---
# T03: Evaluate observable concurrency behavior

Plan: [Parallel execution](../plan.md)
Prerequisites: [T01](T01-execution-guidance.md), [T02](T02-integration-surfaces.md).
Research: [Evaluation design](../research.md)

## Outcome

Retain meaningful evidence that revised guidance encourages concurrent work and respects necessary serialization. A primary coordinator conducts native dispatch cases; an assigned worker may prepare/analyze isolated fixtures and return evidence without spawning children.

## Acceptance

- Load skill-evaluation and define observable success/failure before runs. Keep fixtures bounded and isolated; native coordinator permissions and available tool contracts remain authoritative.
- Exercise two independent siblings and three siblings with cap two, recording launch/completion/wait order and live count. Check initial launch-before-wait and safe same-layer refill, not merely the presence of two sessions.
- Exercise or explicitly classify static/unrun cases for shared output conflicts, genuine dependencies, tiny edits, cap one, unavailable concurrency and unknown active writer. Do not fake overlap with sleeps or infer it from “parallel” in a report.
- Confirm no early later-layer launch, duplicate assignment, conflicting primary mutation, nested worker delegation, unreviewed acceptance or unsynchronized shared record writes. Check resulting artifacts as well as event ordering.
- Preserve evidence provenance: exact skill revision, raw requests, fixture inputs, tool-event summaries, outputs, observed results and limits. Typed report and listing index follow OKF; raw legacy fixtures remain outside the live bundle or serialized in JSON.
- Any baseline comparison uses equivalent capabilities and context. Label OpenCode runtime observations separately from Claude instruction review/simulation. No universal throughput/latency improvement claim.
- Return defects to T01/T02 owners rather than editing their files. Unavailable native concurrency evidence remains an explicit acceptance gap for T04, not a passing behavioral result.

## Execution

Dispatch/session: Not dispatched.
Checkout/branch/base commit: Not assigned.
Changed files: None.
Validation: None.
Review/integration: Not reviewed.
Blockers: None known; native execution capability checked when evaluation starts.

## History

Created during planning; no implementation started.
