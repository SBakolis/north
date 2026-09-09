# Execution plans

The maintained [North plan contract](../assets/skills/north-sources/references/plan-format.md)
defines directory and task templates, metadata, status meanings, graph validation,
plan selection, and resume rules. It ships with `north-sources`, which the
installer includes whenever a consuming skill is selected.

New feature plans use `north/plans/<feature>/index.md`, a research record, and
linked task files. Existing single-file plans can resume in place. All paths use
the project's configured North directory.

The workflow is divided among these owners:

| Responsibility | Guidance |
| --- | --- |
| Artifact locations and plan contract | [north-sources](../assets/skills/north-sources/SKILL.md) |
| Clarification, exploration, and plan creation | [north-plan](../assets/skills/north-plan/SKILL.md) |
| Task transitions, layer barriers, and phase handoff | [north-execute](../assets/skills/north-execute/SKILL.md) |
| Worker assignments, isolation, and Git integration | [subagent-usage](../assets/skills/subagent-usage/SKILL.md) |
| Verified implementation knowledge | [north-save](../assets/skills/north-save/SKILL.md) |

For `T01 → {T02, T03} → T04`, execution verifies T01, runs the compatible tasks
T02 and T03 concurrently, and verifies both before starting T04. The execution
skill defines blocked-task handling and the boundary between layers. Standalone
delegation can use the shared contract without enabling the pipeline.

Plans preserve evidence for agent-led continuation. North does not provide an
executable scheduler or automatic session recovery.
