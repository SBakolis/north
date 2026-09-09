---
name: north-plan
description: Plan a requested feature as linked Markdown task files with statuses, dependencies, and acceptance checks in a feature directory under north/plans/. Use for /north-plan and requests to create or revise a North implementation plan.
---

# North plan

Turn the user's prompt and established context into an executable dependency
graph. This phase writes planning artifacts; it does not implement the feature.

## Establish scope and evidence

Resolve the working project's root and configured North output directory,
defaulting to `<project-root>/north/`, even when invoked in a subdirectory or
when the project itself is named `north`. Read project instructions and relevant
OpenSpec artifacts. Keep authoritative requirements in their existing location
and link to them. Use the same resolved North path throughout the pipeline.

Load `invoke-memory` before planning, then load `clarify-requirements` to resolve
consequential gaps in the user's requested behavior. Reuse settled answers;
inspect repository facts yourself and choose routine details from conventions.
Ask the user only about decisions that materially change scope, behavior, or
acceptance. Continue independent planning while an essential answer is pending;
record the blocked tasks rather than inventing the answer.

Load `north-explore` to investigate similar implementations, best practices,
and useful libraries for this feature. Use its findings to select an approach
and define dependencies and acceptance checks. If required skills are missing,
identify them and explain how to enable **North pipeline** in North's installer,
which includes all pipeline skills and dependencies, before continuing that phase.

## Persist the graph

Read [the plan format](references/plan-format.md) before writing. Use a short
lowercase hyphenated feature slug (letters and digits, without path separators).
Reuse the existing directory for the same feature. If a name already represents
unrelated work, choose a distinct descriptive slug rather than replacing it.
Create directories only when saving, and report conflicting files or symlinks
instead of replacing them or writing through them.

Save `north/plans/<feature>/index.md`, `research.md`, and one Markdown file per
task under `tasks/`. Task files own their dependencies, status, scope, acceptance,
and evidence. The index links every task and mirrors its status and dependencies
for navigation. Tasks link back to the index and to their prerequisites; use
relative Markdown links that resolve from each file. Only the primary agent
writes shared plan files. A delegated `north-planner` returns proposed content.

Split work at real dependency boundaries with disjoint ownership where possible.
Each task needs a stable ID, explicit dependencies (empty for roots), allowed
write scope, agent, observable acceptance checks, and initial `pending` status
(or `blocked` with a precise reason). Include a final integration verification
task depending on all implementation branches. A task can contain its own tests;
do not add artificial tasks just to increase parallelism.

Check all IDs are unique, every dependency and Markdown link resolves, no task
depends on itself, and the graph has no cycles. Include dependencies caused by
reads of unfinished contracts, generated files, and shared resources. Compute
topological layers: roots are layer 1; other tasks are one layer after their
latest prerequisite. Record a concurrency limit (default 2). Serialize tasks
with conflicting scopes even when their dependency layer is the same.

For a previously started single-file North plan, preserve its location, stable
IDs, and evidence on resume. Do not create a competing directory plan. New
plans use the directory format. Link external requirements and task artifacts;
North's task files own execution status. If project instructions require an
external progress record too, update it from verified North outcomes and record
that synchronization in the plan.

## Handoff

Summarize the planned outcome, approach, layers, and any blocked decisions. Link
the saved index and suggest `/north-execute <feature>` as the next command,
after any blocking decisions are resolved. Do not start implementation as part
of `/north-plan`. A read-only planning request returns proposed file contents
and paths without saving and states that persistence is still needed.
