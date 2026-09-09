---
name: invoke-memory
description: Retrieve relevant saved North implementation knowledge before planning, implementation, or resume. Use when prior behavior, decisions, or integration details may inform a change.
---

# Invoke memory

Load `north-sources` for the resolved artifact root and memory location.
Lookup is read-only; absent or empty memories require no setup.

Use the memory index to select entries matching the feature, components, domain
terms, or code paths. Search filenames and relevant content if the index is
absent or incomplete. Read only matching records and follow their plan, code,
and validation references when they affect the task.

Check saved facts against the current code and dependency versions. Distinguish
implemented behavior from partial coverage, unfinished tasks, and obsolete notes.
Resolve consequential contradictions before relying on a memory; apply the
shared North rule that current instructions and evidence take precedence.

Pass a concise summary and exact memory paths into planning and relevant worker
assignments. Identify evidence gaps rather than implying the memory was verified.
Leave memory updates to `north-save`.
