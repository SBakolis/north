---
type: Skill Evaluation
title: Parallel execution guidance evaluation
description: Native OpenCode dispatch evidence and static boundary analysis with explicit timing and coverage limits.
sources:
  - id: evidence
    resource: evidence.json
  - id: events
    resource: events.json
  - id: criteria
    resource: criteria.json
---
# Parallel execution evaluation

Plan: [Parallel execution](../../plans/parallel-execution/plan.md).
Evaluated implementation: `9f5f7ca` on `main` (skills introduced by `d32898f`).
Environment: native OpenCode background subagents on macOS; primary coordinator,
two-assignment budget, immutable repository inputs and separate temporary JSON outputs.

## Observed results

- A/B were launched before waiting for either result. Their recorded inspection
  intervals overlap: A `21:13:48.600950–21:14:02.760693Z`, B
  `21:13:52–21:14:53Z`, on 2026-11-05.[^evidence]
- C was queued until A's completion notification, then dispatched before B's
  completion notification. Peak outstanding assignments: two. Each assignment
  was dispatched once and its output reviewed before acceptance.[^events]
- C's inspection interval was `21:15:03–21:15:24Z`: **B/C inspected work did
  not overlap**. The run demonstrates refill decision/notification ordering,
  not overlapping B/C work or a measured full session lifetime.[^evidence]
- The primary checked plan state/graph and replayed the existing legacy-extension
  regression while assignments were outstanding, without writing worker inputs
  or outputs. No later plan layer was dispatched during this evaluation.[^events]

## Static and replay checks

A inspected unchanged worker/verifier permissions on both hosts. B analyzed all
twelve preregistered boundary scenarios: sibling dispatch, later-layer barrier,
shared mutable resource, immutable shared reads, dependency chain, tiny edit,
cap one, unavailable concurrency, unknown writer, failed sibling, stable reviewers,
and nested delegation. No contradictory instruction was found. These are static
decisions, not runtime fault or permission-enforcement tests. C found all 38 local
documentation references resolve and the concurrency descriptions agree.[^evidence]

Primary checks: source preservation against Git, plan metadata/graph consistency,
JSON/YAML and local links, `git diff --check`, and
`ruby north/evaluations/open-knowledge-format/validate-legacy-extension.rb` passed.
Run `python3 -B north/evaluations/parallel-execution/replay.py` to replay the retained
trace. Synthetic controls separately accepted a valid trace and rejected budget
overflow, duplicate dispatch, and premature acceptance.

## Evidence and limits

[Criteria](criteria.json) were recorded before dispatch. [Requests](requests.json)
retain the actual prompts; [evidence](evidence.json) retains fixture input, complete
worker reports, and exact evaluated skill/core contents with hashes. The
[event trace](events.json) is a primary-recorded summary of native calls and
notifications, not an independently exported host log. Worker timestamps delimit
inspection, not necessarily complete child-session lifetimes.

The initial native concurrency case passed. Safe refill was exercised at the
notification/dispatch boundary, with interval overlap explicitly limited as above,
consistent with the preregistered no-artificial-waits rule.[^criteria]
No baseline comparison, causal improvement, speed/cost gain, automatic routing,
live Claude execution, destructive conflict case, or unavailable-host runtime case
is claimed. Installer/runtime code was unchanged; broad installer suites were not
rerun for instruction-only changes. These limits bound acceptance rather than
turning static cases into claimed behavioral passes.

[^evidence]: [Archived inputs, outputs and evaluated guidance](evidence.json).
[^events]: [Native event summary](events.json).
[^criteria]: [Preregistered criteria and timing rule](criteria.json).
