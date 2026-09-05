---
name: auto-commit
description: Automatically commit completed and validated implementation work using feat, fix, or chore messages without an extra confirmation step. Use when finishing a change in a Git repository with Auto commit enabled.
---

# Auto commit

Treat a local commit as part of finishing implementation. After the requested
work and relevant validation are complete, commit the task's changes without
asking for an extra go-ahead. Respect explicit user instructions to leave changes
uncommitted and applicable repository restrictions. Read-only work needs no commit.

Inspect the current branch, working-tree diff, and staged changes. Include only
completed changes belonging to the task, preserving unrelated user edits and staged
work. Stage specific files or hunks and inspect the staged diff before committing.
Do not include someone else's staged changes or reset their staging to simplify
your commit. If changes cannot be safely separated, report the concrete blocker.
If there is nothing new to commit, report that outcome without an empty commit.

Choose a concise, imperative subject in the form `type: summary` or
`type(scope): summary`:

- `feat`: a new capability or intentional behavior change.
- `fix`: a correction to broken or incorrect behavior.
- `chore`: maintenance, tooling, documentation, or cleanup without a feature or fix.

Describe the resulting change, for example `fix(installer): preserve the selected commit mode`.
Keep each commit coherent; split independent changes when that improves review.
Use a body only when motivation, tradeoffs, or validation need explanation.

Create the commit and honor repository hooks. If a hook fails, address in-scope
issues and retry after verification; do not bypass hooks or claim completion while
blocked. Verify the resulting commit's hash, message, and contents, then report
the commit, validation, and remaining local changes.

In delegated work, the primary agent commits accepted worker changes after review;
workers leave their edits uncommitted. Use the same message format for task commits
in worktrees and follow the integration plan before reporting delivery complete.
Auto commit covers local commits only: it does not authorize pushing, publishing,
amending existing history, or including unfinished or unrelated work.
