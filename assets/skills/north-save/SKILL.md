---
name: north-save
description: Save or update verified implementation memories with plan, code, and validation references under north/memories/. Use for /north-save after a North feature has been implemented.
---

# North save

Preserve knowledge that will help the next implementation: what exists, where
it lives, why consequential decisions were made, and how it was checked.

## Establish what was implemented

Load `north-sources` for artifact locations and its
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
a safe lowercase hyphenated slug. Reserve `index.md` for navigation, choosing a
distinct memory filename such as `index-feature.md` if the feature is `index`.
Verify that an existing filename belongs to this feature; choose a distinct
filename if it belongs to another feature. Preserve unrelated entries and
previous valid context and apply the shared artifact rules. Correct superseded
claims explicitly; do not append contradictory memories for the same feature.

Use this concise record shape, omitting empty or irrelevant sections:

```markdown
# Memory: <feature title>

Feature: <slug>
Coverage: complete | partial
Verified against: <date and revision, or explicit working-tree state>
Plan: <relative link to the authoritative plan index or legacy plan>
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

The index has one row per feature linking its memory, with a short summary,
search terms, complete/partial coverage, and verification date or revision.
Use relative Markdown links to the plan and project files from the memory's
location. Verify saved links resolve, and add a reciprocal memory link to the
plan index or its handoff section.

Keep memories focused on reusable implementation knowledge, with complete or
partial coverage matching the verified task outcomes. Link evidence rather than
copying entire diffs or session transcripts. These records are distinct from
learned working preferences managed by `dry-skillify`.

## Finish

Report saved paths, coverage, and material evidence gaps. Suggest
`/north-plan <next feature prompt>` to start the next cycle. For unfinished work,
suggest `/north-execute <feature>` to finish it first. Do not invoke another
command automatically.
