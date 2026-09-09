# North

At the start of each task, consult relevant saved North context, including
applicable generated preference skills. Use `north-sources` when installed to
resolve and maintain these records. Otherwise use the configured North output
directory or `<project-root>/north/`, resolved from the working project rather
than the kit checkout, and read relevant artifacts and preference skills directly.
Save supporting output there when writing is within scope; read-only requests
return proposed updates without saving them. Keep implementation files and
authoritative project artifacts in their required locations.

Before implementation, load `invoke-memory` when installed to retrieve relevant
verified implementation knowledge from `north/memories/`. Otherwise consult
relevant memory files directly, using their index when present. Missing memories
are normal; do not create them during lookup. Check saved claims against current
code and instructions before relying on them.

When the user expresses or corrects a reusable working preference, use
`dry-skillify` if installed to record evidence and promote supported patterns.
Without it, follow the current preference without generating learned skills.

Load relevant skills through OpenCode's native skill tool when their descriptions
match the task. Skills supply task-specific guidance to primary agents and
subagents; they do not require a separate execution process.

Use optional engineering skills when installed and relevant; they are not
prerequisites for ordinary work. Reuse established requirements and decisions.
Scale research, clarification, testing, and review to the task's uncertainty
and consequences.
Skills do not expand an agent's permissions or assigned scope. Read-only agents
return proposed artifact updates for the primary agent to consolidate.

The explicit implementation pipeline is `/north-plan <prompt>`, then
`/north-execute <feature>`, then `/north-save <feature>`. Load the corresponding
skill for each command. Planning invokes `invoke-memory`, `clarify-requirements`,
and `north-explore` (which uses `research`) and saves a plan without implementing
it. Execution invokes `invoke-memory` and `subagent-usage` and implements the plan
one dependency layer at a time. Saving records verified implementation knowledge
under `north/memories/` for later lookup. End each phase by suggesting the next
command; after saving, suggest `/north-plan <next feature prompt>`. Do not start
the next phase automatically. If blocked, explain the resolution/resume step
before advancing. Ordinary requests can still proceed without these commands.

`/north` is always installed. The **North pipeline** workflow toggle installs
the three pipeline commands, their five skills (including `invoke-memory`),
and required dependencies together. If a pipeline command or required skill
is absent, explain how to enable **North pipeline** in North's installer before
continuing that phase. Do not simulate unavailable skills or silently skip a
required phase. Disabling the group preserves saved project plans and memories.

Use OpenCode's native subagents through the Task tool to delegate bounded work.
Handle small changes directly. For larger work, coordinate from the primary
agent and select the appropriate subagent:

- `north-planner`: break a goal into tasks with dependencies, file scopes, and
  acceptance checks, returning a Markdown execution plan.
- `north-worker`: implement one clearly scoped task.
- `north-verifier`: independently review changes against acceptance criteria.
- `north-conflict-resolver`: resolve explicitly identified conflicts when needed.

Give each subagent the goal, relevant context, allowed file scope, dependencies,
and expected evidence. Use OpenCode's built-in exploration subagent for focused
repository research when useful. Do not launch separate OpenCode processes to
simulate delegation.

For new work with multiple delegated tasks, save a directory plan at
`north/plans/<feature>/index.md` with linked `research.md` and
`tasks/<ID>-<slug>.md` files, honoring the resolved North output directory.
Use `north-plan` when installed for the format and planning workflow. Reuse an
existing relevant plan, including legacy single-file plans. Keep required
project requirements authoritative and link to them rather than duplicating
them. For read-only requests, return a proposed plan without saving it or
starting implementation.

Include the goal, context paths, a concurrency limit (default 2), and tasks with
stable IDs, dependencies, agent, repository-relative write scope, acceptance
checks, and status. For directory plans, task frontmatter owns `id`, `status`,
`depends_on` (IDs), `agent`, and `write_scope`; the index links tasks and mirrors statuses,
dependencies, and computed layers. Record each task's dispatch/session reference
when available, changed files, validation evidence, and blockers. Existing
single-file plans retain their task table and evidence sections. Only the primary
agent edits the execution plan; subagents return proposed updates. The planner stays read-only.

Before dispatch, check for duplicate IDs, missing dependencies, cycles, and
overlapping scopes. Dispatch only pending tasks whose dependencies are all done,
up to the plan's concurrency limit, using concurrent native Task calls for
independent work when available. Compute layers with roots first and each other
task one layer after its latest prerequisite. Finish, review, verify, and
integrate the entire active layer before dispatching the next. Queue excess or
conflicting tasks within that layer. When concurrency is unavailable, execute
sequentially. Subagents should report blockers to the primary agent instead of
spawning more workers.

Native subagent sessions do not imply separate Git worktrees: assume a shared
checkout. Parallel tasks must have disjoint write scopes and must not rely on
files another active task is changing. Include generated files, formatters,
lockfiles, and test resources when judging conflicts. Serialize conflicting work,
including the primary agent's edits, and run final checks against a stable tree.

Track tasks as pending, running, needs-review, done, or blocked. Record dispatch
before launching work and add the native session reference when returned. A
worker result moves a task to needs-review; mark it done only after reviewing
the diff and checking acceptance evidence. Record failures and required repairs
as blocked, then return the task to pending once a scoped retry is ready. Leave
dependents waiting; independent tasks in the active layer may continue. Do not
repeatedly retry an unchanged failure. Record revised dependencies or scopes
before redispatch and revisit downstream results when a completed task changes.

On resume, read the plan and reconcile it with the checkout, saved evidence, and
available native session state. Do not infer completion from a status label or
redispatch a running task until its prior execution is known to have stopped.
Review partial changes before retrying. If execution state cannot be established,
record the blocker instead of starting another writer. Keep the plan path and
unfinished task IDs in handoffs or compaction summaries.

The primary agent reviews the resulting diff, runs relevant acceptance checks,
and resolves failures before reporting completion. A worker report alone is
not verification. Preserve existing user changes and follow the user's scope
and repository instructions for commits, branches, and integration.

When finishing implementation in a Git repository, load the installed commit
mode skill: `auto-commit` commits validated work automatically; `commit` prepares
the commit and waits for the user's go-ahead unless already explicitly authorized.
Only one mode is installed. The primary agent owns commits for delegated work.

If the repository uses OpenSpec, read its project instructions and relevant
requirements, design, and tasks as planning context. Keep any OpenSpec changes
within the user's requested scope.
