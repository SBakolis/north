---
name: invoke-memory
description: Retrieve relevant saved North implementation knowledge before planning, implementation, or resume. Use when prior behavior, decisions, or integration details may inform a change.
---

# Invoke memory

Load `north-sources` for the resolved artifact root and memory location.
Read its [OKF profile](../north-sources/references/okf-format.md) for metadata,
trust, and link interpretation.
Lookup is read-only; absent or empty memories require no setup.

Use the memory index to select entries matching the feature, components, domain
terms, or code paths. Search filenames and relevant content if the index is
absent or incomplete. Read only matching records and follow their plan, code,
and validation references when they affect the task.

Read new typed concepts and legacy memories in place, including legacy table
indexes and new heading-grouped listings. Missing indexes, optional trust data,
unknown types/extensions, or dangling optional links do not prevent reading.
Resolve file-relative links and path-valued source resources from the record;
resolve `/`-prefixed paths from the North bundle root, never the filesystem root.
Links outside the bundle are project evidence, not portable concept edges.
For claim attribution, join footnotes to stable `sources[].id` values. Accept
`verified` as one mapping or a list; absence means unverified, not invalid.
Agent authorship, lifecycle `stable`, and task completion imply neither document
verification nor human review. Assess any recorded verification, freshness, and
partial coverage against current evidence rather than trusting labels.

Check saved facts against the current code and dependency versions. Distinguish
implemented behavior from partial coverage, unfinished tasks, and obsolete notes.
Resolve consequential contradictions before relying on a memory; apply the
shared North rule that current instructions and evidence take precedence.
Report missing claimed evidence without rejecting unrelated readable content.
When following an executable plan, use the plan contract's new/legacy authority
and schema rules; optional-link tolerance never waives required prerequisites.
Do not rewrite records, normalize stored metadata, create navigation, or migrate
legacy content during retrieval.

Pass a concise summary and exact memory paths into planning and relevant worker
assignments. Identify evidence gaps rather than implying the memory was verified.
Leave memory updates to `north-save`.
