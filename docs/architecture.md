# Architecture

North consists of `install.sh`, shared instructions, four Markdown subagent
definitions, and task-specific skills. Installation links these assets into OpenCode's global
configuration. OpenCode loads the instructions and executes the subagents with
its native Task tool.

Skills live in `assets/skills/<name>/SKILL.md`. OpenCode discovers their names
and descriptions and loads the full instructions through its native skill tool
when relevant. Skills guide how an agent performs a task; subagents provide
delegated execution. Both primary agents and subagents can use skills.

The primary agent scopes the work, delegates planning or implementation when
useful, waits for dependencies, reviews the diff, and runs acceptance checks.
Workers return changes and evidence. The verifier provides a read-only review;
the conflict resolver handles explicitly assigned conflicts.

Subagents share the checkout unless isolation is arranged separately. Parallel
writes must have disjoint scopes; overlapping changes run sequentially. North
has no scheduler, subprocess runner, plan schema, approval database, run state,
automatic worktree management, or integration service.

Agent frontmatter supplies OpenCode permissions and Markdown supplies workflow
guidance. File scopes and validation expectations are instructions, not enforced
filesystem boundaries. The primary agent remains responsible for checking work.
