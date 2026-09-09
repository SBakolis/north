---
name: north-plan
description: Turn a feature request into a linked Markdown dependency plan using requirements clarification, saved memory, and exploration. Use for /north-plan or creating and revising a North implementation plan.
---

# North plan

Plan the user's requested outcome without implementing the feature.

Load `north-sources` for the artifact root and read its
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
Create the index, research record, and task files defined by the plan contract.
Link existing requirements and research rather than copying them.

Split work at real dependency boundaries. Give each task a concrete outcome,
allowed write scope, and observable acceptance checks. Include dependencies
caused by reads of unfinished contracts, generated outputs, or shared resources.
Keep task-local tests with their implementation when practical; do not add tasks
just to create parallel activity. Include a final integration verification task
depending on every implementation branch.

Validate the graph and compute its layers using the contract. New tasks start
`pending`, or `blocked` with the consequential decision or missing evidence.
A delegated `north-planner` returns proposed contents for the primary to save.
Record any project-required synchronization with external task artifacts.

## Handoff

Summarize the planned outcome, chosen approach, layers, and blocked decisions.
Link the saved index and suggest `/north-execute <feature>` after those decisions
are resolved. For read-only planning, return proposed contents and paths and
state that saving the plan is still needed before execution.
