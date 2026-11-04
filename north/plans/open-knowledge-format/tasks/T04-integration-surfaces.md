---
id: T04
status: done
depends_on: [T02, T03]
agent: north-worker
write_scope: [assets/opencode/instructions/core.md, assets/claude/instructions/core.md, assets/opencode/commands/, assets/claude/commands/, assets/opencode/agents/, assets/claude/agents/, README.md, docs/, installer/src/install.rs, tests/install.sh]
---

# T04: Align tool instructions, docs, and installation checks

Plan: [Open Knowledge Format](../index.md)
Prerequisites: [T02](T02-pipeline.md), [T03](T03-supporting-skills.md).
Research: [Related surfaces](../research.md)

## Outcome

Expose consistent OKF guidance to OpenCode and Claude Code, including standalone artifact skills, and document new and legacy behavior accurately.

## Acceptance

- Core instructions route artifact writing to the shared profile with a concise compatible fallback when unavailable. Preserve configured roots, primary ownership and native frontmatter.
- Audit scoped commands/agents for obsolete index/status instructions and change only contradictions; `/north` still creates only the directory, and pipeline commands do not automatically invoke the next phase.
- README/architecture/plan guide explain generated-artifact scope, new layout, legacy support, relative links, optional provenance and mixed-bundle limits, linking rather than duplicating the profile.
- Installer includes north-sources for architecture-review persistence and exposes the new reference with pipeline and standalone consumers on both tools; disabling pipeline retains required standalone dependencies and project data.
- Extend relevant existing installation tests rather than adding wording-matching skill tests.
- Run `sh -n install.sh tests/install.sh`, `cargo fmt --manifest-path installer/Cargo.toml --check`, `cargo clippy --locked --manifest-path installer/Cargo.toml --all-targets -- -D warnings`, `cargo test --locked --manifest-path installer/Cargo.toml`, and `sh tests/install.sh`. Record exact outcomes and environmental limits.

## Execution

Dispatch/session: `ses_e57494f2cffelaEOtODsjCwjNL`, completed.
Checkout/branch/base commit: `north-okf-t04` / `feature/okf-t04` / `7d721b2` (temporary parent in plan).
Changed files: Both core instructions; OpenCode execute/save wrappers; README and three docs; installer/src/install.rs; tests/install.sh.
Validation: Worker ran shell syntax, Rust fmt/clippy, cargo test (45 passed), installation smoke (passed), and diff check. Primary reviewed full diff and independently ran the shared-artifact-contracts installer test (passed, both tools).
Review/integration: Accepted and fast-forwarded as `6974637`. macOS checks only; Linux remains CI coverage.
Blockers: None known.

## History

Created during planning; no implementation started.
