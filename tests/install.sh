#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
export HOME="$tmp/home"
export XDG_CONFIG_HOME="$tmp/config with spaces"
config=$XDG_CONFIG_HOME/opencode

# Install from another working directory, then repeat without modifying links.
cd "$tmp"
sh "$root/install.sh"
sh "$root/install.sh"
[ "$(readlink "$config/AGENTS.md")" = "$root/assets/instructions/core.md" ]
for source in "$root"/assets/agents/*.md; do
    target=$config/agents/$(basename "$source")
    [ "$(readlink "$target")" = "$source" ]
    cmp "$source" "$target"
done
for source in "$root"/assets/skills/*; do
    target=$config/skills/$(basename "$source")
    [ "$(readlink "$target")" = "$source" ]
    cmp "$source/SKILL.md" "$target/SKILL.md"
done

# Existing skill directories must survive, with no partial installation.
rm -rf "$config"
mkdir -p "$config/skills/unity-ui"
printf 'custom skill\n' > "$config/skills/unity-ui/SKILL.md"
if sh "$root/install.sh"; then exit 1; fi
[ ! -e "$config/AGENTS.md" ]
[ ! -d "$config/agents" ]
[ "$(cat "$config/skills/unity-ui/SKILL.md")" = 'custom skill' ]

# Unrelated skills remain intact during a successful install.
rm -rf "$config/skills/unity-ui"
mkdir -p "$config/skills/custom"
printf 'unrelated skill\n' > "$config/skills/custom/SKILL.md"
sh "$root/install.sh"
[ "$(cat "$config/skills/custom/SKILL.md")" = 'unrelated skill' ]

# A late conflict must prevent even the first link from being installed.
rm -rf "$config"
mkdir -p "$config/agents"
printf 'custom agent\n' > "$config/agents/north-worker.md"
if sh "$root/install.sh"; then exit 1; fi
[ ! -e "$config/AGENTS.md" ]
[ "$(cat "$config/agents/north-worker.md")" = 'custom agent' ]

# Broken symlinks and existing instruction files are also preserved.
rm -rf "$config"
mkdir -p "$config"
ln -s "$tmp/missing" "$config/AGENTS.md"
if sh "$root/install.sh"; then exit 1; fi
[ "$(readlink "$config/AGENTS.md")" = "$tmp/missing" ]
rm "$config/AGENTS.md"
printf 'custom instructions\n' > "$config/AGENTS.md"
if sh "$root/install.sh"; then exit 1; fi
[ "$(cat "$config/AGENTS.md")" = 'custom instructions' ]
[ ! -d "$config/agents" ]

# Reject relative XDG roots, and honor HOME when XDG is unset.
export XDG_CONFIG_HOME=relative
if sh "$root/install.sh"; then exit 1; fi
[ ! -e relative ]
unset XDG_CONFIG_HOME
sh "$root/install.sh"
[ -L "$HOME/.config/opencode/AGENTS.md" ]

# Source paths can contain spaces too.
mkdir -p "$tmp/source with spaces"
cp "$root/install.sh" "$tmp/source with spaces/"
cp -R "$root/assets" "$tmp/source with spaces/"
export XDG_CONFIG_HOME="$tmp/other config"
sh "$tmp/source with spaces/install.sh"
cmp "$root/assets/instructions/core.md" "$XDG_CONFIG_HOME/opencode/AGENTS.md"
printf 'Installer checks passed\n'
