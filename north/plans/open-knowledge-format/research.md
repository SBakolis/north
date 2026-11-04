# Research: Open Knowledge Format adoption

Plan: [Open Knowledge Format](index.md)
Observed: 2026-11-05. Local baseline: `4a3a858` on `main`.

## Question and recommendation

Which North skills must change so project-local Markdown output is a coherent OKF bundle, and what compatibility boundaries matter?

Extend `north-sources` with one reusable OKF profile, then adapt the artifact writers and consumers below. Use OKF v0.2, preserve relative links, separate task progress from lifecycle, and support old plans without mandatory migration. No external runtime dependency is necessary: OKF is a file convention and North's workflow is instruction-driven.

## Primary external evidence

The user supplied the official repository after search returned no results. Read its README, full SPEC.md, and pyproject.toml. GitHub's commit API identified main as `ad30107c31c06aec8a7d5636e0d1058118604e6f` (2026-08-21). Sources retrieved 2026-11-05:

- [OKF v0.2 specification, pinned](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/ad30107c31c06aec8a7d5636e0d1058118604e6f/SPEC.md).
- [Project scope and sample bundles](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/ad30107c31c06aec8a7d5636e0d1058118604e6f/README.md).
- [Reference agent dependencies and Apache-2.0 header](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/ad30107c31c06aec8a7d5636e0d1058118604e6f/pyproject.toml).

Specification findings:

| Rule | Source | Consequence for North |
| --- | --- | --- |
| Every non-reserved Markdown file has YAML frontmatter and a non-empty `type`; unknown extensions are allowed | §§4, 11 | Plans, research, memories, supporting notes, and generated `SKILL.md` files all count within the bundle |
| Concept identity is its bundle path without `.md` | §2 | Stable task IDs remain North-specific extensions, not replacements for OKF concept identity |
| `index.md` and `log.md` are reserved at every level | §§3.1, 8–9 | Prevent concept slug collisions with both names; detailed plan overview belongs in `plan.md` |
| Indexes are heading-grouped listings without frontmatter, except root `okf_version` | §§8, 12 | Replace generated task-table-only and memory-table-only indexes with navigational listings; keep execution tables in the plan concept |
| Relative links are valid; root-relative links are recommended | §6 | Keep North's authoring convention, teach consumers bundle-root interpretation, distinguish code references outside the bundle |
| Missing indexes, optional fields, unknown types and broken cross-links cannot invalidate consumption | §11 | Do not confuse generic OKF conformance with stricter North execution prerequisites |
| `sources` entries require `resource`; per-claim attribution uses footnotes keyed by stable source IDs | §5.1 | Research and memory templates need provenance guidance rather than only a freeform citations list |
| `generated` is authorship; `verified` is actual confirmation; absent verification is valid | §§5.2–5.3, 7 | Agent-written or `done` content is not automatically human-reviewed; record real actors and explicit-offset timestamps only when known |
| Lifecycle `status` is draft/stable/deprecated | §5.4 | Move task progress into `task_status`; use a separate preference progress field such as `preference_status` |
| Trust, freshness and attestation families are optional | §§5, 10–11 | Do not add attestation infrastructure, mandatory timestamps, or fabricated trust fields |

## Repository evidence and skill impact

The shared catalog is `assets/skills`, not `assets/opencode/skills`. Both tools link to this same catalog. There are no relevant saved memories, preference skills, main OpenSpec specifications, or active OpenSpec changes to reuse.

### Substantive contract/template/reader updates

