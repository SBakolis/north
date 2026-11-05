---
type: Implementation Memory
title: Stronger parallel execution guidance
description: Completed launch-before-wait and same-layer refill guidance, with bounded OpenCode concurrency evidence and explicit runtime coverage limits.
tags: [parallel-execution, concurrency, subagents, execution, dependency-layers, skills]
sources:
  - id: execution
    resource: ../../assets/skills/north-execute/SKILL.md
  - id: delegation
    resource: ../../assets/skills/subagent-usage/SKILL.md
  - id: opencode-core
    resource: ../../assets/opencode/instructions/core.md
  - id: claude-core
    resource: ../../assets/claude/instructions/core.md
  - id: architecture
    resource: ../../docs/architecture.md
  - id: research
    resource: ../plans/parallel-execution/research.md
  - id: evaluation
    resource: ../evaluations/parallel-execution/report.md
  - id: evidence
    resource: ../evaluations/parallel-execution/evidence.json
  - id: acceptance
    resource: ../plans/parallel-execution/tasks/T04-verification.md
---
# Memory: Stronger parallel execution guidance

Feature: parallel-execution
Coverage: complete (T01–T04; bounded validation below)
Evidence checked against: `fff4c60` on `main`, clean tree before this save. Implementation commits `d32898f` and `9f5f7ca`; evaluation `ba1e662`; final acceptance `fff4c60`.
Plan: [Authoritative plan](../plans/parallel-execution/plan.md)

## Implemented behavior

- **T01:** `north-execute` explicitly launches compatible ready tasks before waiting for results, within the configured budget (default two). When a worker finishes, consider queued work from the same frozen layer after rechecking prerequisites, resource conflicts and pending integration. Essential coordinator work may take priority; no later layer starts until the whole current layer is accepted.[^execution]
- **T01:** `subagent-usage` owns standalone concurrency safety: prefer worthwhile independent work, allow shared immutable reads and useful non-conflicting primary work, and serialize write/read dependencies or shared mutable resources. Disjoint paths/worktrees alone do not establish independence. Tiny edits remain direct; genuine dependency chains remain serial.[^delegation]
- **T02:** Both host cores route the primary toward useful concurrency through the existing delegation skill. README, architecture and plan guide describe the same cap, dispatch/refill and barrier rules. Worker nesting permissions, installer/runtime code and plan schema were unchanged.[^opencode-core][^claude-core][^architecture]
- **T03–T04:** Native OpenCode assignments and static boundary analysis were retained, reviewed and accepted. All four tasks are integrated on source `main`; no feature worktrees were created.[^evaluation][^acceptance]

## Decisions and integration points

- Keep scheduling policy in its existing owners: pipeline layer lifecycle in `north-execute`, concurrency/isolation/integration in `subagent-usage`. Core files provide short routing cues. No new scheduler, skill, dependency, cross-layer scheduling or higher default cap was introduced.[^research]
- Use only available, permitted native concurrency mechanisms. Cap one or unavailable concurrency means serial execution; unknown active-writer state instead blocks redispatch until reconciled. Workers cannot delegate further. Primary acceptance, shared progress and Git integration remain serialized.[^delegation]
- Preserve schema-aware `task_status`, legacy authority and existing review-before-done behavior. Encouraging concurrency neither starts a pipeline phase nor grants extra permissions.[^execution][^claude-core]
- For future evaluations, distinguish outstanding assignments and completion-notification ordering from measured work intervals. Retain exact requests, outputs and guidance identities; do not manufacture overlap with delays or treat static scenarios as runtime enforcement tests.[^evaluation][^evidence]

## Validation

Reuse the execution-phase results below; this save checked current claims and references rather than rerunning implementation suites.

- `python3 -B north/evaluations/parallel-execution/replay.py`: passed, peak outstanding assignments **2**, refill before the other completion notification **true**. Synthetic controls rejected budget overflow, duplicate dispatch and premature acceptance.[^evaluation]
- Native A/B inspection intervals overlapped. C was dispatched after A's notification and before B's notification, but **B/C inspection did not overlap**. All three outputs were reviewed before acceptance.[^evaluation][^evidence]
- Twelve boundary scenarios were analyzed statically; 38 documentation references resolved. Primary JSON/YAML, 58 local links, task graph/state, source/hash/UTC, unchanged permissions/contract and whitespace checks passed.[^evaluation][^acceptance]
- `ruby north/evaluations/open-knowledge-format/validate-legacy-extension.rb`: passed. Independent final review found no blocking issues. Broad installer tests were not rerun for instruction-only changes.[^evaluation][^acceptance]

## Remaining work and limitations

No unfinished task IDs or implementation blockers remain. Evidence does not establish live Claude behavior, automatic skill routing, runtime negative-case enforcement, full child-session lifetimes, an independently exported host log, baseline improvement or speed/cost gains. The event trace is primary-recorded; timestamps delimit worker inspection. Retain these limits when reusing the evidence. North's artifact bundle remains mixed legacy/new; this save does not certify full bundle conformance.[^evaluation]

[^execution]: [Pipeline lifecycle and dispatch](../../assets/skills/north-execute/SKILL.md).
[^delegation]: [Delegation, concurrency safety and integration](../../assets/skills/subagent-usage/SKILL.md).
[^opencode-core]: [OpenCode routing cue](../../assets/opencode/instructions/core.md).
[^claude-core]: [Claude routing cue](../../assets/claude/instructions/core.md).
[^architecture]: [Architecture](../../docs/architecture.md); see also [plan guide](../../docs/plan-format.md) and [README](../../README.md).
[^research]: [Scope and design decisions](../plans/parallel-execution/research.md).
[^evaluation]: [Evaluation report and linked replay evidence](../evaluations/parallel-execution/report.md).
[^evidence]: [Retained raw outputs, fixture and guidance identities](../evaluations/parallel-execution/evidence.json).
[^acceptance]: [Final acceptance record](../plans/parallel-execution/tasks/T04-verification.md).
