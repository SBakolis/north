#!/bin/sh
set -eu

if [ "$#" -ne 0 ]; then
    printf 'Usage: %s\n' "$0" >&2
    exit 1
fi

root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
config=${XDG_CONFIG_HOME:-"$HOME/.config"}/opencode
case "$config" in
    /*) ;;
    *) printf 'XDG_CONFIG_HOME must be an absolute path\n' >&2; exit 1 ;;
esac

# Check every destination before creating links. Never replace user files.
check_link() {
    if [ -L "$2" ] && [ "$(readlink "$2")" = "$1" ]; then
        return
    fi
    if [ -e "$2" ] || [ -L "$2" ]; then
        printf 'Refusing to replace %s; move it aside or merge it manually first.\n' "$2" >&2
        exit 1
    fi
}

check_link "$root/assets/instructions/core.md" "$config/AGENTS.md"
for source in "$root"/assets/agents/*.md; do
    check_link "$source" "$config/agents/$(basename "$source")"
done
for source in "$root"/assets/skills/*; do
    check_link "$source" "$config/skills/$(basename "$source")"
done

mkdir -p "$config/agents" "$config/skills"
link_file() {
    if [ ! -L "$2" ]; then
        ln -s "$1" "$2"
    fi
    printf '%s -> %s\n' "$2" "$1"
}

link_file "$root/assets/instructions/core.md" "$config/AGENTS.md"
for source in "$root"/assets/agents/*.md; do
    link_file "$source" "$config/agents/$(basename "$source")"
done
for source in "$root"/assets/skills/*; do
    link_file "$source" "$config/skills/$(basename "$source")"
done
