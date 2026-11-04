---
type: Implementation Memory
title: Open Knowledge Format adoption
description: North artifact guidance uses OKF v0.2 with legacy-plan compatibility, backed by bounded OpenCode evaluations and installer checks.
tags: [okf, markdown, frontmatter, plans, legacy-compatibility, skills, installer]
sources:
  - id: profile
    resource: ../../assets/skills/north-sources/references/okf-format.md
  - id: contract
    resource: ../../assets/skills/north-sources/references/plan-format.md
  - id: plan
    resource: ../plans/open-knowledge-format/index.md
  - id: impact
    resource: ../plans/open-knowledge-format/research.md
  - id: installer
    resource: ../../installer/src/install.rs
  - id: evaluation
    resource: ../evaluations/open-knowledge-format/report.md
  - id: replay
    resource: ../evaluations/open-knowledge-format/validation.json
  - id: acceptance
    resource: ../plans/open-knowledge-format/tasks/T06-verification.md
---
# Memory: Open Knowledge Format adoption

Feature: open-knowledge-format
Coverage: complete (planned implementation; validation limits below)
Evidence checked against: `49013ff` on `main`, clean tree before this memory save; implementation and compatibility repair integrated through `25bbcdb`.
Plan: [Authoritative legacy overview](../plans/open-knowledge-format/index.md)

## Implemented behavior

- **T01:** `north-sources` owns the pinned OKF v0.2 profile and revised plan contract. The configured North output directory is the bundle root. New non-reserved Markdown concepts use YAML `type`; unknown extensions are preserved. `index.md` is navigation and `log.md` history at every level. File-relative links remain the authoring default; `/`-prefixed concept links resolve from the bundle root.[^profile]
- **T02:** New plans use `plan.md` for the detailed overview/task table and `index.md` for listings. Plan/task records declare `north_schema: north-plan-v1`; tasks use `task_status`, independently of OKF lifecycle `status`. Planning, execution, exploration, delegation, memory saving and retrieval follow the shared contract.[^contract]
- **T03:** Supporting artifact writers route saved North records through the profile. Preferences use `preference_status`; generated local skills include `type: Agent Skill` alongside native `name`/`description`. The three-observation promotion rule remains. Research uses source resources and stable source IDs for attributed footnotes. The [impact inventory](../plans/open-knowledge-format/research.md) identifies all sixteen updated skills.[^impact]
- **T04:** OpenCode/Claude core guidance, selectors and documentation agree with the shared profile. The installer supplies `north-sources` for standalone `architecture-review`, and installation tests check profile/contract availability for both tools with and without the pipeline.[^installer]
- **T05–T06:** Retained native OpenCode fixture outputs cover new plans, partial-memory saving/retrieval, preferences, research/handoff, optional-link tolerance, missing prerequisites, ambiguous state and legacy extension. Final review accepted the integrated change after a compatibility repair; all six tasks are complete.[^evaluation][^acceptance]

## Decisions and integration points

- **Legacy extension is a deliberate exception:** revising a legacy directory plan keeps its detailed index authoritative and added tasks in the existing legacy execution format (`status`, without `north_schema`). Legacy single-file plans retain their table/details. Do not mix new-schema tasks into a legacy authority or force conversion just to resume. Explicit conversion must preserve history, references and execution evidence. This rule was clarified in `25bbcdb` after review found conflicting new-task instructions.[^contract]
- Apply OKF to generated project artifacts, including generated preference skills—not bundled steering files, repository docs, code or authoritative OpenSpec. Shared catalog changes belong in `assets/skills/`; tool-specific instructions live under `assets/opencode/` and `assets/claude/`. No external OKF runtime or new selectable skill was added.[^impact][^profile]
- Optional dangling knowledge links do not invalidate OKF consumption; missing required North prerequisites still block dispatch. Unknown schemas or conflicting execution/lifecycle metadata require clarification rather than resetting progress.[^contract]
- Provenance and trust fields are evidence-backed and optional. A passing test, task completion, authorship or save operation does not itself confer document verification or human review. Read-only lookup creates no files; `/north` still creates only the directory.[^profile]
- The bootstrap plan remains legacy by design. This repository's North directory is a mixed legacy/new bundle, not certified fully conformant. Relative code references leaving the bundle are checkout evidence and are not included in a North-only export.[^plan][^profile]

## Validation

Reuse the recorded results below; this save checked current source/evidence and links without repeating implementation suites.

- Final source-tree `cargo test --locked --manifest-path installer/Cargo.toml`: **45 passed**. Earlier macOS shell syntax, Rust formatting/clippy and `sh tests/install.sh` passed; installer reference checks cover both tool layouts.[^acceptance]
- `ruby north/evaluations/open-knowledge-format/validate.rb`: **249 assertions, zero failures** on final delivery. The initial baseline had 246 assertions; three later deliverable-link checks account for the final count. Retained results and scripts are under [evaluation evidence](../evaluations/open-knowledge-format/index.md).[^replay]
- `ruby north/evaluations/open-knowledge-format/validate-legacy-extension.rb`: **passed**. The native regression preserved the legacy authority, original task/evidence/extensions and added pending dependent tasks; read-only preparation identified the eligible task without mutation.[^evaluation]
- Independent read-only review found no remaining actionable issue after the legacy-extension repair. Implementation was merged into `main`, and all six clean, merged task-owned worktrees/branches were removed.[^acceptance]

## Remaining work and limitations

No unfinished implementation task IDs remain. Coverage is bounded: no live Claude runtime, Linux execution, upstream viewer/reference-agent interoperability, automatic skill routing, full task-completion fixture, optional root-version emission, unfamiliar-version consumption, or Index-title collision workflow was demonstrated. Claude instruction consumption was simulated within OpenCode; installation checks are not runtime behavior checks. Historical hashes and tool summaries are retained, not complete session transcripts. These limits do not invalidate the accepted rollout, but must constrain future interoperability or conformance claims.[^evaluation]

[^profile]: [Shared OKF profile](../../assets/skills/north-sources/references/okf-format.md).
[^contract]: [Plan schema, lifecycle, selection and legacy rules](../../assets/skills/north-sources/references/plan-format.md).
[^plan]: [Completed bootstrap plan](../plans/open-knowledge-format/index.md).
[^impact]: [Research and sixteen-skill impact map](../plans/open-knowledge-format/research.md).
[^installer]: [Installer dependency map and shared-artifact tests](../../installer/src/install.rs).
[^evaluation]: [Observed evaluations, regression and limits](../evaluations/open-knowledge-format/report.md).
[^replay]: [Retained final replay results](../evaluations/open-knowledge-format/validation.json).
[^acceptance]: [Final review and source-tree validation](../plans/open-knowledge-format/tasks/T06-verification.md).
