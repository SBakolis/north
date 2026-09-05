# Permissions and ownership

Installation creates symlinks into this checkout. It saves existing instructions
as `AGENTS-backup.md` before replacing `AGENTS.md`, refuses other conflicting
files, and never overwrites an existing backup. Uninstall removes only matching
North links and restores the saved instructions. Keep the backup and
`.north-installation.json` in the OpenCode configuration directory until removal.
Review changes to the linked instructions when updating the checkout: those
changes become available to OpenCode without another installation step.

Merge mode preserves `AGENTS.md` and combines North's bundled defaults with
existing OpenCode JSON/JSONC configuration. It preserves existing scalar settings
and appends unique array entries, including any future plugin defaults. Original
and merged configurations are saved in `.north-installation.json` with owner-only
permissions; this file may contain private configuration values. Uninstall restores
unchanged files exactly or removes matching North additions while retaining later
user edits. Configuration symlinks are refused, and failed transactions roll back
configuration changes together with links.

OpenCode applies the permissions in each agent's frontmatter. Workflow guidance
such as file scopes and leaving changes uncommitted is instructional, not a
sandbox. North provides no runtime guardrail plugin or process isolation.
Configure OpenCode's own permissions for your environment.
