---
name: north-plan
description: Turn a feature request into a linked Markdown dependency plan using requirements clarification, saved memory, and exploration. Use for /north-plan or creating and revising a North implementation plan.
---

# North plan

Plan the user's requested outcome without implementing the feature.

Load `north-sources` for the artifact root and read its
[OKF profile](../north-sources/references/okf-format.md) and
[plan contract](../north-sources/references/plan-format.md). Load `invoke-memory`
for relevant prior implementation knowledge, then `clarify-requirements` for
the agreed behavior, acceptance examples, and consequential unanswered questions.
Use existing project instructions and OpenSpec requirements as context.

Load `north-explore` to compare implementation approaches. Use its recommendation,
constraints, and evidence gaps to shape the work. Record unresolved decisions
against the tasks they block; this phase can save a partial plan without
inventing the user's answer.

## Build the feature plan

Choose a short lowercase hyphenated feature slug. Reuse the same feature's
existing plan; use a distinct slug when a name belongs to unrelated work.
For a new feature, create the navigation-only `index.md`, detailed `plan.md`,
research record, and task files defined by the plan contract. Use `type: North Plan`
and `type: North Task` with `north_schema: north-plan-v1` on plan/tasks, and
`type: Research` on research. The overview owns the task table; the index lists
the overview, research, and actual tasks (or their directory listing).
Link existing requirements and research rather than copying them.

For existing plans, select the actual authority using the contract, including
legacy detailed indexes and single-file plans. Revisions do not implicitly
convert them. Preserve unknown metadata, provenance, history, and incoming links;
explicit conversion follows the contract. Apply the profile's reserved-name,
evidence, and meaningful-update rules without inventing verification.

Split work at real dependency boundaries. Give each task a concrete outcome,
allowed write scope, and observable acceptance checks. Include dependencies
caused by reads of unfinished contracts, generated outputs, or shared resources.
Keep task-local tests with their implementation when practical; do not add tasks
just to create parallel activity. Include a final integration verification task
depending on every implementation branch.

Validate the graph and compute its layers using the contract. New tasks start
with `task_status: pending`, or `task_status: blocked` with the consequential
decision or missing evidence. OKF `status` is lifecycle, not task progress.
A delegated `north-planner` returns proposed contents for the primary to save.
Record any project-required synchronization with external task artifacts.

## Handoff

Summarize the planned outcome, chosen approach, layers, and blocked decisions.
Link the authoritative overview (new `plan.md`, legacy index or single file),
optionally its navigation index, and suggest `/north-execute <feature>` after those decisions
are resolved. For read-only planning, return proposed contents and paths and
state that saving the plan is still needed before execution.
