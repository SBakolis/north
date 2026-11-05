---
type: Research
title: Stronger parallel execution guidance
description: Identify precise dispatch encouragement points without changing North's concurrency limits or ownership model.
sources:
  - id: execution
    resource: ../../../assets/skills/north-execute/SKILL.md
  - id: delegation
    resource: ../../../assets/skills/subagent-usage/SKILL.md
  - id: core
    resource: ../../../assets/opencode/instructions/core.md
  - id: worker
    resource: ../../../assets/opencode/agents/north-worker.md
  - id: architecture
    resource: ../../../docs/architecture.md
  - id: evaluation
    resource: ../../../assets/skills/skill-evaluation/SKILL.md
  - id: memory
    resource: ../../memories/open-knowledge-format.md
  - id: claude-subagents
    resource: https://code.claude.com/docs/en/sub-agents
---
# Research: Parallel execution encouragement

Plan: [Parallel execution](plan.md)
Repository baseline: `e0f3244` on `main`. External documentation retrieved 2026-11-05.

## Finding

North already favors parallel execution. The useful change is to make the choice actionable: find ready independent work, launch compatible assignments before waiting, refill available current-layer slots, and use independent primary work while agents run. Preserve the established cap and layer barrier rather than raising concurrency or changing the scheduler model.[^execution][^delegation]

This is a requested behavioral preference, not a demonstrated runtime defect. The preceding OKF implementation used two concurrent workers for independent skill groups, demonstrating that the existing workflow can do this; that history is not evidence that the current wording always serializes or that revised wording will be faster. No new evaluation was run during planning.

## Repository investigation and impact

| Surface | Current evidence | Planned treatment |
| --- | --- | --- |
| `subagent-usage` | Lines 8–15 prefer subagents for meaningful concurrent progress; lines 29–33 say use concurrent calls but emphasize conflicts; ownership/merges are explicitly serialized | Primary update: actionable default for suitable independent work, launch-before-wait guidance, useful primary overlap, concrete reasons to remain serial |
| `north-execute` | Lines 35–54 freeze layers, dispatch compatible tasks concurrently and queue excess; no explicit same-layer slot-refill example | Primary update: make initial concurrent dispatch and safe current-layer refill explicit, preserve barriers and failure/review handling |
| Both tool core instructions | Route to delegation skills but do not state a proactive concurrency preference | Short consistent routing cue, not a duplicate scheduling contract |
| README and architecture/plan guide | Describe concurrent workers and default two; architecture explicitly says North has no executable scheduler | Keep user-facing explanation consistent with clarified behavior |
| Tool worker agents | OpenCode `task: deny`, Claude `disallowedTools: Agent`; both prohibit further delegation | Preserve unchanged. Encouragement applies to the authorized primary coordinator, not nested workers |
| `north-plan` and plan contract | Already split at real dependency boundaries, default cap two, no task splitting just for parallel activity | No change required for this execution-focused request; use as compatibility checks |
| `skill-evaluation` | Uses isolated fixtures, observable behavior and accurate distinction between static/actual runs | Reuse methodology for T03; no generic concurrency rewrite needed |
| `diagnosing-bugs`, `test-design`, `code-review`, `prototype`, `research`, `north-explore` | Method skills, not scheduler owners; may be used by non-delegating workers/read-only verifiers | Avoid blanket “spawn agents” rules; the coordinator can delegate compatible units through the shared owner |
| `commit`, `auto-commit`, `north-save`, `dry-skillify` | Own Git mutations or consolidated records | Preserve serialized ownership; encouraging simultaneous writers here would conflict with established behavior |

The two core skills and both core instructions agree on primary ownership, native delegation and conflict handling.[^core][^worker] Read-only sharing is not equivalent to conflicting reads of unfinished writes; new guidance should name the dependency or shared mutable resource rather than imply any common read is unsafe.[^delegation]

The saved [OKF memory](../../memories/open-knowledge-format.md) remains applicable: preserve `task_status`, schema-aware legacy handling, primary plan updates and typed evaluation artifacts. This feature does not need a schema change or migration.[^memory]

## External evidence and applicability

Claude Code's official subagent documentation distinguishes blocking foreground calls from background agents that run while the primary continues, and recommends simultaneous agents for investigations that do not depend on each other. It also notes that subagents consume usage and have independent permissions.[^claude-subagents] These support selective, bounded overlap rather than delegation for every small operation.

