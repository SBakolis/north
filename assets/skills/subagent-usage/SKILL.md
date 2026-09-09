---
name: subagent-usage
description: Delegate bounded tasks through native subagents, isolate checkouts, and integrate accepted changes into the original source branch. Use when work benefits from delegation or Git worktree coordination; north-execute owns the pipeline lifecycle.
---

# Subagent usage

Prefer subagents for bounded research, implementation, and independent review
when delegation makes meaningful progress possible alongside the primary agent's
work. Look for independent tasks before doing a substantial change serially.
Handle trivial edits directly; do not split tightly coupled work merely to create
parallel activity. Use OpenCode's native Task tool and North's existing agents.
The primary agent owns coordination, Git operations, and final integration;
workers do not spawn further workers or manage branches themselves.

## Assign compatible work

Load `north-sources` and use its
[plan contract](../north-sources/references/plan-format.md) for graph validation,
task evidence, and recovery. For multiple delegated tasks, create or reuse a plan
under that contract; a separate planning phase is optional outside the pipeline.
Follow the active coordinator's ordering: `north-execute` decides which pipeline
layer may run.
For standalone delegation, dispatch independent work after its prerequisites
are accepted; this skill does not start a pipeline phase.

Use concurrent native Task calls within the coordinator's limit (default 2).
Serialize assignments with conflicting reads, writes, generated outputs,
lockfiles, or test resources, including primary-agent edits. Worktrees isolate
file writes but do not remove semantic or external-resource conflicts.
Coordinate scope changes before a worker expands its assignment.

Give each worker its task ID, goal, relevant context, prerequisite results, allowed
write scope, acceptance checks, and exact checkout path and branch. Require it to
confirm that checkout before editing and return changed files, actual validation
results, and blockers. Review returned changes before marking a task done.

## Prefer worktrees for implementation

For substantial delegated implementation, prefer separate Git worktrees, especially
when multiple workers would otherwise write to one checkout. Read-only exploration
and review generally need no new worktree. Use a shared checkout when isolation
is unavailable or disproportionate, and serialize conflicting work there.

Before creating worktrees, record the source checkout, original source branch,
starting commit, and existing local changes in the plan. The source branch is
the branch where the requested work began; do not substitute the default branch.
If HEAD is detached and no intended target can be established from context, resolve
the integration target before choosing a source branch.

The primary agent creates a dedicated integration branch/worktree from the source
commit and task branches/worktrees from the appropriate verified integration
commit. Follow repository branch conventions, using `feature/` otherwise. Record
each task's worktree, branch, and base commit. Use distinct paths and branches;
native subagent sessions alone do not provide isolation. Share the authoritative
plan's location so workers do not maintain competing plan copies.

Ensure dependent worktrees contain the accepted prerequisite commits before
dispatch. Never assume uncommitted source changes are present in a new worktree.
If prerequisites depend on local edits, preserve them and establish an explicit,
reviewable base that includes only the relevant changes before proceeding. Do not
silently stash, discard, or commit unrelated user work.

## Integrate and finish on the source branch

Treat merging the completed implementation back into the recorded source branch
as part of delivery, subject to explicit user and repository Git restrictions.
Follow the installed `commit` or `auto-commit` mode when creating task commits.
Do not stop with finished work stranded on task branches when local integration
is permitted. Local integration does not imply permission to push or publish.

Workers leave their edits uncommitted for review. The primary agent reviews the
diff and evidence, stages only accepted task changes, and commits them on the
task branch. Merge accepted task branches into the integration branch in dependency
order. Make those commits available to downstream tasks before marking their
prerequisites done. Record commit IDs and integration evidence in the plan.

Serialize merges. Inspect conflicts and resolve them against the task requirements;
use `north-conflict-resolver` for a bounded conflict assignment when useful. Do not
blindly choose one side. Re-run affected checks after conflict resolution and
validate the stable combined implementation before merging it into the source.

Before the final merge, recheck the source branch and working tree. If the source
has advanced, incorporate its current commits into the integration branch and
repeat affected validation. Preserve existing source edits; if they prevent safe
integration, retain the verified branches and report the precise blocker. Do not
force-reset, force-push, or switch another active checkout's branch to finish.

Merge from the recorded source checkout when safe, then verify that the intended
commits are included and run relevant final checks on the resulting source tree.
Only report integration complete after it actually succeeds. Once no worker is
active, remove task-owned worktrees and branches only after confirming that they
are clean and their work is merged. Preserve incomplete or unmerged work for recovery.

Report the source branch, integration result, validation, and any remaining
worktrees or blockers. Return these results to the active coordinator for progress
updates and the next phase; use the plan contract when resuming integration.
