---
id: T01
status: pending
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

Dispatch/session: Not dispatched.
Checkout/branch/base commit: Not assigned.
Changed files: None.
Validation: None.
Review/integration: Not reviewed.
Blockers: None known.

## History

Created during planning; no implementation started.
