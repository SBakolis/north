---
name: north-execute
description: Progress a saved North implementation plan through dependency layers, reviewing each layer before advancing. Use for /north-execute or resuming a North feature implementation.
---

# North execute

Own the pipeline lifecycle: select a plan, progress its tasks, verify the
integrated result, and hand off to saving memory.

## Prepare

Load `north-sources` and follow its
[plan contract](../north-sources/references/plan-format.md) for selection,
validation, statuses, and recovery. Without a selector or conversation context,
look for a sole unfinished plan. If no plan exists, suggest
`/north-plan <feature prompt>`.

Read the selected plan's task files, research, requirements, and current project
instructions. Load `invoke-memory` before implementation and `subagent-usage`
for worker assignments, concurrency safety, checkout isolation, and integration.
Reconcile the plan with actual execution using the contract before dispatch.

## Progress dependency layers

Start at the first unfinished layer computed by the contract. Freeze its
membership before launching work. A completed task does not unlock a later
layer while another task in the active layer remains unfinished.

Within that layer:

1. Select `pending` tasks with accepted prerequisites present in their assigned
   checkout. Apply `subagent-usage` to dispatch compatible tasks concurrently
   within `Max parallel`, queuing excess or conflicting work. Record each launch
   and set its task to `running` under the contract.
2. Returned work becomes `needs-review`. Review the changes and acceptance
   evidence, execute missing checks, and resolve review findings. Use
   `subagent-usage` to integrate accepted changes; only then set the task to
   `done` and synchronize the index and any required external progress record.
3. Set failures to `blocked` with diagnostics and a next action. Other tasks in
   this layer may finish. Resolve the blocker before returning its task to
   `pending` for a scoped repair; do not repeat an unchanged failed attempt.
4. Advance only after every task in the active layer is `done`. Reconcile and
   recompute the remaining graph after a revision before dispatching more work.

Keep progress updates serialized. If native parallel execution is unavailable,
run the same order serially and disclose the limitation. Worktree integration
must honor the installed commit mode. If a required commit is not authorized,
use a compatible shared checkout where practical or report the integration
blocker; do not silently change commit mode.

## Verify and hand off

Complete the planned final integration checks on a stable combined tree.
The read-only verifier reviews supplied evidence; the primary executes missing
checks. Apply `subagent-usage` for source-branch delivery when worktrees were used.

Report completion only when all required tasks meet the contract's `done`
conditions. Record the final evidence and limitations, link the plan, and suggest
`/north-save <feature>`. If blocked, report unfinished IDs and the next resume
action; final memory follows completion. An explicitly requested early
`/north-save` may record a verified subset as partial memory.
Do not start another phase automatically.
