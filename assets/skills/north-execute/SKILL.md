---
name: north-execute
description: Implement a saved North Markdown dependency plan with native subagents, parallel work within each layer, and verified progress before advancing. Use for /north-execute or resuming a North feature implementation.
---

# North execute

## Select and reconcile

Resolve the working project's root and configured North directory, defaulting
to `<project-root>/north/`. Select the feature named by the user, a supplied plan
directory/index path, or the unambiguous active plan from the conversation.
Without a selector, use a sole unfinished plan if one exists. If multiple plans
could match, ask which one; do not choose by modification time. If no plan exists,
suggest `/north-plan <feature prompt>` rather than inventing one and executing it.

Read the index, every task file, relevant research and requirements, and current
project instructions. Load `invoke-memory` before editing implementation files,
then load `subagent-usage` for dispatch, checkout isolation, and integration.
If required skills are missing, identify them and explain how to enable
**North pipeline** in North's installer, which includes all pipeline skills and
dependencies, before starting implementation.

Directory plans use `index.md`, `research.md`, and `tasks/<ID>-<slug>.md`.
Task frontmatter owns `id`, `status`, `depends_on` (IDs), `agent`, and
`write_scope` (project-relative paths); the body owns acceptance and execution
evidence. The index mirrors dependencies, layers, and status and links tasks.
For an existing single-file plan, its table and detail sections retain that role.
Only the primary edits shared plan records; workers return evidence and updates.
If project instructions require an external progress record (such as OpenSpec
tasks), synchronize it from verified outcomes as recorded in the plan.

Validate unique IDs, existing dependencies and referenced files, absence of
cycles/self-dependencies, real write scopes, and testable acceptance criteria.
Reconcile statuses with actual changes, review evidence, commits, and sessions.
Establish that any previous writer has stopped before retrying a `running` task;
if unknown, record a blocker instead of creating a second writer. Preserve
partial work and unrelated user changes. Recheck stale completion evidence.

## Execute one dependency layer at a time

Compute topological layers from the validated graph: roots are layer 1, every
other task is one layer after its latest prerequisite. Resume at the first
unfinished layer. Freeze that layer's membership before dispatch; finishing one
task does not allow a later layer to start while its peers remain unfinished.

Within the active layer:

1. Select `pending` tasks whose prerequisites are all `done` and available in
   the assigned checkout. Check shared reads, writes, generated outputs,
   lockfiles, and test resources for conflicts. Queue or serialize conflicting
   tasks and excess work beyond `Max parallel` (default 2).
2. Record dispatch and `running` status in the task and index, then launch
   independent tasks concurrently with OpenCode's native Task tool. Use
   `north-worker` for scoped implementation and `north-verifier` for read-only
   review. Pass task ID/file, goal, context and memory pointers, prerequisite
   results, allowed paths, exact checkout, and acceptance checks. Add session
   references when available. Workers must not edit plans, claim other tasks,
   commit, or spawn workers.
3. Use worktrees when useful under `subagent-usage`; native sessions alone do
   not isolate files. Respect the installed commit mode and existing user
   authorization. If worktree integration needs an ungranted commit, use a
   compatible shared checkout with serialized conflicting edits where practical,
   or report the concrete integration blocker. Do not silently change commit mode.
4. Returned work becomes `needs-review`. Review the diff and actual acceptance
   evidence, run missing checks, and integrate accepted work into the tree that
   dependents will use. Only then mark it `done`. Record changed files, check
   commands/results, review findings, and integration/session references.
5. A failure becomes `blocked` with diagnostics and a next action. Finish
   independent work in this layer, then resolve the blocker and return its task
   to `pending` for a scoped repair. Do not retry an unchanged failure repeatedly.
   Do not cross the layer boundary until every task in it is `done`.

After the whole layer is verified and integrated, advance to the next and repeat.
When native parallel execution is unavailable, perform the same tasks serially
and disclose that limitation. Keep plan updates and merges serialized. Revalidate
and recompute layers after changing task boundaries or dependencies; reassess
downstream results whenever a completed prerequisite changes.

## Finish and hand off

Run the planned final integration checks against the stable combined tree.
The read-only verifier reviews supplied results; the primary executes missing
checks and resolves findings. Follow project instructions and commit mode for
delivery, and confirm source-branch integration if worktrees were used.

Synchronize the index with task statuses and record final evidence and remaining
limitations. Report completion only when every required task is reviewed,
verified, and integrated. Suggest `/north-save <feature>` to preserve what was
implemented. If blocked, report unfinished IDs and how to resume this command;
explain that saving final memory follows successful completion. If the user
explicitly requests `/north-save` before then, that skill can record only the
verified subset as partial memory. Do not save memory or begin another phase
automatically.
