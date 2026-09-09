# Architecture

North consists of `install.sh`, shared instructions, Markdown commands, four subagent
definitions, and task-specific skills. Installation links these assets into OpenCode's global
configuration. OpenCode loads the instructions and executes the subagents with
its native Task tool.

Commands live in `assets/commands/*.md` and install into OpenCode's global
`commands/` directory. `/north` is always installed and directs the primary build
agent to create the current project's `north/` directory and offer OpenSpec initialization when
the CLI is installed. The confirmation and setup run in the current conversation.
See [OpenCode commands](https://opencode.ai/docs/commands/) for the command format.

The **North pipeline** workflow toggle installs `/north-plan`, `/north-execute`,
and `/north-save` with the five pipeline skills and their prerequisites. The
commands guide planning, implementation, and saving implementation knowledge,
then suggest the next command when their stage completes. If a required skill
is absent, the command directs the user to enable **North pipeline** through the
installer before continuing. Disabling the group removes its owned links while
preserving project plans and memories.

Skills live in `assets/skills/<name>/SKILL.md`. OpenCode discovers their names
and descriptions and loads the full instructions through its native skill tool
when relevant. Skills guide how an agent performs a task; subagents provide
delegated execution. Both primary agents and subagents can use skills.

Engineering skills provide focused methods for requirements clarification, bug
diagnosis, test design, code and architecture review, domain modeling, research,
prototyping, handoffs, and skill evaluation. They are independently selectable;
agents load only installed methods relevant to the assignment. The planner uses
them to propose decisions and acceptance checks, the worker to implement and
validate scoped behavior, and the verifier to review changes and supplied evidence.
The verifier's permissions still leave test execution to the primary agent.
The pipeline group includes `clarify-requirements`, `research`, `subagent-usage`,
and `north-sources` as shared dependencies. Standalone methods also include
`north-sources` when they use its artifact conventions or plan contract. These
dependencies remain ordinary selectable skills outside the pipeline; the five
pipeline skills are controlled by their group.

These methods reuse the authoritative project requirements, OpenSpec artifacts,
and North plan rather than creating another task lifecycle. The primary agent
owns shared documentation and progress updates. A read-only request or agent
returns proposed content without saving it. Skill evaluation checks observable
behavior in isolated scenarios; installer checks alone do not establish that a
skill selects or performs the right work.

The `north-sources` skill owns artifact locations and the shared plan contract.
`invoke-memory` owns implementation-memory retrieval; `north-save` owns its
persistence. Shared instructions route context lookup and define primary-agent
ownership and read-only behavior. `dry-skillify` records preference observations in
`north/dry/*.md` and promotes consistent patterns after three distinct instances
into `north/skills/<name>/SKILL.md`. North reads matching generated skills directly;
they do not need global installation. These are agent-driven Markdown workflows,
not background monitoring. The primary agent consolidates shared records to
avoid conflicting writes from subagents.
When no selected skill requires `north-sources`, it can be disabled; shared
instructions retain basic context lookup and a default artifact root.
Disabling `dry-skillify` stops preference recording and skill generation; current
user preferences and relevant previously saved skills still guide the task.

The primary agent scopes the work, delegates planning or implementation when
useful, waits for dependencies, reviews the diff, and runs acceptance checks.
Workers return changes and evidence. The verifier provides a read-only review;
the conflict resolver handles explicitly assigned conflicts.

`north-plan` delegates clarification to `clarify-requirements`, and
`north-explore` delegates research methods to `research`. `north-execute` owns
the pipeline lifecycle, while `subagent-usage` owns assignment and integration
mechanics. Their common plan schema and recovery rules are maintained once in
the [plan contract](../assets/skills/north-sources/references/plan-format.md).

`/north-plan <prompt>` uses the existing requirements clarification skill to
resolve consequential questions, `invoke-memory` to consult prior implementation
knowledge, and `north-explore` to research relevant repository patterns, similar
implementations, best practices, and libraries. New execution plans use this
structure under the working project's resolved North directory:

```text
north/plans/<feature-slug>/
  index.md
  research.md
  tasks/
    A-<task-slug>.md
    B-<task-slug>.md
```

The index records the goal, context, acceptance criteria, and links to research
and task files. Each task file owns its dependencies, status, scope, and evidence;
the index keeps a synchronized overview of task statuses and dependency layers.
Stable task IDs and links describe a directed acyclic graph (DAG). Only the
primary agent edits the plan; subagents return proposed updates. Existing
single-file plans can resume in place without being rewritten.

`/north-execute <feature-slug-or-plan-path>` checks relevant memories, validates
the DAG, and freezes its topological layers before dispatch. The primary agent
launches independent tasks within the current layer through OpenCode's native
Task tool, with a default concurrency limit of two. It reviews results and
verifies the whole layer before starting the next. Tasks move through `pending`,
`running`, `needs-review`, `done`, or `blocked`; a worker report alone cannot mark
a task done. Changes to the graph require updated layers before further dispatch.
See [execution plans](plan-format.md) for the templates, evidence requirements,
and resume procedure.

`/north-save <feature-slug-or-plan-path>` updates `north/memories/index.md` and
`north/memories/<feature-slug>.md` with verified implemented facts, including
references to the plan, code, and validation evidence. Proposed or unfinished
work is not saved as implemented knowledge. `invoke-memory` reads relevant
memories before implementation and checks their applicability to the current
repository; stale memories are context, not instructions. Memory lookup is
read-only and does nothing when no memories exist. All these paths honor the
project's configured North directory.

Subagents share the checkout unless isolation is arranged separately. Parallel
writes must have disjoint scopes; overlapping changes run sequentially. North
has no executable scheduler, subprocess runner, machine-validated plan schema,
approval database, automatic worktree management, or integration service. Plan
files persist progress and evidence; they do not provide automatic crash recovery.

Agent frontmatter supplies OpenCode permissions and Markdown supplies workflow
guidance. File scopes and validation expectations are instructions, not enforced
filesystem boundaries. The primary agent remains responsible for checking work.
