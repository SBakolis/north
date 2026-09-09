```text
 _   _  ___  ____ _____ _   _
| \ | |/ _ \|  _ \_   _| | | |
|  \| | | | | |_) || | | |_| |
| |\  | |_| |  _ < | | |  _  |
|_| \_|\___/|_| \_\|_| |_| |_|
```

![North banner](assets/north.png)

North is a small set of instructions, agent definitions, and skills for OpenCode.
OpenCode provides subagent execution; the primary agent coordinates planning,
implementation, and review.

## Install

Clone this repository to a permanent location, then run:

```sh
./install.sh
```

The script builds and opens a small Ratatui installer. It requires a POSIX shell
and Rust/Cargo (Rust 1.88+); the first build downloads its dependencies. Use
Up/Down and Space to choose options in the **Workflow**, **Skills**, **OpenSpec**,
and **Installation** categories, then Enter to install shared instructions,
commands, four subagents, and the selected skills into
`${XDG_CONFIG_HOME:-$HOME/.config}/opencode`.

Select the optional **OpenSpec CLI** checkbox to install OpenSpec if it is missing,
or run `./install.sh --all --openspec`. The installer checks for an existing CLI
first; installation uses npm and requires Node.js 20.19.0+.

Select **Merge installations** (or run `./install.sh --merge`) to merge North's
configuration into existing `opencode.json` / `opencode.jsonc` files and keep your
`AGENTS.md` active. Nested settings are combined, arrays such as `plugin` are
extended without duplicate entries, and existing settings and JSONC comments are preserved.

Without merging, your existing `AGENTS.md` is saved as `AGENTS-backup.md`. Run `./install.sh` again
to enable or disable skills, or press `u` to uninstall North and restore that
backup. Keep the checkout in place because the installed links point into it.

Start a new OpenCode session in your project and run `/north` to create its
`north/` directory. If the OpenSpec CLI is installed and the project does not
already have OpenSpec, the command asks whether to initialize it with OpenCode
support. Existing North contents and OpenSpec setup are preserved.

Enable **North pipeline** in the installer's **Workflow** category, then run:

```text
/north-plan Add account settings so users can update their display name
/north-execute account-settings
/north-save account-settings
```

`/north-plan` clarifies the request, consults relevant memories, and researches
similar implementations, best practices, and useful libraries. It saves a linked
Markdown plan under `north/plans/<feature-slug>/`, with acceptance checks,
dependencies, and task statuses. `/north-execute` implements independent tasks
with subagents, verifies each dependency layer, and then starts the next layer.
`/north-save` records verified implementation knowledge in `north/memories/` for
future tasks. Each command suggests the next command; after saving, start the
next feature with `/north-plan`. Execute and save also accept a plan path.

You can still ask for a change normally. The shared instructions guide delegation
for larger tasks. You can also invoke an agent explicitly, for example:

```text
@north-planner plan the login change
```

See [installation](docs/installation.md) for existing configurations, updates,
and removal, and [architecture](docs/architecture.md) for the workflow.

See the [execution plan format](docs/plan-format.md) for linked task files,
statuses, and resuming work. Plans and memories live in the working project,
honoring its configured North directory. Coordination runs through agent
instructions and OpenCode's native subagents.

## Skills

OpenCode discovers skill descriptions and loads matching guidance on demand.

Implementation pipeline:

- [north-plan](assets/skills/north-plan/SKILL.md): clarify the requested feature
  and create a linked Markdown plan with task dependencies and acceptance checks.
- [north-explore](assets/skills/north-explore/SKILL.md): research repository
  patterns, similar implementations, best practices, and suitable libraries.
- [north-execute](assets/skills/north-execute/SKILL.md): dispatch independent
  tasks in parallel and verify each dependency layer before continuing.
- [north-save](assets/skills/north-save/SKILL.md): save verified implementation
  facts with references to plans, code, and validation evidence.
- [invoke-memory](assets/skills/invoke-memory/SKILL.md): read relevant saved
  implementation knowledge before work; absent memories need no setup.

The optional **North pipeline** toggle installs these five skills, their
prerequisites, and `/north-plan`, `/north-execute`, and `/north-save` together.
They do not appear as individual checklist options. `/north` is always installed.
Disabling the pipeline preserves project plans and memories.

Engineering methods:

- [clarify-requirements](assets/skills/clarify-requirements/SKILL.md): resolve
  consequential gaps in requested behavior and define concrete acceptance examples.
- [diagnosing-bugs](assets/skills/diagnosing-bugs/SKILL.md): investigate reported
  failures with reproductions, targeted evidence, and checks of the actual symptom.
- [test-design](assets/skills/test-design/SKILL.md): choose behavior checks and
  independent expectations that detect meaningful regressions.
- [code-review](assets/skills/code-review/SKILL.md): review scoped changes against
  requirements, caller behavior, and repository conventions with actionable evidence.
- [architecture-review](assets/skills/architecture-review/SKILL.md): assess
  responsibilities, dependencies, and design tradeoffs within the requested scope.
- [domain-modeling](assets/skills/domain-modeling/SKILL.md): clarify product terms,
  relationships, and business rules in the project's authoritative documentation.
- [research](assets/skills/research/SKILL.md): investigate technical questions using
  primary sources, version context, and cited findings.
- [prototype](assets/skills/prototype/SKILL.md): answer an uncertain design question
  with a bounded, runnable experiment and a recorded outcome.
- [handoff](assets/skills/handoff/SKILL.md): prepare a continuation brief with
  artifact pointers, actual execution state, and the next actions.
- [skill-evaluation](assets/skills/skill-evaluation/SKILL.md): evaluate skill
  selection and observable behavior using isolated, realistic scenarios.

Project context and delivery:

- [explain-code](assets/skills/explain-code/SKILL.md): explain existing code in
  standard technical English with verified line references and symbol definitions.
- [unity-ui](assets/skills/unity-ui/SKILL.md): author Unity UI in the Editor as
  saved, editable assets, with runtime scripts reserved for behavior and data.
- [north-sources](assets/skills/north-sources/SKILL.md): own shared artifact
  locations and the plan contract; included automatically by consuming skills.
- [dry-skillify](assets/skills/dry-skillify/SKILL.md): record recurring user
  preferences as Markdown in `north/dry/` and automatically create scoped skills
  in `north/skills/` after three distinct user-supported observations.
- [subagent-usage](assets/skills/subagent-usage/SKILL.md): assign bounded work,
  isolate implementation in Git worktrees, and merge verified
  results back into the original source branch.
- **Auto commit** is a **Workflow** option for two skills:
  [auto-commit](assets/skills/auto-commit/SKILL.md) commits validated work automatically
  when checked; [commit](assets/skills/commit/SKILL.md) prepares the commit and waits
  for your go-ahead when unchecked. Both use `feat:`, `fix:`, or `chore:` messages.

North reads applicable generated preference skills on later tasks. These records
are project-local; the kit checkout is not a shared store for every project.
Implementation files and explicitly located deliverables keep their required
locations. Rerun `./install.sh` after updating to choose newly added bundled skills.

Engineering skills guide the existing primary agent and subagents. Select the
methods needed for the task; small changes can proceed without an interview,
prototype, or execution plan. Skills reuse existing OpenSpec requirements and
North evidence, and keep the current commit mode and agent permissions in force.
