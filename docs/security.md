# Permissions and ownership

Installation creates symlinks into this checkout. It saves existing instructions
as `AGENTS-backup.md` (OpenCode) or `CLAUDE-backup.md` (Claude Code) before
replacing `AGENTS.md` or `CLAUDE.md`, refuses other conflicting files, and never
overwrites an existing backup. Uninstall removes only matching North links and
restores the saved instructions. Keep the backup and `.north-installation.json`
in the tool's configuration directory until removal. Each tool's installation is
recorded separately, and a state file written for one tool is refused by the other.
Review changes to the linked instructions when updating the checkout: those
changes become available to the tool without another installation step.

Merge mode preserves your instructions file. For OpenCode it combines North's
bundled defaults with existing JSON/JSONC configuration, preserving existing
scalar settings and appending unique array entries, including any future plugin
defaults. For Claude Code it appends one `@import` line to `CLAUDE.md`. Original
and merged contents are saved in `.north-installation.json` with owner-only
permissions; this file may contain private configuration values or your
instructions. Uninstall restores unchanged files exactly or removes matching
North additions while retaining later user edits. Configuration symlinks are
refused, and failed transactions roll back configuration changes together with links.

Each tool applies the permissions in its agent frontmatter: OpenCode uses the
`permission` map, and Claude Code uses `tools`, `disallowedTools`, and
`permissionMode`. Workflow guidance such as file scopes and leaving changes
uncommitted is instructional, not a sandbox. North provides no runtime guardrail
plugin or process isolation. Configure the tool's own permissions for your environment.
