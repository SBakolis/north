# North

North is a small set of instructions, agent definitions, and skills for OpenCode.
OpenCode provides subagent execution; the primary agent coordinates planning,
implementation, and review.

## Install

Clone this repository to a permanent location, then run:

```sh
./install.sh
```

The script symlinks shared instructions, four subagents, and skill directories into
`${XDG_CONFIG_HOME:-$HOME/.config}/opencode`. It requires a POSIX shell and standard
Unix utilities, creates no runtime state, and does not overwrite existing files.
Keep the checkout in place because the links point directly into it.

Start a new OpenCode session and ask for your change normally. The shared
instructions guide delegation for larger tasks. You can also invoke an agent
explicitly, for example:

```text
@north-planner plan the login change
```

See [installation](docs/installation.md) for existing configurations, updates,
and removal, and [architecture](docs/architecture.md) for the workflow.

## Skills

OpenCode discovers skill descriptions and loads matching guidance on demand.

- [explain-code](assets/skills/explain-code/SKILL.md): explain existing code in
  standard technical English with verified line references and symbol definitions.
- [unity-ui](assets/skills/unity-ui/SKILL.md): author Unity UI in the Editor as
  saved, editable assets, with runtime scripts reserved for behavior and data.
- [north-sources](assets/skills/north-sources/SKILL.md): consult and save North's
  supporting output in each working project's `north/` directory.
- [dry-skillify](assets/skills/dry-skillify/SKILL.md): record recurring user
  preferences as Markdown in `north/dry/` and automatically create scoped skills
  in `north/skills/` after three distinct user-supported observations.

North reads applicable generated preference skills on later tasks. These records
are project-local; the kit checkout is not a shared store for every project.
Implementation files and explicitly located deliverables keep their required
locations. Rerun `./install.sh` after updating to link newly added bundled skills.
