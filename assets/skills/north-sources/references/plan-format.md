# North plan contract

Use the artifact root resolved by `north-sources`. New feature plans use:

```text
north/plans/account-settings/
  index.md
  research.md
  tasks/
    T01-contract.md
    T02-endpoint.md
    T03-form.md
    T04-integration.md
```

The primary agent is the only editor of the shared plan. Each task file's
frontmatter is authoritative for task metadata and status; its body holds
acceptance and execution evidence. The index is a synchronized navigation view.
Reconcile a stale index against task files and actual execution before dispatch.
These Markdown conventions guide an agent; there is no automatic scheduler.

## Index example

```markdown
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
id: T02
status: pending
depends_on: [T01]
agent: north-worker
write_scope: [src/server/settings/, tests/server/settings/]
---

# T02: Implement settings endpoint

Plan: [Account settings](../index.md)
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

`research.md` links back to `index.md` and records the question, repository and
external evidence, alternatives considered, selected approach, applicability,
and unresolved gaps. It is context, not an executable task. Link to relevant
research already stored elsewhere instead of duplicating it.

## Graph validation

Task IDs are stable and unique. Every dependency names an existing task, every
local reference resolves, and the graph has no cycles or self-dependencies.
Write scopes and acceptance checks must describe real project paths and
observable outcomes. Include semantic dependencies such as reads of unfinished
contracts, generated outputs, lockfiles, and shared test resources.

Use a positive `Max parallel` value, defaulting to 2. Compute the index's layers
from dependencies: roots are layer 1; each other task is one layer after its
latest prerequisite. Recompute after graph revisions. The execution workflow
decides when a layer may start; `north-execute` owns pipeline layer barriers.

## Status and evidence

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
label alone is not completion evidence. Synchronize index entries from task files;
preserve previous outcomes and reasons for repairs in the history.

If project instructions also require an external progress artifact such as
OpenSpec tasks, record its location and synchronize it from verified outcomes.

## Select and resume a plan

An explicit selector may be a feature name, plan directory, index path, or legacy
single-file plan. Otherwise use an unambiguous plan from the conversation, or a
sole plan matching the requested phase. Ask when several match; do not choose
by modification time. If none exists, identify the missing plan.

Before continuing, compare task records and the index with the checkout,
validation evidence, and available native sessions, worktrees, and commits.
Establish that a prior writer has stopped before retrying a `running` task.
Unknown execution state is a blocker, not permission to launch a second writer.
Review partial work, preserve user changes, and repair a stale index.

Recheck completed results against the current implementation. If a prerequisite
changes, invalidate affected downstream completion evidence and record which
tasks need review or reexecution. Revalidate the graph after revisions. Keep
plan paths and unfinished IDs in handoffs.

## Existing single-file plans

An existing `north/plans/<feature>.md` remains authoritative when resuming.
Its task table owns metadata/status and its detail sections hold evidence.
Apply this contract without rewriting history or creating a competing plan.
New features use directory plans; migrate existing plans only when requested.
