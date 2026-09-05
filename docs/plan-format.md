# Execution plans

North uses Markdown plans to coordinate native OpenCode subagents. The primary
agent interprets dependencies and updates progress. There is no North executable
or automatic scheduler; these rules are instructions, not runtime guarantees.

For work with multiple delegated tasks, save one plan under the working project's
`north/plans/<change>.md`, honoring a configured North output directory. Reuse
the relevant plan when continuing work. If project conventions require another
location, keep the authoritative plan there and save a reference under `north/`.
For example, an existing OpenSpec task artifact can carry the execution details
when its format permits. Small changes do not require a plan. A read-only planning
request returns the proposed content without saving it or starting implementation.

## Template

Replace example paths and checks with real project details. The table is the
authoritative task status list; the detail sections hold context and evidence.

```markdown
# Plan: Add account settings

Goal: Users can view and update their display name.
Context: <requirements and relevant repository paths>
Max parallel: 2

| ID | Task | Depends on | Agent | Write scope | Status |
| --- | --- | --- | --- | --- | --- |
| A | Define settings contract | — | north-worker | src/contracts/settings.ts | pending |
| B | Implement endpoint | A | north-worker | src/server/settings/, tests/server/settings/ | pending |
| C | Implement settings form | A | north-worker | src/ui/settings/, tests/ui/settings/ | pending |
| D | Review integration | B, C | north-verifier | none (read-only) | pending |

## A: Define settings contract
Acceptance: Contract defines display-name input, result, and validation errors;
the primary agent runs the project's type check successfully.
Dispatch/session: Not dispatched.
Changed files: None yet.
Evidence: None yet.
Blockers: None known.

## B: Implement endpoint
Acceptance: Endpoint implements A; focused server tests cover persistence and
invalid input. Record the actual command and result.
Dispatch/session: Not dispatched.
Changed files: None yet.
Evidence: None yet.
Blockers: None known.

## C: Implement settings form
Acceptance: Form uses A; focused UI tests cover loading, saving, and errors.
Record the actual command and result.
Dispatch/session: Not dispatched.
Changed files: None yet.
Evidence: None yet.
Blockers: None known.

## D: Review integration
Acceptance: Primary runs the relevant integration checks on the stable combined
tree; verifier reviews the diff and supplied results against the goal. Findings
are resolved and missing evidence is supplied before the primary marks D done.
Dispatch/session: Not dispatched.
Changed files: None (review only).
Evidence: None yet.
Blockers: None known.

## Decisions and handoff
Record plan revisions, reasons for retries, remaining work, and any uncertainty
about an interrupted execution. Include the plan path in session handoffs.
```

## Coordination

The planner returns proposed plan content and remains read-only. The primary
agent is the sole plan editor, including updates requested by workers or reviewers.
Give each worker its task ID, goal, context, scope, prerequisites, and acceptance
checks. Keep task IDs stable as the plan evolves.

Before dispatch, check IDs are unique, every dependency exists, the graph has no
cycles, and parallel scopes are compatible. A task is ready only when it is
pending and all its dependencies are done. Launch ready independent tasks using
concurrent native Task calls when available, within the plan's concurrency limit;
otherwise run sequentially. In the example, B and C can run together after A is
done, and D waits for both. Independent work may continue when another task blocks.

Assume a shared checkout. Disjoint write paths are necessary, but also check
whether a task reads files another task is changing or uses shared test resources.
Account for generated files, lockfiles, and formatter output. Serialize conflicts,
including primary-agent edits. The final review and integration checks need a
stable combined tree. Scope changes must be coordinated before work expands.

## Status and evidence

| Status | Meaning |
| --- | --- |
| pending | Not dispatched; may be waiting for dependencies. |
| running | Dispatch recorded; execution has started or is being launched. |
| needs-review | Worker returned; diff and acceptance evidence need review. |
| done | Primary reviewed the result and confirmed acceptance checks. |
| blocked | Failure, missing input, or uncertain execution needs resolution. |

Record dispatch before launch, adding the native session reference when available.
Worker reports alone do not establish completion. Record changed files, actual
checks and results, missing evidence, and review findings. The verifier reviews
supplied evidence; its permissions leave execution of missing checks to the primary.

A failed task becomes blocked with diagnostics and a next action. Once a scoped
repair is ready, move it back to pending for dispatch. Preserve prior evidence and
do not repeatedly retry an unchanged failure. Changes to completed prerequisites
require reassessing downstream work and invalidating affected completion evidence.

## Resume

Read the existing plan and compare its entries with the checkout, evidence, and
available native session state before dispatching. A running entry may represent
active work, an interrupted worker, or a launch that never completed. Establish
that the previous execution has stopped before retrying; if that cannot be
determined, record a blocker instead of creating a second writer.

Review partial changes before continuing. Confirm completed results still apply
to the current tree and revise stale evidence or dependencies. Retain the plan
path and unfinished IDs in handoffs and compaction summaries. This supports
agent-led recovery; the file itself does not resume sessions or enforce execution.
