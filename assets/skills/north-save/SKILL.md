---
name: north-save
description: Save or update verified implementation memories with plan, code, and validation references under north/memories/. Use for /north-save after a North feature has been implemented.
---

# North save

Preserve knowledge that will help the next implementation: what exists, where
it lives, why consequential decisions were made, and how it was checked.

## Establish what was implemented

Load `north-sources` for artifact locations, its
[OKF profile](../north-sources/references/okf-format.md), and its
[plan contract](../north-sources/references/plan-format.md) for plan selection
and evidence. Without a selector or conversation context, look for a sole
completed plan. Read its task records, research decisions, code, and validation.
If no plan or implementation evidence can be found, explain what is missing;
do not infer completed work from an idea or fabricate a memory.

Check that completed tasks were reviewed, verified, and integrated into the
project's implementation. A `done` label or worker report alone is insufficient.
Validate saved claims against current code; reuse still-applicable validation
results instead of rerunning every check. Report stale or missing evidence.
Do not perform unfinished implementation as part of this command.

If the feature is incomplete, record only any verified, integrated subset as
partial work and list unfinished IDs and limitations explicitly. If no such
subset exists, do not create an implementation memory. Keep plans and their
history intact; saving memory does not mark any task done or archive the plan.

## Save and link

Update the feature memory and memory index at the locations defined by
`north-sources`. Reuse the plan's feature slug; for a legacy plan derive
a safe lowercase hyphenated slug. Reserve both `index.md` and `log.md` at every
directory level, choosing a distinct memory filename such as `index-feature.md`
or `log-feature.md` for those feature names. Report existing reserved-name
collisions rather than overwrite or silently rename their content.
Verify that an existing filename belongs to this feature; choose a distinct
filename if it belongs to another feature. Preserve unrelated entries and
previous valid context and apply the shared artifact rules. Preserve unknown
keys, source IDs, provenance, and history when making meaningful updates; identify
verification invalidated by changed claims without implying reconfirmation.
Read legacy memories in place; convert only the selected record on an explicit
format update, never unrelated memories or the active plan. Correct superseded
claims explicitly; do not append contradictory memories for the same feature.

Use this concise record shape, omitting empty or irrelevant sections:

```markdown
---
type: Implementation Memory
title: <feature title>
description: <one-sentence implemented coverage and material limits>
---
# Memory: <feature title>

Feature: <slug>
Coverage: complete | partial
Evidence checked against: <revision or explicit working-tree state>
Plan: <relative link to authoritative plan.md, legacy detailed index, or single file>
Tags: <domain terms, components, useful search terms>

## Implemented behavior
Only verified outcomes, with the completed task IDs and code references.

## Decisions and integration points
Consequential rationale, interfaces, configuration, and patterns to reuse.
Link supporting research and authoritative requirements where useful.

## Validation
Actual commands/results and evidence links, including important limits.

## Remaining work and limitations
Unfinished task IDs, known gaps, and conditions future changes must consider.
```

Add `sources` entries with actual `resource` references for saved claims; use
stable IDs when citing claims with matching footnotes. Keep code and requirements
at their authoritative locations. Optional `generated` describes actual
authorship, not verification. Add `verified` only for actual confirmation of
the document's claims against sources, with known actor and explicit-offset
timestamp as required by the profile. A task's `done`, a passing test, or this
save operation alone grants neither `verified` nor human review. Omit unknown
trust, source dates, counts, and authors rather than fill placeholders.

The memory index is a heading-grouped Markdown link listing with descriptions,
not a table or concept; it has no frontmatter. Keep coverage and validation facts
in each memory, optionally summarized in its listing description. The primary
maintains relevant navigation lazily, preserving unrelated entries; lookup alone
creates nothing. Use file-relative Markdown links by default; also accept
bundle-root `/` links as defined by the profile. Verify claimed evidence links
resolve, report missing evidence, and add a reciprocal memory link to the actual
plan overview's handoff section, not a new plan's navigation index. Do not claim
full bundle conformance from these checked records; report remaining legacy
records as a mixed bundle.

Keep memories focused on reusable implementation knowledge, with complete or
partial coverage matching the verified task outcomes. Link evidence rather than
copying entire diffs or session transcripts. These records are distinct from
learned working preferences managed by `dry-skillify`.

## Finish

Report saved paths, coverage, and material evidence gaps. Suggest
`/north-plan <next feature prompt>` to start the next cycle. For unfinished work,
suggest `/north-execute <feature>` to finish it first. Do not invoke another
command automatically.
