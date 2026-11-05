---
type: North Plan
north_schema: north-plan-v1
title: Encourage parallel execution
description: Make concurrent execution the explicit preference for independent worthwhile work while retaining North's existing execution limits.
sources:
  - id: execution
    resource: ../../../assets/skills/north-execute/SKILL.md
  - id: delegation
    resource: ../../../assets/skills/subagent-usage/SKILL.md
  - id: prior-memory
    resource: ../../memories/open-knowledge-format.md
---
# Plan: Encourage parallel execution

Feature: parallel-execution
Goal: North execution guidance more consistently launches compatible independent work concurrently, rather than serializing it by habit.
Acceptance: Eligible siblings are launched before waiting for one; free slots can admit compatible queued work in the current layer; dependent/conflicting work and primary-owned mutations remain ordered.
Max parallel: 2
Research: [Findings and decisions](research.md)
Context: [Execution skill](../../../assets/skills/north-execute/SKILL.md), [delegation skill](../../../assets/skills/subagent-usage/SKILL.md), [OKF implementation memory](../../memories/open-knowledge-format.md).

## Requirements and scope

The user's request is to encourage parallel execution “a bit more” in execution-oriented skills. Treat this as a stronger operational preference within the existing model, rather than a concurrency-limit or scheduling-architecture change. No product question blocks this bounded interpretation.

1. Put the detailed preference in `subagent-usage`, with pipeline-specific dispatch/refill behavior in `north-execute`. These already own concurrency and dependency-layer decisions.[^delegation][^execution]
2. Before substantial execution, identify ready independent work; dispatch compatible worthwhile assignments together through available native tools, up to the configured limit, before awaiting the first result. Do not create trivial tasks just to fill slots.
3. When an active worker finishes, consider compatible queued tasks in the same frozen layer for the available slot. Respect accepted prerequisites, resource conflicts and serialized primary progress/integration. No later layer starts before the current layer is accepted.
4. Where supported, the primary can perform useful non-conflicting work while agents run (for example, inspect an unrelated stable area or prepare review criteria). It must not duplicate a worker's assignment, inspect changing outputs as stable evidence, or consume unaccepted results.
5. Distinguish independent tool calls from delegated work. Independent reads/checks may run together when their tools/resources permit; shared immutable reads alone are not a write conflict. Checks that mutate the same outputs or service, broad formatters and Git integration remain serialized as necessary.
6. Preserve native tool availability and permissions. Use supported concurrent invocation/background mechanisms; do not assume provider-specific API flags or polling. If concurrency is unavailable or unsuitable, execute serially with a concise concrete reason when relevant—no new approval gate or mandatory prose for every small action.
7. Align both tool core routing and documentation with this preference while keeping detailed rules in the two owning skills.

Preserved constraints: default `Max parallel: 2`; explicit lower/higher limits; dependency barriers; review before acceptance; legacy/new plan semantics; primary-only shared records and Git operations; worker prohibition on further delegation; scope and worktree isolation rules.[^prior-memory]

Non-goals: raising the default limit, permitting nested worker delegation, introducing a scheduler or new runtime, switching to agent teams/separate tool processes, expanding tool permissions, broadly rewriting all method skills, or benchmarking universal speed/cost gains.

## Acceptance examples

- Two substantial independent tasks in one ready layer and limit two: both launch before the primary awaits either result, with distinct scopes and recorded sessions.
- Three compatible tasks in one layer and limit two: at most two workers are active; after one completes, the third may start without waiting for the other, subject to current conflicts and coordinator work. Later layers remain closed.
- Two tasks touching the same shared generated output or a write/read dependency: isolate if that actually removes the conflict, otherwise serialize; shared read-only source access is not itself a reason to serialize.
- A tiny single-file edit or a genuine dependency chain: remain direct/serial; do not invent agents, dependency-free tasks or unnecessary worktrees.
- Limit one, unavailable native concurrency, or unknown active-writer state: honor the limit/capability/recovery rule and report the relevant restriction; do not spawn duplicates.
- Standalone substantial execution can use the delegation preference without implicitly invoking the North pipeline. Workers still cannot spawn workers.
- Primary acceptance/progress updates and merges are serialized even when disjoint workers or read-only reviewers run concurrently. A failed task prevents crossing the layer barrier; compatible siblings may finish.

## Tasks

| ID | Task | Depends on | Layer | Status |
| --- | --- | --- | --- | --- |
| T01 | [Strengthen execution/delegation guidance](tasks/T01-execution-guidance.md) | — | 1 | pending |
| T02 | [Align tool instructions and docs](tasks/T02-integration-surfaces.md) | T01 | 2 | pending |
| T03 | [Evaluate concurrent execution behavior](tasks/T03-behavioral-evaluation.md) | T01, T02 | 3 | pending |
| T04 | [Verify integrated outcome](tasks/T04-verification.md) | T01, T02, T03 | 4 | pending |

This change is intentionally a dependency chain: the integration wording consumes the revised skill contract, and evaluations consume the combined instructions. Artificial parallel tasks would contradict the goal of useful independence.

## Decisions and unresolved questions

- Selected approach: strengthen observable dispatch behavior rather than adding more general “prefer parallel” wording.
- Preserve existing default limits and barriers; no separate user decision is needed for this scoped wording change.
- No new skill/dependency is planned. If inspection identifies a contradictory caller outside the allowed scopes, report it before expanding the task.
- No unresolved product blocker. Native concurrency evidence may depend on execution-time tool availability. If it cannot be observed, record the evaluation as limited rather than claim that behavior passed.

## Execution and handoff

- Working project: `/Users/stamatis/Documents/projects/north`.
- Resolved artifact root: `/Users/stamatis/Documents/projects/north/north`.
- Source branch/base: `main` / `e0f3244`; working tree clean at planning start.
- Prior knowledge: [OKF adoption memory](../../memories/open-knowledge-format.md), checked against current owners/contracts. New records use the OKF plan layout; the earlier completed plan remains legacy.
- No matching prior plan, generated preference skills, active OpenSpec changes or main OpenSpec requirements were found. No external task synchronization required.
- Active layer: none. No implementation, delegation sessions or worktrees started for this feature.
- Planning validation: Ruby Psych parsed concept/task metadata and source records; four unique tasks form an acyclic four-layer graph, task table states/layers agree, and 43 local Markdown links resolve. An initial validation-script table-cell indexing error was corrected; the plan required no graph change. Behavioral checks belong to T03, not this planning phase.
- Next command: `/north-execute parallel-execution` (suggested only).

[^execution]: [north-execute](../../../assets/skills/north-execute/SKILL.md), current layer dispatch and review lifecycle.
[^delegation]: [subagent-usage](../../../assets/skills/subagent-usage/SKILL.md), concurrency, isolation and ownership rules.
[^prior-memory]: [OKF implementation memory](../../memories/open-knowledge-format.md), schema/legacy compatibility and observed validation limits.
