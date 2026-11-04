---
id: T05
status: done
depends_on: [T02, T03, T04]
agent: north-worker
write_scope: [north/evaluations/open-knowledge-format/]
---

# T05: Evaluate artifact production and legacy consumption

Plan: [Open Knowledge Format](../index.md)
Prerequisites: [T02](T02-pipeline.md), [T03](T03-supporting-skills.md), [T04](T04-integration-surfaces.md).
Research: [Verification strategy](../research.md)

## Outcome

Provide reproducible evidence that the revised instructions produce valid bundles and preserve legacy workflows. Keep evaluation fixtures/output isolated from active plans; write only consolidated evidence under the scoped location using the new profile.

## Acceptance

- Define expected behavior before running cases; load skill-evaluation for methods. The primary coordinates any native agent runs, with bounded temporary fixture directories and no nested worker delegation.
- Exercise fresh plan generation; legacy directory and single-file resumption; partial-memory save followed by read-only retrieval; preference promotion/generated-skill reading; saved research/handoff; chat-only and read-only near-misses.
- Check real YAML parsing, non-empty type, listing-only reserved indexes, optional root version, status separation, source ID/footnote matching, known-evidence links, and preservation of unknown metadata.
- Exercise relative and bundle-root links, missing optional concepts, missing required prerequisites, reserved `index`/`log` collisions, schema ambiguity, and mixed legacy/new output without false full-conformance claims.
- Demonstrate generated local skills remain readable with name/description and type through the supported direct-read path on both tool instruction sets. Clearly distinguish observed tool executions from static compatibility review.
- Do not claim upstream viewer or cross-tool runtime validation unless actually performed. Record skill revision, raw inputs, commands/tool use, outputs, outcomes and gaps. An unavailable execution facility leaves the relevant checks unverified for T06.
- Any discovered implementation repair returns to its owning task; do not edit outside this scope or create a production validator/dependency as an evaluation shortcut.

## Execution

Dispatch/session: Producers `ses_e5745d897ffeZtDTDdQC1PfgIS`, `ses_e5745d894ffeNuweiQRl5M3OpK`, `ses_e57429398ffepEtCK5mSGz2t7u`; consolidation `ses_e57406904ffeFKsjhPE8M51yhS`; all completed.
Checkout/branch/base commit: `north-okf-t05` / `feature/okf-t05` / `6974637`; isolated fixtures in sibling `north-okf-evals` directory.
Changed files: Seven files under north/evaluations/open-knowledge-format; raw fixtures/guidance retained in JSON, report/index, capture/replay scripts and command/validation results.
Validation: Primary inspected report/script and replayed Ruby Psych validation: 246 assertions, zero failures. Native OpenCode producer outputs checked for YAML, links, provenance, status/graph, legacy handling and nonmutation. Staged diff check passed.
Review/integration: Accepted with explicit bounded coverage; committed/integrated `ae5cadd`. See evaluation report for live-Claude/Linux/root-version/Index-title and historical-evidence limits. These do not establish full upstream or cross-runtime certification.
Blockers: None known.

## History

Created during planning; no implementation started.

T06 requested a legacy-directory task-extension fixture. Existing results remain valid for the tested baseline; add regression evidence against repaired T01/T02 before acceptance.

Added native fixture session `ses_e57389237ffePG1ONkCvPgMlTJ` with raw original/final bytes and guidance in legacy-extension.json. Primary ran validate-legacy-extension.rb and baseline validate.rb successfully; accepted supplement in `25bbcdb`. No successful task-completion round-trip claimed.
