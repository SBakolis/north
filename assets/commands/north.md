---
description: Scaffold North in the current project, with optional OpenSpec setup
agent: build
subtask: false
---

Scaffold North in the current working project. Perform this small setup directly.

1. Resolve the current project's root from the repository or established
   workspace, even when invoked from a subdirectory. Use the working project,
   not the checkout containing this command. A project named `north` still uses
   `<project-root>/north/`. Honor an explicitly configured North output directory.
2. Create that North directory if it is missing. Preserve an existing directory
   and all its contents. If the path is a file or a symlink, report the conflict
   without replacing it or writing through it. For now, create only the directory;
   do not generate plans, templates, configuration, or placeholder files.
3. Inspect `<project-root>/openspec/`. If it already exists, report that OpenSpec
   is already present and leave it untouched; do not reinitialize it. If that
   path is a file or a symlink, report the conflict and skip OpenSpec setup.
4. Otherwise, check whether `openspec` is on PATH and run `openspec --version`.
   If it is missing, report that OpenSpec setup was skipped and finish. Do not
   install the CLI or invoke a package runner to fetch it. If the version check
   fails or returns no version, report the error and skip OpenSpec setup.
5. When the CLI is available and the project has no OpenSpec directory, ask:
   "OpenSpec is installed. Add OpenSpec to this project with OpenCode support?"
   State the resolved project path and that this will run
   `openspec init --tools opencode` there, creating OpenSpec project files and
   OpenCode integration files. Wait for an explicit answer. Declining or leaving
   the question unanswered keeps the North directory and skips initialization.
6. Only after the user accepts, run `openspec init --tools opencode` with the
   working directory set to the resolved project root. Never use `--force`.
   Check the exit status and resulting OpenSpec directory before reporting
   success. On failure, report the error and retain the North directory; do not
   delete any partially created files or retry with destructive options.

Finish with the North directory's path and whether OpenSpec was added, already
present, declined, unavailable, or failed.