The retrieved page includes evolving tool/version-specific background and nested-agent features. North already uses native tools on two hosts and deliberately denies worker delegation. Do not copy newer runtime defaults, flags, nested-agent capabilities, agent teams, or separate-session tooling into North's shared policy. Check the actual available tools at execution time. External documentation is conceptual support, not evidence of installed runtime behavior or measured speedup.

## Approaches considered

1. **Selected: stronger dispatch instructions in existing owners.** Small change, clear behavioral acceptance, no runtime or new dependency. Both tools get the shared skills.
2. **Increase default Max parallel.** Could increase resource use and conflicts; does not fix serial awaiting. Not requested by “a bit more”; retain two.
3. **Relax the layer barrier to a continuously ready DAG scheduler.** More aggressive utilization, but materially changes lifecycle/recovery and existing acceptance. Outside scope.
4. **Add a parallelism skill or scheduler library.** Duplicates current owners and creates routing/installation work; unnecessary for instruction-based coordination.[^architecture]
5. **Add concurrency instructions to every execution-adjacent skill or enable nested workers.** Risks permission conflicts and needless fragmentation. Keep orchestration in its owners and existing role boundaries.

No library or dependency installation is needed. Existing native tools, scoped worktrees and task evidence are the reusable mechanisms.

## Evaluation design

Use the existing skill-evaluation approach: success is actual tool dispatch/return ordering and preserved results, not counts of the word “parallel.”[^evaluation]

- Predefine a small matrix: two independent siblings; three siblings with cap two; shared mutable output; chain/tiny edit; limit one; unavailable concurrency or interrupted writer.
- Capture native event/call ordering where available. Both independent launches must precede the first wait/return barrier in a positive case. For refill, distinguish eligibility from mandatory immediate launch when the primary has essential review/conflict work.
- Prove maximum live worker count stays within the limit, no later-layer launch occurs early, and tasks sharing a mutable resource are not overlapped unsafely. Shared immutable reads are allowed.
- Keep artifact outcomes and acceptance evidence part of evaluation; parallel dispatch that produces conflicting/unreviewed outputs is not success.
- The primary coordinates native concurrency cases. A `north-worker` or `north-verifier` evaluation assignment must not gain nested delegation privileges. If an isolated coordinator is used, its role/permissions must explicitly allow the native tools already supported by the host; do not launch external CLI sessions to work around a restriction.
- A baseline comparison is useful only with equivalent fixtures/permissions and meaningful event evidence. Do not claim universal speed/cost improvements, automatic routing, or live Claude coverage from simulated instructions or OpenCode-only runs.
- Retain replayable inputs, outputs and event summaries as structured evidence under the configured North evaluation directory. Raw legacy/non-OKF fixture Markdown belongs in isolated fixtures or JSON archives, not as untyped concepts in the real bundle.

## Gaps and decisions

No unresolved product decision blocks execution. Execution-time native tool/event availability determines which concurrency cases can be observed; unavailable cases must be recorded as unrun. The user has not requested a higher concurrency cap or cross-layer scheduling, so those are excluded rather than silently assumed.

[^execution]: [Execution lifecycle](../../../assets/skills/north-execute/SKILL.md).
[^delegation]: [Delegation and integration](../../../assets/skills/subagent-usage/SKILL.md).
[^core]: [OpenCode core](../../../assets/opencode/instructions/core.md); [Claude core](../../../assets/claude/instructions/core.md) follows the same ownership policy.
[^worker]: [OpenCode worker](../../../assets/opencode/agents/north-worker.md); [Claude worker](../../../assets/claude/agents/north-worker.md) also prohibits further delegation.
[^architecture]: [North architecture](../../../docs/architecture.md), instruction-driven coordination and lack of automatic scheduler.
[^evaluation]: [Skill evaluation method](../../../assets/skills/skill-evaluation/SKILL.md).
[^memory]: [OKF implementation memory](../../memories/open-knowledge-format.md), checked against the current contracts.
[^claude-subagents]: [Official Claude Code subagent documentation](https://code.claude.com/docs/en/sub-agents#run-subagents-in-foreground-or-background), plus its “Run parallel research” section; retrieved 2026-11-05.
