#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
if ! command -v cargo >/dev/null 2>&1; then
    printf 'North requires Rust and Cargo (Rust 1.88+). Install them from https://rustup.rs, then rerun %s\n' "$0" >&2
    exit 1
fi

exec cargo run --quiet --locked --manifest-path "$root/installer/Cargo.toml" -- --repo "$root" "$@"
