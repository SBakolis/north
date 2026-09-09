# Installation

Install OpenCode separately and clone North to a permanent location. Install
Rust and Cargo (Rust 1.88 or newer) from [rustup](https://rustup.rs), then run
`./install.sh` from the checkout, or invoke its absolute path from any directory.
The POSIX shell script builds and launches a small Ratatui installer. The first
build needs internet access to download the locked Rust dependencies; subsequent
runs reuse the build. No North daemon or OpenCode plugin is installed.

## Choose options

The installer separates its options into four categories, in this order:

| Category | Options |
| --- | --- |
| Workflow | Auto commit, North pipeline |
| Skills | Individually selectable engineering and project skills |
| OpenSpec | OpenSpec CLI |
| Installation | Merge installations |

Use Up/Down (or j/k) to move and Space to toggle an option. `a` and `n` select
or clear only the regular **Skills** category, preserving the **Workflow**,
**OpenSpec**, and **Installation** choices. Press Enter to apply or q/Esc to leave
unchanged.
The checklist scrolls with the selection. On first installation, regular skills,
**Auto commit**, and **North pipeline** start checked; **OpenSpec** and
**Installation** options start unchecked. Shared instructions, `/north`, and the four North agents are always
included. See the [skill catalog](../README.md#skills) for each method's purpose.

**North pipeline** installs `/north-plan`, `/north-execute`, and `/north-save`
together with `north-plan`, `north-explore`, `north-execute`, `north-save`, and
`invoke-memory`. These five skills do not appear as individual checklist rows.
The group also requires `clarify-requirements`, `research`, and `subagent-usage`;
the checklist marks automatically included skills as required. These shared
skills remain individually selectable when the pipeline is off.

Disabling **North pipeline** removes its owned command and skill links. Project
plans and memories remain intact. Reruns preserve the group setting. When
upgrading an older installation, the group starts enabled if any owned pipeline
skill is installed, otherwise disabled.
Shared skills already checked in the Skills category stay selected when you
turn the pipeline off; uncheck them separately if they are no longer needed.

**Auto commit** is a single checkbox, checked on first installation. When checked,
the installer links `auto-commit`, which commits completed, validated work without
an extra confirmation. When unchecked, it links `commit`, which prepares the commit
and waits for your go-ahead. Both use `feat:`, `fix:`, or `chore:` messages. Exactly
one mode is linked. The `a` and `n` shortcuts leave this choice unchanged. Reruns
preserve the mode; installations from before commit modes were introduced start
with Auto commit unchecked and add `commit` when you apply. The mode affects
local commits, not pushing to a remote.

The optional **OpenSpec CLI** checkbox starts unchecked. Select it with Space
to install OpenSpec if it is missing.
On apply, the installer checks `openspec --version` and skips installation if
it succeeds. If the command is missing, it checks for Node.js 20.19.0 or newer,
runs `npm install -g @fission-ai/openspec@latest`, and verifies the CLI afterward,
following the [OpenSpec installation instructions](https://openspec.dev/docs/installation).
Node.js and npm must already be on `PATH`, and npm's global directory must be
writable. An existing CLI that fails its version check produces an error rather
than being overwritten.

OpenSpec is installed globally, separately from North's links. The installer does not
initialize OpenSpec in a project, upgrade an existing installation, or remove it
when you uninstall North. If OpenSpec setup fails, the installer exits with an
error and explains that North's changes were saved; fix the reported issue and
retry with `./install.sh --openspec`.

The installer creates symlinks under
`${XDG_CONFIG_HOME:-$HOME/.config}/opencode`:

| Destination | Repository source |
| --- | --- |
| `AGENTS.md` | `assets/instructions/core.md` |
| `commands/north.md` | `assets/commands/north.md` |
| `commands/north-plan.md` (North pipeline enabled) | `assets/commands/north-plan.md` |
| `commands/north-execute.md` (North pipeline enabled) | `assets/commands/north-execute.md` |
| `commands/north-save.md` (North pipeline enabled) | `assets/commands/north-save.md` |
| `agents/north-planner.md` | `assets/agents/north-planner.md` |
| `agents/north-worker.md` | `assets/agents/north-worker.md` |
| `agents/north-verifier.md` | `assets/agents/north-verifier.md` |
| `agents/north-conflict-resolver.md` | `assets/agents/north-conflict-resolver.md` |
| `skills/<name>` (each enabled skill in the catalog) | `assets/skills/<name>/` |
| `skills/auto-commit` (Auto commit checked) | `assets/skills/auto-commit/` |
| `skills/commit` (Auto commit unchecked) | `assets/skills/commit/` |

With **Merge installations** enabled, `AGENTS.md` stays in place; North's shared
instructions are loaded through the configuration's `instructions` array instead.

Each skill directory contains a `SKILL.md`. The installer discovers bundled
skills automatically and links each enabled directory separately. Workflow
options group the pipeline skills and select one of the two commit modes. Unrelated
skills remain intact. Keep the checkout because the links point directly into it.
Start a new OpenCode session to load the installed instructions and skills.

## Scaffold a project

Run `/north` in an OpenCode session for the project. It creates the project's
`north/` directory without changing existing contents. When `openspec --version`
succeeds and the project has no `openspec/` directory, it asks whether to add
OpenSpec with OpenCode support. Accepting runs `openspec init --tools opencode`
from the project root; declining leaves only the North directory. A missing CLI
is skipped, and an existing OpenSpec directory is left untouched.

Rerun the installer after updating North to register the commands in an existing
installation. `/north` is always included; the three pipeline commands require
**North pipeline** to be enabled.

## Use the implementation pipeline

Enable **North pipeline** in **Workflow**, or select it from the command line:

```sh
./install.sh --skills north-pipeline
```

In the working project's OpenCode session, run `/north-plan <your feature prompt>`.
The agent asks relevant requirements questions, checks existing memories, and
researches similar implementations and useful libraries. It creates linked
Markdown files under `north/plans/<feature-slug>/` and suggests
`/north-execute <feature-slug>`. Execution works through independent tasks in
dependency layers, verifies each layer, and suggests `/north-save <feature-slug>`
when implementation is complete. Execute and save can also take a plan path.

Saving records verified implementation facts in `north/memories/`, then suggests
`/north-plan <next feature>`. Future implementation retrieves relevant memories
through `invoke-memory`; a project without memories needs no initialization.
Plans and memories honor a configured North directory. See the
[plan format](plan-format.md) for task files, statuses, and resuming an existing
plan, including plans created with the earlier single-file format.

## Existing instructions and conflicts

The optional **Merge installations** checkbox starts unchecked and remembers your
choice on later runs. Enable it to combine North with an existing OpenCode setup
in `${XDG_CONFIG_HOME:-$HOME/.config}/opencode`. The installer detects `opencode.json`
and `opencode.jsonc`, merges into each existing file, or creates `opencode.json`
if neither exists. It preserves JSONC comments and unrelated formatting.

North's defaults come from `assets/opencode.json`; the installer also adds the
absolute path to North's shared instructions. Objects merge recursively, missing
settings are added, and arrays (including `plugin` and `instructions`) are extended
with unique entries. Existing scalar values win conflicts. This supports future
bundled plugins by adding their entries to the `plugin` array in North's config;
no plugins are bundled yet. OpenCode loads configured plugins itself.

Merge mode leaves your `AGENTS.md` active alongside North's instructions, as
described in [OpenCode's custom instructions documentation](https://opencode.ai/docs/rules/#custom-instructions).
Switching an existing North installation to merge mode restores its saved
`AGENTS-backup.md`. Unchecking merge removes North's config additions and switches
back to the instruction link and backup behavior described below.

Configuration changes are part of the installation transaction. Invalid JSON/JSONC,
duplicate object keys, incompatible `instructions`/`plugin` types, and configuration
symlinks or directory conflicts stop the merge before changes are applied. The
installer saves original and merged text in `.north-installation.json` with
owner-only permissions. Keep this state file for updates and uninstall; it can
contain private settings from your configuration.

Reruns update North's additions without accumulating duplicates. Uninstall restores
an untouched config exactly, including comments and formatting, or deletes a config
created solely for North. If you edit it afterward, uninstall removes only matching
North additions and retains your later settings and plugins. Comment-only edits
also retain the file. Existing agent, command, and skill filename conflicts still
follow the checks below.

Without merge mode, before linking North's instructions, the installer renames an existing
`AGENTS.md` to `AGENTS-backup.md` in the same OpenCode configuration directory.
This preserves the original file, or the original symlink including its target.
North's instructions are active while installed; the backup is retained for
uninstall and is never overwritten on reruns. If no original `AGENTS.md` exists,
no backup is needed.

All link destinations are checked before modifying instructions or skills.
The installer refuses conflicts at enabled command, agent, and skill destinations,
including dangling links. Disable the relevant skill or workflow option to leave
it alone, or move the conflicting file aside yourself. An existing untracked
`AGENTS-backup.md` also blocks installation so it cannot be overwritten.
Symlinks in place of the OpenCode, commands, agents, or skills directory are refused.
An untracked or user-replaced commit skill also blocks switching to the opposite
mode, so both modes cannot accidentally remain active. Move that conflict aside
before switching; the installer will not delete it.

The installer records link ownership and whether it created a backup in
`.north-installation.json`. Keep this file and `AGENTS-backup.md` until uninstall.
A lock prevents simultaneous installers from changing the same installation.
Detected failures roll back completed configuration, link, and backup changes. If a process is
forcibly terminated, inspect the links, backup, and state before retrying; remove
a stale `.north-install.lock` directory only when no installer is running.

## Change skills, update, and uninstall

Run `./install.sh` again to open the checklist with the saved workflow and skill
choices. Toggle options and press Enter to link or unlink them. Updating this
checkout changes the contents of linked assets immediately. Newly added regular
skills start unchecked on an existing installation unless required by the enabled
pipeline; rerun the installer to enable them.
If the checkout moves, run its `install.sh` from the new location to update links
using the saved installation state.

Press `u` in the installed menu, then `y`, to uninstall. North removes its matching
instruction, command, agent, and skill links, restores `AGENTS-backup.md` to `AGENTS.md`
when present, and removes its installation state. With no original instructions,
`AGENTS.md` is simply removed. The checkout, project plans and memories, unrelated
skills, user replacements for agent/skill links, and other OpenCode configuration remain.
You can delete the checkout yourself afterward.

If you replaced North's `AGENTS.md` link with your own file, move that file aside
before uninstalling so North can restore the original backup. A missing saved
backup also blocks changes until you restore it; the installer will not silently
claim to have restored instructions it cannot find.

Matching links from the previous shell-only installer are recognized without a
state file. That installer did not replace existing instructions or create a
backup, so there are no original instructions to restore for those installations.
For installations made with the older North CLI, use that version's uninstall
procedure first, retaining any user instructions and unfinished work.

## Unattended use

An interactive terminal is required by default. Scripts and CI can explicitly
select an action using the same installation logic:

```sh
./install.sh --all                         # All skills, Auto commit, and North pipeline
./install.sh --all --openspec              # Also install OpenSpec if missing
./install.sh --openspec                    # Keep skill selection; ensure OpenSpec
./install.sh --merge                       # Keep skills; merge existing OpenCode config
./install.sh --all --merge --openspec       # Merge config and also ensure OpenSpec
./install.sh --skills explain-code,unity-ui # These skills plus confirmation-based commit
./install.sh --skills explain-code,auto-commit # Explain code with Auto commit enabled
./install.sh --skills north-pipeline       # Pipeline and confirmation-based commit
./install.sh --skills north-pipeline,auto-commit # Pipeline with Auto commit
./install.sh --skills ''                   # Only commit; keep /north, instructions, agents
./install.sh --uninstall                   # Remove North and restore the backup
./install.sh --help
```

`--openspec` and `--merge` can be combined with each other, `--all`, or `--skills`,
but not `--uninstall`. Either used alone preserves the current skill selection
(all skills on first install). Once enabled, merge mode persists for unattended
updates; uncheck it in the TUI to return to the default installation mode.
For `--skills`, including `auto-commit` checks Auto commit; omitting it selects
`commit`. Explicit `commit` is also accepted; requesting both modes is an error.
`--skills` is an explicit selection: include `north-pipeline` to enable the entire
group and its prerequisites. The older individual names `north-plan`,
`north-explore`, `north-execute`, `north-save`, and `invoke-memory` are accepted
as aliases that enable the full group. `--all` enables all regular skills,
Auto commit, and North pipeline; it does not enable merge mode on first
installation or install OpenSpec.

Without merge mode, the installer leaves OpenCode JSON configuration untouched. OpenCode documents
[agent discovery](https://opencode.ai/docs/agents/),
[skills](https://opencode.ai/docs/skills/), and
[global instructions](https://opencode.ai/docs/rules/).