| Skill | Observed responsibility and needed change | Task |
| --- | --- | --- |
| [north-sources](../../../assets/skills/north-sources/SKILL.md) | Owns storage and relative links. Add shared profile, bundle boundary, types, reserved files, source/link semantics and mixed legacy policy; revise [plan contract](../../../assets/skills/north-sources/references/plan-format.md) | T01 |
| [north-plan](../../../assets/skills/north-plan/SKILL.md) | Creates indexes, research and tasks. Produce new plan concept plus navigation; use new metadata while preserving graph checks | T02 |
| [north-execute](../../../assets/skills/north-execute/SKILL.md) | Reads task status and updates progress. Support schema-aware `task_status` and legacy state, preserving lifecycle/provenance/extensions | T02 |
| [north-save](../../../assets/skills/north-save/SKILL.md) | Has an explicit memory template and table index; reserves only `index` as a slug. Add concept metadata, evidence-derived provenance and list indexes, reserve `log` too, link actual plan authority | T02 |
| [invoke-memory](../../../assets/skills/invoke-memory/SKILL.md) | Reads index, coverage and evidence. Accept OKF plus legacy records; follow both link forms; preserve read-only and evidence validation behavior | T02 |
| [dry-skillify](../../../assets/skills/dry-skillify/SKILL.md) | Writes preference records and generated skills. Add `type`, separate preference status, retain generated skill name/description and evidence links, preserve three-observation threshold | T03 |
| [research](../../../assets/skills/research/SKILL.md) | Writes standalone/plan research with citations. Apply concept metadata and stable keyed source attribution when saving | T03 |

### Focused routing, persistence, and compatibility updates

These skills already use `north-sources`; keep edits short rather than repeating its schema.

| Skill | Needed adaptation | Task |
| --- | --- | --- |
| [north-explore](../../../assets/skills/north-explore/SKILL.md) | Return evidence sufficient for the primary's OKF research record; reference actual plan authority | T02 |
| [subagent-usage](../../../assets/skills/subagent-usage/SKILL.md) | Carry authoritative plan concept/index and schema-aware status conventions into assignments; preserve primary ownership | T02 |
| [handoff](../../../assets/skills/handoff/SKILL.md) | Saved handoff becomes a concept; locate new or legacy plan authority without rewriting on read | T03 |
| [prototype](../../../assets/skills/prototype/SKILL.md) | Apply profile to saved experiment notes, preserving location of runnable assets | T03 |
| [skill-evaluation](../../../assets/skills/skill-evaluation/SKILL.md) | Apply profile to saved evaluation evidence; distinguish static examples from observed agent results | T03 |
| [retrospective](../../../assets/skills/retrospective/SKILL.md) | Apply profile only when persistence is requested; retain chat-first output | T03 |
| [clarify-requirements](../../../assets/skills/clarify-requirements/SKILL.md) | Apply profile to North supporting records only; authoritative external requirements retain their format | T03 |
| [domain-modeling](../../../assets/skills/domain-modeling/SKILL.md) | Same boundary for supporting domain records versus existing glossary/OpenSpec | T03 |
| [architecture-review](../../../assets/skills/architecture-review/SKILL.md) | Currently has inline artifact-root fallback rather than loading `north-sources`; route saved North assessments through shared profile and make dependency available | T03/T04 |

Sixteen skills have identified changes. No direct schema change is needed in `diagnosing-bugs`, `code-review`, `test-design`, `explain-code`, `unity-ui`, `commit`, or `auto-commit`: they do not own persisted North record formats. Review their references for contradictions, but avoid broad rewrites. External installed skills and generated OpenSpec integration skills are outside the kit change.

### Related surfaces

- [OpenCode core](../../../assets/opencode/instructions/core.md) and [Claude core](../../../assets/claude/instructions/core.md): shared artifact routing and fallback behavior must agree, without loading the full format into every session.
- [Installer dependency map](../../../installer/src/install.rs), lines 29–50: the shared profile can ship inside existing `north-sources`; no new selectable skill is needed. Add `architecture-review` to its consumers if its persistence path now requires that skill.
- The installer's `shared_plan_contract_is_available_without_enabling_pipeline` test (same file, around line 909) demonstrates how to test installed references. Extend coverage for the OKF reference and both tools.
- [Scaffold command](../../../assets/opencode/commands/north.md) and its [Claude counterpart](../../../assets/claude/commands/north.md) explicitly create only the directory. Keep that behavior; indexes are optional in OKF.
- [README](../../../README.md), [architecture](../../../docs/architecture.md), and [plan guide](../../../docs/plan-format.md) contain layout/status explanations that need alignment. Tool-specific agents/commands should be audited for stale plan pointers; their native frontmatter stays intact.
- [CI](../../../.github/workflows/ci.yml) runs shell syntax, Rust fmt/clippy/tests, and [installation smoke tests](../../../tests/install.sh) on Linux/macOS. These establish installation behavior, not agent-generated bundle correctness.

