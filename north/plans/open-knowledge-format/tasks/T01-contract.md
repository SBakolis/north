---
id: T01
status: done
depends_on: []
agent: north-worker
write_scope: [assets/skills/north-sources/]
---

# T01: Define shared OKF and compatibility contracts

Plan: [Open Knowledge Format](../index.md)
Prerequisites: None.
Research: [Specification and conflicts](../research.md)

## Outcome

Extend north-sources with a concise, pinned OKF v0.2 profile (a new reference such as `references/okf-format.md`) and update its existing plan contract. Keep format ownership here; no new selectable skill or runtime dependency.

## Acceptance

- Define bundle boundary, artifact types, YAML extensions, reserved filenames, indexes/logs, link resolution, evidence outside the bundle, optional provenance/trust and truthful conformance reporting.
- Define new plan layout (`index.md` navigation, `plan.md` overview), task schema discriminator, `task_status`, and the separate OKF lifecycle. Preserve task IDs/dependencies/scopes/evidence and graph rules.
- Provide examples for plan/task, memory/research, generated skill, and navigation using required `type` and optional fields only when supported by evidence. Explain keyed source footnotes and explicit-offset timestamps.
- Define legacy single-file and directory-plan selection, resume and explicit conversion rules, ambiguous metadata handling, reserved-name collisions, and mixed-bundle reporting. Reading/resuming does not force conversion.
- Distinguish tolerant OKF consumption from missing required North execution prerequisites. Define lazy navigation maintenance without changing `/north` directory-only scaffolding.
- Validate example YAML and link intent against the pinned spec; review old/new selection cases. Record commands and any static-only checks.

## Execution

Dispatch/session: `ses_e574efee9ffeoUPJG0SC1wdGlS`, completed.
Checkout/branch/base commit: `north-okf-t01` temporary worktree / `feature/okf-t01` / `dd182ec` (full parent path in plan).
Changed files: north-sources SKILL.md, references/plan-format.md, new references/okf-format.md.
Validation: Worker parsed seven YAML blocks and checked four reference links/six resolution cases. Primary independently parsed example YAML with Ruby Psych; passed. Full diff/profile reviewed against spec and task acceptance.
Review/integration: Accepted after review; committed and fast-forwarded into integration as `dc54069`. Behavioral validation remains T05.
Blockers: None known.

## History

Created during planning; no implementation started.

T06 found ambiguous new-task metadata when extending legacy plans. Primary repair explicitly retains legacy task format under the selected legacy authority; acceptance awaits focused fixture and re-review.

Repair accepted in `25bbcdb`: native fixture preserves legacy authority and extensions; primary replay passed. No downstream contract conflict found in T03/T04.
