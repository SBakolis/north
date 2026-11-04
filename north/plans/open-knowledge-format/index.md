# Plan: Open Knowledge Format for North artifacts

Feature: open-knowledge-format
Goal: North writes and links project-local Markdown artifacts using Open Knowledge Format (OKF) v0.2, while continuing to read existing North records.
Context: [Research and skill impact](research.md), [current plan contract](../../../assets/skills/north-sources/references/plan-format.md), and [architecture](../../../docs/architecture.md).
Acceptance: New North artifact bundles conform to OKF; task progress remains distinct from document lifecycle; old plans resume without bulk migration or evidence loss.
Non-goals: Converting bundled skills, commands, repository documentation, or OpenSpec; adding a knowledge server, graph viewer, Google agent, or attestation runtime; automatically migrating historical projects.
Max parallel: 2
Research: [Findings and decisions](research.md)

## Tasks

| ID | Task | Depends on | Layer | Status |
| --- | --- | --- | --- | --- |
| T01 | [Define shared OKF and compatibility contracts](tasks/T01-contract.md) | — | 1 | pending |
| T02 | [Adapt planning, execution, and memory skills](tasks/T02-pipeline.md) | T01 | 2 | pending |
| T03 | [Adapt supporting artifact skills](tasks/T03-supporting-skills.md) | T01 | 2 | pending |
| T04 | [Align tool instructions, docs, and installation checks](tasks/T04-integration-surfaces.md) | T02, T03 | 3 | pending |
| T05 | [Evaluate artifact production and legacy consumption](tasks/T05-evaluation.md) | T02, T03, T04 | 4 | pending |
| T06 | [Verify integrated change](tasks/T06-verification.md) | T01, T02, T03, T04, T05 | 5 | pending |

## Decisions and requirements

User-confirmed in this planning session:

1. The target is [GoogleCloudPlatform/open-knowledge-format](https://github.com/GoogleCloudPlatform/open-knowledge-format), specifically the inspected v0.2 specification.
2. Adoption covers generated project artifacts: plans, memories, research, handoffs, preference records, generated preference skills, and other saved North supporting records. Bundled steering files keep their native formats.
3. Preserve legacy reading and convert existing documents when explicitly updated; do not bulk-migrate existing repositories.
4. New plans may move their detailed overview into `plan.md`, reserve `index.md` for navigation, and use `task_status` for execution progress, separately from OKF `status`.

Selected implementation approach:

- The configured North directory is the bundle root. `north-sources` owns a single shared OKF profile and the revised plan contract; other skills reference it.
- Retain file-relative Markdown links as the authoring default (valid OKF and compatible with repository browsing). Consumers also interpret `/`-prefixed concept paths relative to the bundle root.
- New non-reserved Markdown records receive a non-empty `type`, with descriptive title/description and evidence-backed optional provenance. No fabricated verification, authorship, dates, or source statistics.
- New plan indexes and memory indexes are directory listings with descriptions. Keep detailed metadata in concept records. Only a bundle-root index can carry `okf_version` frontmatter.
- Existing task IDs, dependencies, scopes, evidence, layer barriers, and ownership remain authoritative. Legacy `status` is execution state only in positively identified legacy task records; ambiguous combinations are reported rather than guessed.
- Existing legacy plans can continue in place during execution. Explicit conversion preserves history and incoming references; this rollout does not require conversion merely to resume work.
- Read-only lookups create nothing. `/north` continues to scaffold only the output directory. New artifact writes can maintain navigation lazily; optional indexes are not a prerequisite to reading a bundle.
- Missing concept links are tolerated as OKF permits. Required task dependencies and execution prerequisites still block dispatch when unavailable. Local references North claims as existing evidence must resolve.

## Acceptance examples

- Planning a new feature creates a directory listing, an OKF plan concept, an OKF research concept, and OKF task concepts; task progress uses `task_status: pending`, not `status: pending`.
- Resuming a legacy task with `status: blocked` does not interpret it as OKF lifecycle, reset its progress, invent verification, or create a competing plan.
- Saving a partially completed feature records partial coverage and exact evidence without claiming uncompleted tasks are implemented. Reading the resulting memory performs no writes.
- Saving a preference skill retains its `name`, `description`, behavior, and evidence link while making the generated file a valid OKF concept. Preference progress does not overload OKF lifecycle.
- A minimal concept containing only `type` remains readable, as do unknown types/extensions and a dangling optional relationship; a missing required task prerequisite still prevents execution.
- A mixed legacy/new directory is reported honestly as mixed, not falsely certified as a fully conformant OKF bundle.

## Decisions and open questions

No unresolved product decisions block execution. T01 defines routine profile details and examples within the confirmed scope. Generated-skill compatibility and legacy selection ambiguities have explicit acceptance checks in T03/T05; report concrete failures before expanding scope.

## Execution and handoff

- Project: `/Users/stamatis/Documents/projects/north`
- Resolved North path: `/Users/stamatis/Documents/projects/north/north` (no configured override found).
- Source branch: `main`; starting commit: `4a3a858` (`feat: Add retrospective skill and /retrospective command`).
- Initial working tree: clean. No prior North memories, generated preference skills, or plans existed. OpenSpec specs and active changes are empty; no external task artifact needs synchronization.
- Active layer: none; implementation has not started. No delegated sessions or worktrees.
- This bootstrap plan intentionally uses the currently installed plan contract. Do not reinterpret its legacy task statuses while changing the contract. The primary retains ownership of its progress records.
- Planning validation: graph and links checked before delivery; implementation/evaluation checks remain unrun.
- Next phase: `/north-execute open-knowledge-format`.
