# OpenSpec Knowledge Provider

The `internal/knowledge/openspec` adapter is a read-only bridge from OpenSpec to
North's `knowledge.Snapshot`. It never initializes, updates, archives, or writes
an OpenSpec project.

## Detection

Detection walks from the supplied project path toward the filesystem root and
selects the nearest `openspec/config.yaml`. It then verifies the integration by
running `npx openspec --version`. `Provider.Detection` reports the selected root,
config path, executable form, version, and any current diagnostics. A later
absent detection clears stale details and diagnostics.

## Loading

Loading invokes these machine-readable, non-mutating commands from the detected
root:

```text
npx openspec status --change <id> --json
npx openspec validate <id> --type change --json
npx openspec show <id> --type change --json
```

Status output controls artifact names, dependency order, completion, and concrete
paths. Consequently custom schemas do not need conventional `proposal.md`,
`design.md`, or `tasks.md` names. Show enriches conventional changes with parsed
delta information; it is optional for a custom schema without a proposal.

Only completed artifacts are read. Every concrete path is canonicalized and must
remain beneath the detected project root, including through symlinks. The
snapshot records root-relative source paths, line anchors for normalized entries,
and a SHA-256 hash for every raw artifact. Requirements, scenarios, design
decisions, and checkbox tasks are extracted from semantic Markdown headings and
task syntax. Task completion is preserved.

Command execution is injectable with `Runner` or `RunnerFunc`; production uses
`ExecRunner`. The runner contract receives the working directory, executable,
and argument vector, making command assertions deterministic in tests.
