---
name: commit
description: Prepare a reviewed Git commit after finishing implementation, using feat, fix, or chore messages, and wait for the user's go-ahead before committing. Use when completing work in a Git repository or when the user requests a commit.
---

# Commit

After finishing implementation and relevant validation, prepare a concrete commit
for the completed work. Inspect the current branch, working-tree diff, and staged
changes. Identify only changes belonging to the task; preserve unrelated user
edits and staged work. If there is nothing new to commit, report that outcome.

Choose a concise, imperative subject in the form `type: summary` or
`type(scope): summary`:

- `feat`: a new capability or intentional behavior change.
- `fix`: a correction to broken or incorrect behavior.
- `chore`: maintenance, tooling, documentation, or cleanup without a feature or fix.

Describe the resulting change, for example `feat(installer): add commit mode toggle`.
Keep each commit coherent; split independent changes when that improves review.
Use a body only when motivation, tradeoffs, or validation need explanation.

Present the proposed message, included changes, and validation results, then wait
for the user's go-ahead before creating the commit. Explain that this installed
skill requires confirmation. If the user has already explicitly authorized the
commit in the current task, proceed without asking again. Do not treat a general
implementation request as commit approval in this mode.

After authorization, recheck the diff and stage only the agreed files or hunks.
Inspect the staged diff to ensure the commit contains no unrelated changes. Create
the commit, honor repository hooks, and verify its hash, message, and contents.
If a hook fails, address in-scope issues and retry after verification; do not bypass
hooks or report success while blocked. Report the commit and remaining local changes.

In delegated work, the primary agent owns commits and integration. Workers return
uncommitted changes and validation evidence. Apply the same confirmation rule to
task commits in worktrees. Respect explicit instructions to leave work uncommitted.
Committing does not authorize pushing, publishing, amending existing history, or
including unfinished or unrelated work.
