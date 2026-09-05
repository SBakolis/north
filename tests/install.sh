#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
tmp=$(mktemp -d)
tmp=$(CDPATH= cd -- "$tmp" && pwd -P)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
cargo build --quiet --locked --manifest-path "$root/installer/Cargo.toml"
binary=${CARGO_TARGET_DIR:-"$root/installer/target"}/debug/north-installer
config_root="$tmp/config with spaces"
config=$config_root/opencode
run() {
    env XDG_CONFIG_HOME="$config_root" "$binary" --repo "$root" "$@"
}
fail() {
    if run "$@"; then
        printf 'Expected failure: %s\n' "$*" >&2
        exit 1
    fi
}

# The shell entry point works from another directory and forwards its arguments.
cd "$tmp"
env XDG_CONFIG_HOME="$config_root" sh "$root/install.sh" --all
run --all
[ "$(readlink "$config/AGENTS.md")" = "$root/assets/instructions/core.md" ]
for source in "$root"/assets/agents/*.md; do
    [ "$(readlink "$config/agents/$(basename "$source")")" = "$source" ]
done
for source in "$root"/assets/commands/*.md; do
    [ "$(readlink "$config/commands/$(basename "$source")")" = "$source" ]
done
for source in "$root"/assets/skills/*; do
    [ "$(readlink "$config/skills/$(basename "$source")")" = "$source" ]
done

# Reruns link and unlink exactly the requested skills.
run --skills explain-code
[ -L "$config/skills/explain-code" ]
[ ! -e "$config/skills/unity-ui" ]
run --skills ''
[ ! -e "$config/skills/explain-code" ]
[ -L "$config/commands/north.md" ]
run --skills unity-ui
[ -L "$config/skills/unity-ui" ]
fail --skills unknown-skill
[ -L "$config/skills/unity-ui" ]
fail --all --uninstall
fail < /dev/null
run --uninstall
[ ! -e "$config/AGENTS.md" ]
[ ! -e "$config/commands/north.md" ]
[ ! -e "$config/.north-installation.json" ]
run --uninstall

# Preserve the exact original instructions through repeated configuration changes.
printf 'original instructions\n' > "$config/AGENTS.md"
run --all
[ "$(cat "$config/AGENTS-backup.md")" = 'original instructions' ]
run --skills explain-code
[ "$(cat "$config/AGENTS-backup.md")" = 'original instructions' ]
mkdir -p "$config/skills/custom"
printf 'custom skill\n' > "$config/skills/custom/SKILL.md"
# A user replacement for an owned skill is never removed by uninstall.
rm "$config/skills/explain-code"
mkdir "$config/skills/explain-code"
printf 'replacement\n' > "$config/skills/explain-code/SKILL.md"
run --uninstall
[ "$(cat "$config/AGENTS.md")" = 'original instructions' ]
[ ! -e "$config/AGENTS-backup.md" ]
[ "$(cat "$config/skills/custom/SKILL.md")" = 'custom skill' ]
[ "$(cat "$config/skills/explain-code/SKILL.md")" = 'replacement' ]

# Preflight catches late conflicts before touching AGENTS.md or its backup.
rm -rf "$config"
mkdir -p "$config/skills/unity-ui"
printf 'original\n' > "$config/AGENTS.md"
printf 'custom\n' > "$config/skills/unity-ui/SKILL.md"
fail --all
[ "$(cat "$config/AGENTS.md")" = 'original' ]
[ ! -e "$config/AGENTS-backup.md" ]
[ ! -e "$config/agents" ]
# A disabled conflicting skill can be left in place.
run --skills explain-code
[ "$(cat "$config/skills/unity-ui/SKILL.md")" = 'custom' ]
run --uninstall

# An existing backup is never overwritten, including a dangling symlink.
ln -s "$tmp/missing-backup" "$config/AGENTS-backup.md"
fail --skills ''
[ "$(readlink "$config/AGENTS-backup.md")" = "$tmp/missing-backup" ]
rm "$config/AGENTS-backup.md"

# Original symlinks (even dangling ones) are restored as symlinks.
rm "$config/AGENTS.md"
ln -s ../missing-original "$config/AGENTS.md"
run --skills ''
[ "$(readlink "$config/AGENTS-backup.md")" = '../missing-original' ]
run --uninstall
[ "$(readlink "$config/AGENTS.md")" = '../missing-original' ]
rm "$config/AGENTS.md"

# Changed instructions or missing backups block restoration without partial removal.
printf 'original\n' > "$config/AGENTS.md"
run --skills explain-code
rm "$config/AGENTS.md"
printf 'new user instructions\n' > "$config/AGENTS.md"
fail --uninstall
[ -L "$config/agents/north-worker.md" ]
[ "$(cat "$config/AGENTS-backup.md")" = 'original' ]
rm "$config/AGENTS.md"
mv "$config/AGENTS-backup.md" "$tmp/saved-backup"
fail --uninstall
fail --skills ''
[ -L "$config/skills/explain-code" ]
mv "$tmp/saved-backup" "$config/AGENTS-backup.md"
run --uninstall
[ "$(cat "$config/AGENTS.md")" = 'original' ]

# Protect directory conflicts and prevent traversing skill-directory symlinks.
rm -rf "$config"
mkdir -p "$config/AGENTS.md"
fail --all
rmdir "$config/AGENTS.md"
mkdir "$tmp/foreign-skills"
ln -s "$tmp/foreign-skills" "$config/skills"
fail --all
[ ! -e "$tmp/foreign-skills/explain-code" ]
rm "$config/skills"
mkdir -p "$config/agents"
printf 'custom agent\n' > "$config/agents/north-worker.md"
fail --all
[ ! -e "$config/AGENTS.md" ]

# Command conflicts fail before changing instructions; unrelated commands survive.
rm -rf "$config"
mkdir -p "$config/commands"
printf 'original\n' > "$config/AGENTS.md"
printf 'custom north\n' > "$config/commands/north.md"
fail --all
[ "$(cat "$config/AGENTS.md")" = original ]
[ ! -e "$config/AGENTS-backup.md" ]
[ "$(cat "$config/commands/north.md")" = 'custom north' ]
mv "$config/commands/north.md" "$config/commands/custom.md"
ln -s "$tmp/missing-command" "$config/commands/north.md"
fail --all
rm "$config/commands/north.md"
run --all
# A user replacement of an installed command is preserved during uninstall.
rm "$config/commands/north.md"
printf 'replacement\n' > "$config/commands/north.md"
run --uninstall
[ "$(cat "$config/commands/north.md")" = replacement ]
[ "$(cat "$config/commands/custom.md")" = 'custom north' ]
mv "$config/commands" "$tmp/foreign-commands"
ln -s "$tmp/foreign-commands" "$config/commands"
fail --all
[ "$(cat "$config/AGENTS.md")" = original ]
[ ! -e "$config/AGENTS-backup.md" ]
rm "$config/commands"

# Reject relative XDG roots, and honor HOME when XDG is unset or empty.
if env XDG_CONFIG_HOME=relative "$binary" --repo "$root" --all; then exit 1; fi
[ ! -e relative ]
env -u XDG_CONFIG_HOME HOME="$tmp/home" "$binary" --repo "$root" --all
[ -L "$tmp/home/.config/opencode/AGENTS.md" ]
env XDG_CONFIG_HOME='' HOME="$tmp/home" "$binary" --repo "$root" --uninstall
[ ! -e "$tmp/home/.config/opencode/AGENTS.md" ]

# Source paths with spaces and relocation are supported using tracked old targets.
mkdir "$tmp/source with spaces"
cp -R "$root/assets" "$tmp/source with spaces/"
config_root="$tmp/relocation config"
config=$config_root/opencode
run --all
env XDG_CONFIG_HOME="$config_root" "$binary" --repo "$tmp/source with spaces" --skills explain-code
[ "$(readlink "$config/skills/explain-code")" = "$tmp/source with spaces/assets/skills/explain-code" ]
[ "$(readlink "$config/commands/north.md")" = "$tmp/source with spaces/assets/commands/north.md" ]
env XDG_CONFIG_HOME="$config_root" "$binary" --repo "$tmp/source with spaces" --uninstall
[ ! -e "$config/AGENTS.md" ]

# Links left by the original shell installer can be managed without a state file.
mkdir -p "$config/skills"
ln -s "$root/assets/instructions/core.md" "$config/AGENTS.md"
ln -s "$root/assets/skills/unity-ui" "$config/skills/unity-ui"
run --skills explain-code
[ ! -e "$config/skills/unity-ui" ]
run --uninstall
[ ! -e "$config/AGENTS.md" ]
printf 'Installer checks passed\n'
