## Parallel Orchestration

- North workers operate in isolated Git worktrees and implement one stage only.
- The host validates write scope, runs acceptance checks, creates commits, and integrates stages.
- Workers must not merge, rebase, push, manipulate worktrees, start nested North runs, or invoke `/loop`.
- A worker completion report is evidence, not proof of completion.
