---
name: north-sources
description: Resolve North's project-local output directory and apply shared artifact and plan-format conventions. Use when locating or saving North artifacts, or reading and writing a North task graph.
---

# North sources

## Resolve the artifact root

Use the configured North output directory or `<project-root>/north/`. Resolve
the project root from the repository or established workspace, not the shell's
current subdirectory or the checkout containing the installed kit. A project
named `north` still uses `<project-root>/north/`. Keep this resolved path for
the task and pass it to delegated agents.

Missing output is normal. Lookup does not create directories; create them only
when saving. Report conflicting files or symlinks instead of replacing them or
writing through them. Keep implementation files and authoritative project
requirements where their tooling or project instructions require them.

## Store supporting artifacts

| Artifact | Location relative to the resolved North directory | Owner |
| --- | --- | --- |
| Feature plan | `plans/<feature>/index.md`, `research.md`, `tasks/*.md` | `north-plan` creates; `north-execute` progresses |
| Implementation memory | `memories/index.md`, `memories/<feature>.md` | `invoke-memory` reads; `north-save` writes |
| Standalone research | `research/` | `research` |
| Continuation brief | `handoffs/` | `handoff` |
| Experiment notes | `prototypes/` | `prototype` |
| Skill evaluation evidence | `evaluations/` | `skill-evaluation` |
| Preference observations | `dry/<behavior>.md` | `dry-skillify` |
| Generated preference skills | `skills/<name>/SKILL.md` | `dry-skillify` |

Reuse an existing relevant artifact rather than creating competing copies.
For notes without a category above, choose a descriptive path under the artifact root.
Link authoritative material stored elsewhere; do not relocate or duplicate it.
Use relative Markdown links that resolve from the file containing them. Preserve
unrelated records and enough provenance to judge whether a saved claim applies.
Do not store secrets or unnecessary personal information.

This skill defines storage conventions. It does not retrieve implementation
memories, save feature knowledge, or promote preferences; use the owners above.

## Work with a plan

Read [the plan contract](references/plan-format.md) when creating, validating,
resuming, or updating a task graph. It owns task metadata, status meanings,
graph validation, and recovery rules, including existing single-file plans.
Execution order and layer barriers belong to `north-execute`; delegation and
checkout integration belong to `subagent-usage`.
