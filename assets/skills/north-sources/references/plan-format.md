# North plan contract

Use the artifact root resolved by `north-sources` and the
[OKF v0.2 profile](okf-format.md). New feature plans use:

```text
north/plans/account-settings/
  index.md
  plan.md
  research.md
  tasks/
    T01-contract.md
    T02-endpoint.md
    T03-form.md
    T04-integration.md
```

The primary agent is the only editor of the shared plan. Each task file's
frontmatter is authoritative for task metadata and status; its body holds
acceptance and execution evidence. `plan.md` owns the overview and synchronized
task table; `index.md` is only navigation (see the profile's listing example).
Reconcile a stale table against task files and actual execution before dispatch.
These Markdown conventions guide an agent; there is no automatic scheduler.

## Plan example (`plan.md`)

```markdown
---
type: North Plan
north_schema: north-plan-v1
title: Account settings
description: Plan for viewing and updating a display name.
---
# Plan: Account settings

Feature: account-settings
Goal: Users can view and update their display name.
Context: Link relevant requirements, code, and applicable saved memory here.
Acceptance: A valid name persists; invalid input shows an error without changing it.
Non-goals: Account deletion and email changes.
Max parallel: 2
Research: [Findings and decisions](research.md)

## Tasks

| ID | Task | Depends on | Layer | Status |
| --- | --- | --- | --- | --- |
| T01 | [Define contract](tasks/T01-contract.md) | — | 1 | pending |
| T02 | [Implement endpoint](tasks/T02-endpoint.md) | T01 | 2 | pending |
| T03 | [Implement form](tasks/T03-form.md) | T01 | 2 | pending |
| T04 | [Verify integration](tasks/T04-integration.md) | T02, T03 | 3 | pending |

## Decisions and open questions

Record settled requirements, assumptions, and unresolved decisions with affected IDs.

## Execution and handoff

Record the project root, resolved North path, source branch and starting commit
when applicable, existing local changes, active layer, session/worktree references,
and integration results. Keep revisions, retry reasons, and unfinished IDs here.
Link saved implementation memory after /north-save.
```

## Task example

Use one file per task. `depends_on` holds IDs, not titles or unchecked prose.
`write_scope` uses project-relative paths; an empty list means read-only.

```markdown
---
type: North Task
north_schema: north-plan-v1
id: T02
task_status: pending
depends_on: [T01]
agent: north-worker
write_scope: [src/server/settings/, tests/server/settings/]
---

# T02: Implement settings endpoint

Plan: [Account settings](../plan.md)
Prerequisites: [T01: Contract](T01-contract.md)
Research: [Findings](../research.md)

## Outcome

Persist the authenticated user's display name using the agreed contract.

## Acceptance

- A valid update is returned by a subsequent read.
- An invalid name returns the agreed validation error and leaves stored data unchanged.
- Run the project's focused endpoint checks; record exact commands and outcomes.

## Execution

Dispatch/session: Not dispatched.
Checkout/branch/base commit: Not assigned; record if using worktrees.
Changed files: None yet.
Validation: None yet.
Review/integration: Not reviewed or integrated.
Blockers: None known.

## History

Record dispatch, outcomes, repairs, and evidence invalidated by later changes.
```

Replace example paths with actual project paths. Root tasks use `depends_on: []`
and `Prerequisites: None`. A final review task can use `north-verifier` and
`write_scope: []`; the primary runs checks the read-only verifier cannot run.
Do not mark a task complete based only on its worker's report.

`research.md` uses `type: Research`, links back to `plan.md`, and records the question, repository and
external evidence, alternatives considered, selected approach, applicability,
and unresolved gaps. It is context, not an executable task. Link to relevant
research already stored elsewhere instead of duplicating it.

## Graph validation

Task IDs are stable and unique. Every dependency names an existing task, required
execution references and local evidence claimed as existing resolve, and the graph
has no cycles or self-dependencies. Dangling optional knowledge links are tolerated
by OKF; they do not waive missing North execution prerequisites.
Write scopes and acceptance checks must describe real project paths and
observable outcomes. Include semantic dependencies such as reads of unfinished
contracts, generated outputs, lockfiles, and shared test resources.

Use a positive `Max parallel` value, defaulting to 2. Compute the plan table's layers
from dependencies: roots are layer 1; each other task is one layer after its
latest prerequisite. Recompute after graph revisions. The execution workflow
decides when a layer may start; `north-execute` owns pipeline layer barriers.

## Status and evidence

New plan/task records require `north_schema: north-plan-v1`. This discriminator
selects the North execution contract independently of OKF version and `type`.
Tasks require a non-empty `id`, `task_status` from the table below, `depends_on`
as an ID list, a named `agent`, and `write_scope` as a project-relative path list
(empty means read-only), plus actionable outcome, acceptance, and evidence sections.
`type: North Task` alone is not permission to execute an incomplete record.
Unknown schemas remain readable but require clarification before dispatch.

For this schema, `status` is optional OKF lifecycle (`draft`, `stable`,
`deprecated`), never execution progress. Omit it unless useful; its default
`stable` implies no execution completion or verification. Preserve lifecycle,
provenance, and unknown extension fields while updating `task_status`.

| Task status | Meaning |
| --- | --- |
| pending | Not dispatched, possibly waiting for prerequisites. |
| running | Dispatch recorded; execution is being launched or is active. |
| needs-review | Work returned; acceptance and integration still need review. |
| done | Primary reviewed the result and acceptance evidence; accepted changes are available to dependents. |
| blocked | Failure, unresolved decision, or uncertain execution requires resolution. |

Record dispatch before launch, adding the native session reference when available.
Keep changed files, exact checks and results, missing evidence, review findings,
and integration state in the task's execution section. A worker report or status
label alone is not completion evidence. Synchronize plan table entries from task files;
preserve previous outcomes and reasons for repairs in the history.

If project instructions also require an external progress artifact such as
OpenSpec tasks, record its location and synchronize it from verified outcomes.

## Select and resume a plan

An explicit selector may be a feature name, plan directory, index or plan path, or legacy
single-file plan. Otherwise use an unambiguous plan from the conversation, or a
sole plan matching the requested phase. Ask when several match; do not choose
by modification time. If none exists, identify the missing plan.

Select authority by layout and metadata, not by filename alone:

| Evidence | Authority and progress field |
| --- | --- |
| New directory: `plan.md` declares `north-plan-v1`; task records declare it too | `plan.md` overview; tasks own `task_status`; `index.md` only lists files. |
| Legacy directory: `index.md` is the existing detailed plan with task table and links to legacy task records | That index remains overview; positively identified legacy tasks own execution `status`. |
| Legacy single-file plan | Its task table owns metadata/execution status; detail sections own evidence. |
| Navigation index only | Follow its plan link; missing overview/tasks block execution, not reading. |
| Competing single-file/directory or two detailed overviews | Ask for authoritative selection unless an explicit selector or recorded conversion resolves it. Do not merge or choose newest. |

Do not infer legacy task state from an arbitrary concept's `status`. Identify a
legacy task by its membership in a legacy plan and its existing ID, dependency,
agent, scope, acceptance, and execution structure. Missing required fields block
dispatch until resolved, not a reset to pending. `task_status` without the schema,
a new-schema record with `status: pending`/`running`/etc., conflicting duplicated
state, or a schema/layout mismatch is ambiguous: report and clarify before writes
or dispatch. A new-schema `task_status: pending` plus `status: draft` is valid.

Before continuing, compare task records and the authoritative overview with the checkout,
validation evidence, and available native sessions, worktrees, and commits.
Establish that a prior writer has stopped before retrying a `running` task.
Unknown execution state is a blocker, not permission to launch a second writer.
Review partial work, preserve user changes, and repair a stale task table in its
existing authority. Reading/resuming does not convert the format or create indexes.

Recheck completed results against the current implementation. If a prerequisite
changes, invalidate affected downstream completion evidence and record which
tasks need review or reexecution. Revalidate the graph after revisions. Keep
plan paths and unfinished IDs in handoffs.

## Explicit conversion only

An existing `north/plans/<feature>.md` remains authoritative when resuming.
Its task table owns metadata/status and its detail sections hold evidence.
Apply this contract without rewriting history or creating a competing plan.
The same preservation rule applies to legacy directory plans, including the
bootstrap plan for this format change. New features use the new directory layout;
convert existing plans only on an explicit format-update/migration request.

Before conversion, establish that active writers have stopped. Record the old
authority and inventory incoming links/anchors. Preserve IDs, dependencies,
scopes, acceptance, history, dispatch references, and evidence. Move the detailed
directory overview from `index.md` to `plan.md`, replace the index with navigation,
and update known overview links (task-file links can remain stable). For a
single-file conversion, preserve a typed compatibility record at the old path
with useful historical anchors and a link to the new authority; do not leave two
active task tables. If incoming anchors cannot be preserved safely, report the
limit and agree a link repair before conversion.

Add `type` and `north_schema` to converted plan/tasks; move each unambiguous legacy
execution `status` to `task_status`, not to lifecycle. Do not invent lifecycle or
verification. Resolve ambiguous values before conversion. Preserve research and
other records unless explicitly included; identify any remaining legacy files
as a mixed bundle. Recheck graph, paths, selectors, and progress equivalence after
conversion; a reserved-name collision is not permission to overwrite content.