## Selected profile and compatibility approach

Use one bundle rooted at the configured North output directory. Define readable types by artifact kind (for example North Plan, North Task, Research, Implementation Memory, Preference Observation, Agent Skill). These are North conventions; OKF has no type registry. Keep ordinary plan metadata as extension fields. Do not treat unknown extension keys as errors or delete them during updates.

Define an explicit North schema discriminator for new plans/tasks in T01. Never infer legacy task state solely from a generic concept's `status`. New writers use `task_status`; legacy readers preserve old task metadata. If both old and new fields appear ambiguously, report the conflict instead of resetting progress. Avoid automatic live-plan conversion while dispatch is active.

For new plans, `index.md` lists `plan.md`, `research.md`, and task entries or a task-directory listing. `plan.md` owns the overview, decisions, execution handoff and synchronized task table. Task files remain authoritative for their metadata. Memory indexes list concepts with descriptions; coverage and verification facts belong to each memory and may be summarized in listing descriptions.

Use existing file-relative Markdown links by default. A `/plans/...` link means North-bundle root, not repository or filesystem root. Relative links to code/OpenSpec outside the bundle remain project evidence references rather than guaranteed portable concept edges; do not copy authoritative sources into the bundle just to make an export self-contained. Describe that portability limit honestly.

Generated local skills are currently read directly rather than installed globally (README and dry-skillify). Preserve that path while adding OKF `type` alongside their native `name`/`description`. Verify direct consumption on both tools; this research does not claim arbitrary third-party Agent Skills validators accept additional top-level fields.

Legacy records remain readable, and unrelated records are preserved. An explicit update may adapt the selected record with provenance intact. Mixed directories cannot claim full OKF conformance until every non-reserved Markdown file conforms; no implicit bulk migration follows from a root version declaration. Reserved-name collisions require safe slug selection, not overwriting an index/log. Optional navigation creation is lazy and serialized by the primary.

## Alternatives and dependencies

1. **Shared profile under north-sources (selected):** reuses existing ownership and dependency distribution; least duplication and no new runtime.
2. **Separate OKF skill:** useful if independent import/export workflows arise, but currently adds installation/routing work for the same consumers.
3. **Root-absolute links everywhere:** OKF recommends them, but relative links are equally valid and preserve repository browsing from a North subdirectory. Consumers should support both.
4. **Convert all repository Markdown or bulk-migrate projects:** user selected generated output with legacy compatibility; broad conversion conflicts with native formats and increases migration risk.
5. **Adopt upstream reference agent/visualizer:** README explicitly labels them proof-of-concept producer/consumer examples. The Python package brings Google ADK, BigQuery, PyYAML, Pydantic and markdownify, requires Python >=3.11, and has a package version distinct from the spec. No such runtime is needed to author this format. Avoid vendoring upstream code; link the pinned spec.

No libraries will be installed during planning. During evaluation, use an available YAML parser for real parse checks rather than regex-only claims; any reproducible new test dependency needs justification in T05. A new public validator CLI is outside scope.

## Verification strategy and evidence limits

T05 exercises new-plan creation, legacy resumption, partial-memory save/read, preferences/generated skills, and standalone saved research/handoff. Add near-misses: chat-only/read-only requests, authoritative OpenSpec edits, and missing optional fields. Check relative/root-relative edges, index/log collisions, schema ambiguity, extension preservation and no false verification. An unknown optional link cannot invalidate OKF, but an unresolved required execution prerequisite must stop dispatch.

Inspection and specification research are complete. No producer, consumer, upstream viewer, revised skill, or future behavioral evaluation has been executed. Full conformance and cross-tool behavior remain implementation acceptance work. No unresolved product decision remains after the user's scope and compatibility answers.
