# Installation

Install OpenCode separately and clone North to a permanent location. Install
Rust and Cargo (Rust 1.88 or newer) from [rustup](https://rustup.rs), then run
`./install.sh` from the checkout, or invoke its absolute path from any directory.
The POSIX shell script builds and launches a small Ratatui installer. The first
build needs internet access to download the locked Rust dependencies; subsequent
runs reuse the build. No North daemon or OpenCode plugin is installed.

## Choose skills

On first installation, all skill options start checked. Use Up/Down (or j/k)
to move, Space to enable or disable a skill, `a` to select all, and `n` to select
none. Press Enter to apply or q/Esc to leave without changing your installation.
Shared instructions, the `/north` command, and the four North agents are always included.

**Auto commit** is a single checkbox, checked on first installation. When checked,
the installer links `auto-commit`, which commits completed, validated work without
an extra confirmation. When unchecked, it links `commit`, which prepares the commit
and waits for your go-ahead. Both use `feat:`, `fix:`, or `chore:` messages. Exactly
one mode is linked, including when `n` clears all options. Reruns preserve the mode;
upgrading an older installation starts with Auto commit unchecked and adds `commit`
when you apply. The mode affects local commits, not pushing to a remote.

The optional **OpenSpec CLI** checkbox starts unchecked. Select it with Space
to install OpenSpec if it is missing. The `a` and `n` shortcuts affect skills only.
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
| `agents/north-planner.md` | `assets/agents/north-planner.md` |
| `agents/north-worker.md` | `assets/agents/north-worker.md` |
| `agents/north-verifier.md` | `assets/agents/north-verifier.md` |
| `agents/north-conflict-resolver.md` | `assets/agents/north-conflict-resolver.md` |
| `skills/explain-code` (if enabled) | `assets/skills/explain-code/` |
| `skills/unity-ui` (if enabled) | `assets/skills/unity-ui/` |
| `skills/north-sources` (if enabled) | `assets/skills/north-sources/` |
| `skills/dry-skillify` (if enabled) | `assets/skills/dry-skillify/` |
| `skills/subagent-usage` (if enabled) | `assets/skills/subagent-usage/` |
| `skills/auto-commit` (Auto commit checked) | `assets/skills/auto-commit/` |
| `skills/commit` (Auto commit unchecked) | `assets/skills/commit/` |

Each skill directory contains a `SKILL.md`. The installer discovers bundled
skills automatically and links each enabled directory separately, treating the
two commit skills as mutually exclusive modes of one option. Unrelated
skills remain intact. Keep the checkout because the links point directly into it.
Start a new OpenCode session to load the installed instructions and skills.

## Scaffold a project

Run `/north` in an OpenCode session for the project. It creates the project's
`north/` directory without changing existing contents. When `openspec --version`
succeeds and the project has no `openspec/` directory, it asks whether to add
OpenSpec with OpenCode support. Accepting runs `openspec init --tools opencode`
from the project root; declining leaves only the North directory. A missing CLI
is skipped, and an existing OpenSpec directory is left untouched.

Rerun the installer after updating North to register the command in an existing
installation. The command is included even when all optional skills are disabled.

## Existing instructions and conflicts

Before linking North's instructions, the installer renames an existing
`AGENTS.md` to `AGENTS-backup.md` in the same OpenCode configuration directory.
This preserves the original file, or the original symlink including its target.
North's instructions are active while installed; the backup is retained for
uninstall and is never overwritten on reruns. If no original `AGENTS.md` exists,
no backup is needed.

All link destinations are checked before modifying instructions or skills.
The installer refuses conflicting command files, agent files, and enabled skill destinations,
including dangling links. Disable a conflicting skill to leave it alone, or
move the conflicting file aside yourself. An existing untracked
`AGENTS-backup.md` also blocks installation so it cannot be overwritten.
Symlinks in place of the OpenCode, commands, agents, or skills directory are refused.
An untracked or user-replaced commit skill also blocks switching to the opposite
mode, so both modes cannot accidentally remain active. Move that conflict aside
before switching; the installer will not delete it.

The installer records link ownership and whether it created a backup in
`.north-installation.json`. Keep this file and `AGENTS-backup.md` until uninstall.
A lock prevents simultaneous installers from changing the same installation.
Detected failures roll back completed link and backup changes. If a process is
forcibly terminated, inspect the links, backup, and state before retrying; remove
a stale `.north-install.lock` directory only when no installer is running.

## Change skills, update, and uninstall

Run `./install.sh` again to open the same checklist with currently enabled skills
checked. Toggle skills and press Enter to link or unlink them. Updating this
checkout changes the contents of linked assets immediately. Newly added skills
start unchecked on an existing installation; rerun the installer to enable them.
If the checkout moves, run its `install.sh` from the new location to update links
using the saved installation state.

Press `u` in the installed menu, then `y`, to uninstall. North removes its matching
instruction, command, agent, and skill links, restores `AGENTS-backup.md` to `AGENTS.md`
when present, and removes its installation state. With no original instructions,
`AGENTS.md` is simply removed. The checkout, project plans, unrelated skills,
user replacements for agent/skill links, and other OpenCode configuration remain.
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
./install.sh --all                         # Enable all options, including Auto commit
./install.sh --all --openspec              # Also install OpenSpec if missing
./install.sh --openspec                    # Keep skill selection; ensure OpenSpec
./install.sh --skills explain-code,unity-ui # These skills plus confirmation-based commit
./install.sh --skills explain-code,auto-commit # Explain code with Auto commit enabled
./install.sh --skills ''                   # Only commit; keep shared instructions/agents
./install.sh --uninstall                   # Remove North and restore the backup
./install.sh --help
```

`--openspec` can be combined with `--all` or `--skills`, but not `--uninstall`.
For `--skills`, including `auto-commit` checks Auto commit; omitting it selects
`commit`. Explicit `commit` is also accepted; requesting both modes is an error.
Used alone on first installation, it enables all bundled skills. `--all` by
itself only selects bundled skills and does not install OpenSpec.

The installer does not edit `opencode.json` or install plugins. OpenCode documents
[agent discovery](https://opencode.ai/docs/agents/),
[skills](https://opencode.ai/docs/skills/), and
[global instructions](https://opencode.ai/docs/rules/).
