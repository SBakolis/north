# Installation

Install OpenCode separately and clone North to a permanent location. Run
`./install.sh` from the checkout, or invoke its absolute path from any directory.
No Go toolchain, North executable, package installation, or build is needed.

The installer creates these symlinks under
`${XDG_CONFIG_HOME:-$HOME/.config}/opencode`:

| Destination | Repository source |
| --- | --- |
| `AGENTS.md` | `assets/instructions/core.md` |
| `agents/north-planner.md` | `assets/agents/north-planner.md` |
| `agents/north-worker.md` | `assets/agents/north-worker.md` |
| `agents/north-verifier.md` | `assets/agents/north-verifier.md` |
| `agents/north-conflict-resolver.md` | `assets/agents/north-conflict-resolver.md` |
| `skills/explain-code` | `assets/skills/explain-code/` |
| `skills/unity-ui` | `assets/skills/unity-ui/` |

Each skill directory contains a `SKILL.md` with a name, discovery description,
and instructions. The installer links each directory separately, preserving
unrelated skills. Skill identifiers use lowercase names as required by OpenCode.

Running the installer again leaves matching symlinks in place. It checks all
file destinations before linking and refuses existing files, directories, or
links to other targets, including broken links. Move conflicting files aside
and preserve any custom instructions before rerunning. If you keep a custom
`AGENTS.md`, combine its contents with North's instructions in the linked source
before installing. Such local edits must be reconciled when updating North.

The installer does not edit `opencode.json`, install plugins, or migrate old
North installations. For an installation made with the former CLI, use that
version's uninstall procedure before switching, retaining any user instructions
and unfinished work. Old run state is not consumed by this version.

## Update and remove

Update this checkout to the desired revision; the symlinks use its current
contents. Rerun `./install.sh` to link any newly added agents or skills. Start a
new OpenCode session to load the updated instructions. If you
move the checkout, remove its old links and rerun `install.sh` from the new path.

To uninstall, remove only the links listed above after checking that they
point into this checkout. Other OpenCode files and configuration stay in place.
Restore any instructions you moved aside when installing.

OpenCode documents [agent discovery](https://opencode.ai/docs/agents/),
[skills](https://opencode.ai/docs/skills/), and
[global instructions](https://opencode.ai/docs/rules/).
