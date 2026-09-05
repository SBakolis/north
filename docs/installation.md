# Installation and Operations

North is distributed as a single executable for macOS and Linux. The agents,
instruction fragments, and OpenCode guardrail plugin it installs are embedded in
that executable; release archives do not require a separate assets directory.

## Supported Platforms

Release archives are built for:

| Operating system | Architectures |
| --- | --- |
| macOS (`darwin`) | `amd64`, `arm64` |
| Linux | `amd64`, `arm64` |

Git and OpenCode must be available on `PATH`. `npx` must also be available when
the optional OpenSpec knowledge component is selected; North invokes it only as
`npx openspec`. `npm` is required when North installs an optional plugin so it
can resolve and pin the package version. North does not install or update these
dependencies.

## Install the Executable

Download the archive and `checksums.txt` for the desired release from the
[GitHub releases page](https://github.com/SBakolis/north/releases). Replace the
example version and platform below with the release being installed.

```sh
VERSION=1.2.3
ARCHIVE="north_${VERSION}_darwin_arm64.tar.gz"
curl -fLO "https://github.com/SBakolis/north/releases/download/v${VERSION}/${ARCHIVE}"
curl -fLO "https://github.com/SBakolis/north/releases/download/v${VERSION}/checksums.txt"
shasum -a 256 -c checksums.txt --ignore-missing
tar -xzf "$ARCHIVE"
install -m 0755 north "$HOME/.local/bin/north"
north version
```

On Linux, use `sha256sum -c checksums.txt --ignore-missing` if `shasum` is not
installed. Ensure `$HOME/.local/bin` is on `PATH`, or choose another user-owned
directory already on `PATH`. Releases currently provide SHA-256 checksums but
are not signed; see [Security](security.md).

## Configure OpenCode

Preview the default installation before writing anything:

```sh
north install --dry-run
```

Install the default components non-interactively:

```sh
north install --non-interactive
```

Defaults include core instructions, no external knowledge provider, and the
parallelization assets. Useful alternatives are:

```sh
north install --non-interactive --no-parallelization
north install --non-interactive --knowledge openspec
north install --non-interactive --agent-source AGENTS.md
north install --non-interactive --agent-source AGENT.md
```

`--agent-source` accepts only a file name, not a path. It is required for a
non-interactive first installation if both OpenCode `AGENTS.md` and legacy
`AGENT.md` exist with different contents. Interactive installation prompts for
the authoritative file in that case. A first interactive install also prompts
for parallelization, the knowledge provider, and each optional plugin.

Optional OpenCode plugins are disabled by default and may be selected explicitly:

```sh
north install --non-interactive --plugin opencode-codex-meter
north install --non-interactive --plugin @sbakolis/open-loop
```

North resolves the package version and invokes `opencode plugin
<module>@<version> --global`, snapshots every candidate configuration before
mutation, and records the resolved version and only registrations it created. Codex
Meter must appear in both a global/server candidate and a TUI candidate; Open
Loop must appear exactly once. Installation fails and rolls back if these
invariants are not met. `OPENCODE_CONFIG` and `OPENCODE_TUI_CONFIG` overrides
are inspected before default candidate paths. North never installs or configures
skills or MCP servers.

Use `north configure` with the same flags to change an existing installation.
It preserves the currently selected components unless the corresponding option
is explicitly supplied. Always preview changes first:

```sh
north configure --dry-run --knowledge none --no-parallelization
north configure --non-interactive --knowledge none --no-parallelization
```

List the components compiled into the executable with:

```sh
north components list
```

## Files and Directories

North honors the XDG environment variables. Defaults are shown below.

| Purpose | Default path |
| --- | --- |
| Generated North configuration | `~/.config/north/config.yaml` |
| Active OpenCode instructions | `~/.config/opencode/AGENTS.md` |
| Installed version assets | `~/.local/share/north/versions/<version>/assets/` |
| Installation manifest | `~/.local/state/north/install-manifest.json` |
| Private transaction snapshots | `~/.local/state/north/backups/` |
| Cache root | `~/.cache/north/` |

Installed OpenCode agent and plugin paths are symlinks to immutable, embedded
asset copies under the version directory. North records hashes and symlink
targets in its manifest. Do not edit generated configuration, managed assets,
managed symlinks, the manifest, or private snapshots. Re-run `north configure`
to make supported changes.

The installer creates an immutable stable backup for each pre-existing
`AGENTS.md` and `AGENT.md`, even when one is not selected as authoritative. It
also records a private snapshot for each original. Reconfiguration merges edits
outside the managed block against the previous generated snapshot. If markers
or managed content conflict, North leaves `AGENTS.md` untouched and writes
`AGENTS.md.north-proposed` for review. Stable backups remain after uninstall.

## Status and Diagnostics

Check whether the manifest exists and every managed path still matches it:

```sh
north status
```

Display the North version, locations of required executables, and installation
health:

```sh
north doctor
north doctor --json
```

The text and JSON forms contain the same structured checks, including executable
versions, required OpenCode flags, plugin registrations, configuration parsing,
stale locks, orphaned stages, and cleanup candidates. Errors produce a non-zero
exit status; advisories do not. OpenSpec diagnosis uses bounded, offline `npx
--no-install` resolution and never downloads a missing package.

## Planning and Orchestration

Validate and approve plans before execution. Approval is bound to the canonical
plan hash; changing any stage, command, scope, or policy invalidates it.

```sh
north plan validate plan.yaml --strict
north plan approve plan.yaml
north plan approve plan.yaml --dry-run
north plan create --goal "Implement the change" --output plan.yaml --dry-run
north graph plan.yaml --format dot
north run plan.yaml
north run status --watch
north integrate <run-id> [--dry-run]
```

`north run` requires a clean checkout and leaves the target branch untouched.
The `north-planner` agent turns goals and normalized knowledge into a validated
execution plan with inferred stages, dependencies, scopes, and acceptance
checks. Workers run in separate worktrees, deterministic acceptance checks gate commits,
and successful commits are progressively cherry-picked into a run-owned
integration branch. Each stage's targeted checks run again in that integration
worktree before the stage releases dependents. Use `--approve-plan` to approve the effective plan at run
time, `--max-parallel N` to override concurrency, `--fail-fast`, or
`--auto-resolve-conflicts` to permit the dedicated resolver workflow.

Operator controls are:

```sh
north run status [<run-id>] [--watch] [--json]
north run stop <run-id> [--dry-run]
north run resume <run-id> [--dry-run]
north stage retry <run-id> <stage-id> [--dry-run]
north stage hold <run-id> <stage-id> [--dry-run]
north stage release <run-id> <stage-id> [--dry-run]
north run <plan-file> --dry-run
north cleanup <run-id> [--dry-run]
```

Final target integration is always explicit unless `north run --auto-integrate`
was supplied. It refuses a dirty checkout, wrong checked-out target, or target
divergence.

## Upgrade

Verify and replace the executable using the release steps above, then preview and
apply configuration so embedded assets are refreshed:

```sh
north configure --dry-run
north configure --non-interactive
north status
```

`north update --dry-run` and `north update` apply the same ownership-safe
configuration transaction using assets embedded in the new executable. Unknown
future manifest schema versions are refused rather than guessed.

## Uninstall

Preview the exact paths North intends to remove or restore:

```sh
north uninstall --dry-run
north uninstall
```

Uninstall removes only paths recorded as North-owned. It refuses to continue if
a managed file, asset, or symlink has changed. If the active instruction file is
unchanged, the original instruction source is restored. If user edits were made
outside the North-managed marker block, only that block is removed. Stable
`AGENTS-backup.md` or `AGENT-backup.md` files and now-empty parent directories are
preserved. Removing the executable itself is a separate manual step.

See [Recovery](recovery.md) before handling an unhealthy installation or an
uninstall refusal.
