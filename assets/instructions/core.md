# North

At the start of each task, load `north-sources` and consult relevant saved context
in the working project's `north/` directory, including applicable generated
preference skills. Save North's supporting output there as the task progresses.
When the user expresses or corrects a reusable working preference, load
`dry-skillify` to record evidence and promote supported recurring patterns.

Load relevant skills through OpenCode's native skill tool when their descriptions
match the task. Skills supply task-specific guidance to primary agents and
subagents; they do not require a separate execution process.

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

For work with multiple delegated tasks, save one Markdown execution plan at
`north/plans/<change>.md` under the working project, honoring the resolved North
output directory. Reuse an existing relevant plan. Follow a required project
planning location instead when applicable, leaving a reference in North's output
directory rather than a competing plan. For read-only requests, return a proposed
plan without saving it or starting implementation.

Include the goal, context paths, a concurrency limit (default 2), and tasks with
stable IDs, dependencies, agent, repository-relative write scope, acceptance
checks, and status. Record each task's dispatch/session reference when available,
changed files, validation evidence, and blockers. Only the primary agent edits
the execution plan; subagents return proposed updates. The planner stays read-only.

Before dispatch, check for duplicate IDs, missing dependencies, cycles, and
overlapping scopes. Dispatch only pending tasks whose dependencies are all done,
up to the plan's concurrency limit, using concurrent native Task calls for
independent work when available. Otherwise execute sequentially. Subagents should
report blockers to the primary agent instead of spawning more workers.

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
dependents waiting; independent tasks may continue. Do not repeatedly retry an
unchanged failure. Record revised dependencies or scopes before redispatch and
revisit affected downstream results when a previously completed task changes.

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

If the repository uses OpenSpec, read its project instructions and relevant
requirements, design, and tasks as planning context. Keep any OpenSpec changes
within the user's requested scope. Optional plugins such as Open Loop are
configured separately; subagents must not invoke `/loop`.
