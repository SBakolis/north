# Recovery and Safety

North treats the installation manifest as the ownership boundary. Prefer its
diagnostic and lifecycle commands over deleting files manually.

## First Response

Stop OpenCode processes that may be reading or changing its configuration, then
collect diagnostics without mutation:

```sh
north version
north doctor
north doctor --json
north status
north uninstall --dry-run
```

Before attempting repair, copy these locations if they exist:

- `${XDG_CONFIG_HOME:-$HOME/.config}/opencode/`
- `${XDG_CONFIG_HOME:-$HOME/.config}/north/`
- `${XDG_DATA_HOME:-$HOME/.local/share}/north/`
- `${XDG_STATE_HOME:-$HOME/.local/state}/north/`

Keep file permissions and symlinks when making that incident copy. The state
directory can contain private copies of user instructions and should not be
published in logs, issues, or support bundles.

## Failed Install or Configure

Within a normal failed invocation, North attempts to restore every file and
symlink captured by its in-memory transaction. Directories created as parents
may remain. This rollback is best effort: process termination, power loss, a full
filesystem, or permission failures can leave partial state.

After a failure:

1. Run `north status` and preserve the error output.
2. Correct external causes such as missing `git`, `opencode`, or `openspec`, bad
   permissions, insufficient disk space, or conflicting user-owned destinations.
3. Run `north install --dry-run` for a first install or `north configure --dry-run`
   for an existing manifest.
4. Re-run the corresponding command only when the preview is expected.
5. Confirm with `north status`.

North refuses to replace an existing agent or plugin destination that is not
recorded in its manifest. Move or rename that user-owned path yourself only after
confirming its provenance; do not delete it merely to make installation pass.

## Unhealthy or Modified Managed Files

`north status` reports missing managed paths, changed file content, and symlinks
whose type or target changed. `north uninstall` also verifies these paths and
refuses an unsafe removal rather than deleting ambiguous content.

Preserve the changed path, compare it with the manifest, and decide whether it is
user data or disposable generated content. If it is disposable, move it outside
the OpenCode and North directories and run `north configure --dry-run` followed
by `north configure`. Do not edit manifest hashes to suppress the ownership
check. If the changed path contains work you need, keep the incident copy and do
not let North overwrite it.

## Recover User Instructions

On first install, North records every existing instruction source and creates a
stable backup beside the OpenCode instructions when one does not already exist:

- `~/.config/opencode/AGENTS-backup.md` for `AGENTS.md`
- `~/.config/opencode/AGENT-backup.md` for legacy `AGENT.md`

The exact paths are in `install-manifest.json` under `instructions`. Private,
content-addressed snapshots are under the North state `backups` directory. Stable
backups are never refreshed or removed automatically, so verify their contents
and date before relying on them.

If reconfiguration reports an instruction conflict, compare
`AGENTS.md.north-proposed` with the active file. North does not change the active
file, manifest, components, or plugins until the conflict is resolved.

The safest normal restoration is `north uninstall --dry-run` followed by
`north uninstall`. If uninstall refuses or the manifest is unreadable:

1. Stop OpenCode and copy the entire configuration and North state directories.
2. Inspect the active `AGENTS.md`, stable backup, and manifest without editing
   them in place.
3. Create a new recovery file from the desired user content. Remove a North block
   only when both `<!-- NORTH:BEGIN managed; schema=1 -->` and
   `<!-- NORTH:END managed -->` are present in the correct order.
4. Atomically replace `AGENTS.md` only after reviewing the recovered file.
5. Leave the original files and state copy intact until OpenCode starts correctly.

There is no CLI command to restore an arbitrary private snapshot. Manual recovery
from those snapshots is an operator action and requires matching the path recorded
in the manifest; snapshot file names alone are not a complete restore procedure.

## Interrupted Orchestration

Run and stage snapshots are written atomically under the North state directory;
events are appended as sequenced NDJSON. After an interrupted process, inspect
state before mutation:

```sh
north run status <run-id> --json
north run resume <run-id>
```

Resume reclaims a persisted lock only when its recorded owner PID is no longer
alive on the same host. North also refuses resume while a stage's persisted
worker PID is still alive. Interrupted preparation, execution, and verification
states are scheduled for repair; `CommitReady` resumes host integration without
rerunning the worker. An interrupted merge becomes `NeedsHumanReview` because
merge completion cannot be inferred safely.

Request cancellation with `north run stop <run-id>`. The running scheduler polls
the durable cancellation intent, stops launching stages, and terminates active
worker contexts. A dead scheduler leaves that intent for resume logic.

Use `north stage retry`, `hold`, and `release` for explicit stage control. Use
`north cleanup <run-id> --dry-run` before cleanup. Cleanup refuses active runs,
unknown worktrees, and dirty worktrees; it never resets or cleans them forcibly.
After worktrees are removed, cleanup deletes only the exact run-owned stage and
integration branches recorded in validated state.
Merge conflicts remain explicit unless automatic conflict resolution was
approved for the plan.

## Escalation Data

For a reproducible report, include the operating system and architecture, `north
version`, textual `north doctor` output, and redacted `north doctor --json`
output. Do not include repository secrets, environment variables, full OpenCode
instructions, North private snapshots, or authentication files.
