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
Up/Down and Space to choose the starting skills, then Enter to install shared
instructions, four subagents, and the selected skills into
`${XDG_CONFIG_HOME:-$HOME/.config}/opencode`.

Your existing `AGENTS.md` is saved as `AGENTS-backup.md`. Run `./install.sh` again
to enable or disable skills, or press `u` to uninstall North and restore that
backup. Keep the checkout in place because the installed links point into it.

Start a new OpenCode session and ask for your change normally. The shared
instructions guide delegation for larger tasks. You can also invoke an agent
explicitly, for example:

```text
@north-planner plan the login change
```

See [installation](docs/installation.md) for existing configurations, updates,
and removal, and [architecture](docs/architecture.md) for the workflow.

For larger changes, the primary agent saves an [execution plan](docs/plan-format.md)
in the working project's `north/plans/` directory, dispatches independent tasks
in parallel, and records verification before starting dependent work. Plans
preserve context for resuming work; coordination runs through agent instructions
and OpenCode's native subagents.

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
locations. Rerun `./install.sh` after updating to choose newly added bundled skills.
