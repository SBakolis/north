---
id: T03
status: done
depends_on: [T01]
agent: north-worker
write_scope: [assets/skills/dry-skillify/SKILL.md, assets/skills/research/SKILL.md, assets/skills/handoff/SKILL.md, assets/skills/prototype/SKILL.md, assets/skills/skill-evaluation/SKILL.md, assets/skills/retrospective/SKILL.md, assets/skills/clarify-requirements/SKILL.md, assets/skills/domain-modeling/SKILL.md, assets/skills/architecture-review/SKILL.md]
---

# T03: Adapt supporting artifact skills

Plan: [Open Knowledge Format](../index.md)
Prerequisites: [T01](T01-contract.md).
Research: [Writer inventory](../research.md)

## Outcome

Route saved North supporting artifacts through the shared profile, with focused template changes for preferences and research.

## Acceptance

- All nine scoped skills refer to the shared profile when persisting North output; ordinary chat output and authoritative external formats retain their behavior.
- Research records include actual source metadata with stable IDs when attributing claims; no fabricated dates, actors, credibility statistics or verification.
- Preference observations use a separate progress field and generated local SKILL.md files include OKF `type` while retaining native name/description, focused behavior and working evidence links.
- Preserve three distinct observations, contradiction handling, retirement behavior, and primary ownership. This product requirement is not itself recorded as a reusable user preference.
- Handoffs correctly refer to new or legacy plan authorities; saved prototypes/evaluations keep runnable files and evidence in appropriate locations.
- architecture-review loads north-sources for saved North assessments; report its newly required installer dependency to T04.
- Static review confirms every in-scope saved Markdown kind has a profile/type route; generated skills remain directly readable by existing shared instructions. T05 verifies actual behavior.

## Execution

Dispatch/session: `ses_e574bc87affetLT4IvxWyPFZiJ`, completed.
Checkout/branch/base commit: `north-okf-t03` / `feature/okf-t03` / `dc54069` (temporary parent in plan).
Changed files: All nine scoped SKILL.md files.
Validation: Worker static scope/header/link/type-route checks passed; primary reviewed full diff and independently ran git diff --check (passed). Behavioral checks assigned to T05.
Review/integration: Accepted; `d01dd16` merged as `7d721b2`. architecture-review dependency passed to T04.
Blockers: None known.

## History

Created during planning; no implementation started.
