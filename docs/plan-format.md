# Execution plans

North coordinates implementation through a Markdown dependency graph interpreted
by the primary agent. There is no North runtime scheduler. `/north-plan <prompt>`
uses requirement clarification, saved implementation memory, and exploration to
save a plan under the working project's North directory. `/north-execute` then
implements it, and `/north-save` preserves verified implementation knowledge.

## Files and ownership

New plans use this layout, honoring an explicitly configured North directory:

```text
north/plans/<feature>/
  index.md
  research.md
  tasks/
    T01-contract.md
    T02-endpoint.md
    T03-form.md
    T04-integration.md
```

The [bundled plan format](../assets/skills/north-plan/references/plan-format.md)
contains the maintained index/task templates, statuses, and resume procedure.
It is shipped with `north-plan` so installed agents can read it.

Each task file owns its ID, status, dependencies, agent, write scope, acceptance
checks, and evidence. The index links every task and mirrors statuses,
dependencies, and computed layers. Task files link back to the index and their
prerequisites. Research records sources, alternatives, decisions, and evidence
gaps, with a link back to the index. Only the primary agent edits shared records;
the planner, workers, and verifier return proposed updates and evidence.

Use stable task IDs and unique feature names. Preserve relevant OpenSpec or other
authoritative project requirements and link to them. Existing single-file plans
at `north/plans/<feature>.md` can resume without migration or lost history. A
read-only planning request returns proposed contents without creating files.
Ordinary small edits do not require the pipeline.

## Dependency layers

Validate unique IDs, existing dependencies, acyclicity, and compatible scopes
before dispatch. Roots form layer 1; each other task belongs one layer after
its latest prerequisite. Freeze the active layer before starting it. With
`T01 → {T02, T03} → T04`, finish and verify T01, run T02 and T03 concurrently,
then finish and verify both before starting T04.

Dispatch only pending tasks whose prerequisites are done and present in their
checkout. Use concurrent native Task calls within the plan's limit (default 2),
queuing excess or conflicting work. Worktrees can isolate files, but shared
contracts, generated outputs, lockfiles, and external resources still matter.
Serialize conflicting work, plan updates, and integration. Without concurrent
tools, follow the same dependency order serially and report that limitation.

Tasks move from `pending` to `running` to `needs-review`, then to `done` only
after primary review, acceptance verification, and integration. A failure or
uncertain session becomes `blocked` with diagnostics and a next action. A
blocker holds the layer boundary while independent work in the active layer
can finish. Resolve it before returning the task to `pending`; preserve failure
evidence. Do not repeatedly retry the same unresolved failure.

Record actual dispatch/session references, changed files, validation commands
and results, review findings, and integration state. The verifier is read-only;
the primary executes missing checks. Run final integration checks on the stable
combined tree before reporting completion. Revalidate and recompute layers after
graph changes; changed prerequisites can invalidate downstream completion.

## Resume and memory

Reconcile plan files with current code, validation evidence, native sessions,
and worktrees before resuming. A running status alone does not prove that a
worker is active or finished. Establish that the prior writer has stopped before
retrying; otherwise record the uncertainty as blocked. Review partial changes
and preserve user edits. Retain plan paths and unfinished IDs in handoffs.

After implementation, `/north-save <feature>` writes
`north/memories/<feature>.md`, updates `north/memories/index.md`, and links the
memory and plan to each other. It records verified behavior, decisions, code
references, validation, and limitations. Partial work stays explicitly partial;
saving memory does not complete tasks or remove plans. `invoke-memory` reads
relevant records before later implementation and checks their applicability.
