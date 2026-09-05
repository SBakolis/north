# Permissions and ownership

Installation creates symlinks into this checkout. It saves existing instructions
as `AGENTS-backup.md` before replacing `AGENTS.md`, refuses other conflicting
files, and never overwrites an existing backup. Uninstall removes only matching
North links and restores the saved instructions. Keep the backup and
`.north-installation.json` in the OpenCode configuration directory until removal.
Review changes to the linked instructions when updating the checkout: those
changes become available to OpenCode without another installation step.

OpenCode applies the permissions in each agent's frontmatter. Workflow guidance
such as file scopes and leaving changes uncommitted is instructional, not a
sandbox. North provides no runtime guardrail plugin or process isolation.
Configure OpenCode's own permissions for your environment.
