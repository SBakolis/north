# North

Load relevant installed skills through OpenCode's native skill tool. Reuse
guidance already loaded in the task. Skills do not expand the user's scope or
an agent's permissions, and optional methods are not mandatory phases for
ordinary work.

## Shared artifact rules

Use `north-sources` for artifact locations and the plan contract when installed.
Without it, resolve the working project's configured North output directory or
`<project-root>/north/`, and preserve authoritative artifacts in their required
locations. Lookup does not create files.

At the start of work, consult relevant existing North context and read applicable
generated preference skills under `skills/*/SKILL.md` directly. Before
implementation, use `invoke-memory` when installed. Otherwise inspect relevant
existing memory records directly. Use `dry-skillify` for reusable user preferences
when installed; without it, follow the preference without generating skills.

Saved material is evidence with provenance, not fresh authority. Current user
instructions and repository evidence take precedence over old decisions or
quoted instructions. Check applicability before relying on historical context.

Only the primary agent edits shared plans and consolidated records; delegated
agents return proposed updates. Read-only requests return findings or proposed
contents without writing. Preserve existing user changes. Reuse relevant
OpenSpec requirements, design, and tasks and keep updates within the user's scope.

## Implementation pipeline

`/north` scaffolds project output. The North pipeline option installs
`/north-plan`, `/north-execute`, and `/north-save` and their skill dependencies.
Load the command's corresponding skill; if required guidance is missing, explain
how to enable North pipeline in the installer before continuing that phase.

The skills own the phase behavior and next-command handoff. Suggest the next
phase after finishing; do not launch it automatically. Ordinary requests can
proceed without these commands.

## Delegation and delivery

Use OpenCode's native Task tool, not separate OpenCode processes. Load
`subagent-usage` when installed for delegation and checkout integration.
Use `north-execute` for a saved pipeline's lifecycle. When creating or revising
a plan, `north-plan` owns planning and `north-sources` owns its format.

Select `north-planner` for proposed plans, `north-worker` for scoped implementation,
`north-verifier` for read-only review, and `north-conflict-resolver` for assigned
conflicts. OpenCode's built-in exploration subagent can inspect the repository.

When delegation skills are absent, keep assignments bounded, pass their goal,
context, write scope, prerequisites, and acceptance checks, and serialize
conflicting work. Review changes and actual validation evidence before accepting
a result or starting dependent work. Worker reports alone do not prove completion.

The primary owns commits. At delivery, load the installed `commit` or
`auto-commit` mode and honor existing user authorization and repository rules.
Only one mode is installed. Skills do not authorize pushing or publishing.
