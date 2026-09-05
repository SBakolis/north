# OpenSpec context

When a project uses OpenSpec, agents read its project instructions and relevant
requirements, design, and task artifacts directly. Include the applicable paths
and constraints when delegating to subagents.

The North installer can optionally install the OpenSpec CLI. In a working
project, `/north` creates the North directory and checks for an existing
OpenSpec setup. If none exists and the CLI is available, it asks before running
`openspec init --tools opencode` in that project. It skips initialization when
the CLI is missing or the user declines, and preserves existing OpenSpec files.
See the [OpenSpec CLI reference](https://github.com/Fission-AI/OpenSpec/blob/main/docs/cli.md)
for initialization options.

North does not normalize OpenSpec artifacts, run an adapter, or update change
status automatically. Follow the project's OpenSpec workflow and keep
modifications within the user's requested scope.
