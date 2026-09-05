# North

Load relevant skills through OpenCode's native skill tool when their descriptions
match the task. Skills supply task-specific guidance to primary agents and
subagents; they do not require a separate execution process.

Use OpenCode's native subagents through the Task tool to delegate bounded work.
Handle small changes directly. For larger work, coordinate from the primary
agent and select the appropriate subagent:

- `north-planner`: break a goal into tasks with dependencies, file scopes, and
  acceptance checks. A concise Markdown plan is sufficient.
- `north-worker`: implement one clearly scoped task.
- `north-verifier`: independently review changes against acceptance criteria.
- `north-conflict-resolver`: resolve explicitly identified conflicts when needed.

Give each subagent the goal, relevant context, allowed file scope, dependencies,
and expected evidence. Use OpenCode's built-in exploration subagent for focused
repository research when useful. Do not launch separate OpenCode processes to
simulate delegation.

Parallelize only independent tasks. Native subagent sessions do not imply
separate Git worktrees: assume a shared checkout, assign disjoint write scopes,
and serialize overlapping edits. Wait for prerequisite tasks before dispatching
dependent work. Subagents should report blockers to the primary agent instead
of spawning more workers.

The primary agent reviews the resulting diff, runs relevant acceptance checks,
and resolves failures before reporting completion. A worker report alone is
not verification. Preserve existing user changes and follow the user's scope
and repository instructions for commits, branches, and integration.

If the repository uses OpenSpec, read its project instructions and relevant
requirements, design, and tasks as planning context. Keep any OpenSpec changes
within the user's requested scope. Optional plugins such as Open Loop are
configured separately; subagents must not invoke `/loop`.
