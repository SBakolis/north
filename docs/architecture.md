# Architecture

North consists of `install.sh`, shared instructions, a `/north` command, four Markdown subagent
definitions, and task-specific skills. Installation links these assets into OpenCode's global
configuration. OpenCode loads the instructions and executes the subagents with
its native Task tool.

Commands live in `assets/commands/*.md` and are always installed into OpenCode's
global `commands/` directory. `/north` directs the primary build agent to create
the current project's `north/` directory and offer OpenSpec initialization when
the CLI is installed. The confirmation and setup run in the current conversation.
See [OpenCode commands](https://opencode.ai/docs/commands/) for the command format.

Skills live in `assets/skills/<name>/SKILL.md`. OpenCode discovers their names
and descriptions and loads the full instructions through its native skill tool
when relevant. Skills guide how an agent performs a task; subagents provide
delegated execution. Both primary agents and subagents can use skills.

The `north-sources` skill makes `<working-project>/north/` the persistent store
for North's supporting artifacts and consults relevant saved context on each
task. `dry-skillify` records user-supported preference observations in
`north/dry/*.md` and promotes consistent patterns after three distinct instances
into `north/skills/<name>/SKILL.md`. North reads matching generated skills directly;
they do not need global installation. These are agent-driven Markdown workflows,
not background monitoring. The primary agent consolidates shared records to
avoid conflicting writes from subagents.

The primary agent scopes the work, delegates planning or implementation when
useful, waits for dependencies, reviews the diff, and runs acceptance checks.
Workers return changes and evidence. The verifier provides a read-only review;
the conflict resolver handles explicitly assigned conflicts.

For multiple delegated tasks, the primary agent maintains a Markdown execution
plan in the working project's `north/plans/` directory. Stable task IDs and
dependencies describe a DAG; the primary agent selects ready tasks and dispatches
independent work through OpenCode's native Task tool, with a default concurrency
limit of two. Only the primary agent edits the plan. Worker results require review
before completion unlocks dependent work. See [execution plans](plan-format.md)
for the template, statuses, and resume procedure.

Subagents share the checkout unless isolation is arranged separately. Parallel
writes must have disjoint scopes; overlapping changes run sequentially. North
has no executable scheduler, subprocess runner, machine-validated plan schema,
approval database, automatic worktree management, or integration service. Plan
files persist progress and evidence; they do not provide automatic crash recovery.

Agent frontmatter supplies OpenCode permissions and Markdown supplies workflow
guidance. File scopes and validation expectations are instructions, not enforced
filesystem boundaries. The primary agent remains responsible for checking work.
