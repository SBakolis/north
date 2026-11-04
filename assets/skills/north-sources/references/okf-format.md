# North OKF v0.2 profile

Format authority: [OKF v0.2 specification](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/ad30107c31c06aec8a7d5636e0d1058118604e6f/SPEC.md)
(pinned commit `ad30107c31c06aec8a7d5636e0d1058118604e6f`). This is a file
convention, not a new skill, validator, or runtime dependency.

## Boundary and concepts

The resolved North output directory is one bundle, including generated local
`skills/*/SKILL.md`. Apply this profile to newly saved project artifacts, not
bundled kit skills/commands, repository documentation, code, or authoritative
requirements such as OpenSpec. Link those sources where they already live.
Any non-reserved `.md` file inside the bundle counts when assessing conformance;
legacy or foreign files are not silently exempted by their origin.

A concept is UTF-8 Markdown starting with `---`-delimited, parseable YAML mapping
frontmatter and a Markdown body. Its only always-required field is a non-empty
string `type`. Its identity is its bundle path without `.md`; a task's `id` is
a separate North identifier. Use meaningful `title` and one-sentence `description`
when known; `resource` (canonical asset URI) and `tags` are optional. Unknown
types and extension keys are valid; preserve them during updates.

North's descriptive types (not an OKF registry):

| Artifact | `type` |
| --- | --- |
| Plan overview / task | `North Plan` / `North Task` |
| Research / implementation memory | `Research` / `Implementation Memory` |
| Preference record / generated skill | `Preference Observation` / `Agent Skill` |
| Continuation / experiment | `Handoff` / `Prototype` |
| Evaluation / retrospective | `Skill Evaluation` / `Retrospective` |
| Supporting requirements / domain / architecture notes | `Requirements` / `Domain Model` / `Architecture Review` |

Other saved records may choose another descriptive type. North-specific keys,
including `north_schema`, `id`, `depends_on`, `agent`, `write_scope`,
`task_status`, and `preference_status`, are extensions. Do not overload OKF fields
with workflow state. See [the plan contract](plan-format.md) for executable plans.

## Reserved files and lazy navigation

`index.md` and `log.md` are reserved at **every** directory level, never concept
slugs. Choose a different descriptive slug for a concept named index or log;
report an existing collision instead of overwriting or silently renaming it.

Indexes are optional heading-grouped Markdown link listings with descriptions,
not detailed plan overviews or task/memory tables. They have no frontmatter,
except the bundle-root `index.md` may contain only `okf_version: "0.2"`.
For example, at the bundle root:

```markdown
---
okf_version: "0.2"
---
# Plans

- [Account settings](plans/account-settings/) - Plan, research, and tasks.

# Memories

- [Implementation memories](memories/) - Saved coverage and evidence.
```

At `plans/account-settings/index.md` (no frontmatter):

```markdown
# Account settings

- [Plan](plan.md) - Goal, decisions, task graph, and execution handoff.
- [Research](research.md) - Evidence and selected approach.
- [Contract task](tasks/T01-contract.md) - Define the settings contract.
- [Endpoint task](tasks/T02-endpoint.md) - Implement the settings endpoint.
```

List every actual task or link to a task-directory listing. Memory indexes
similarly list memory concepts; coverage belongs in those concepts, optionally
summarized in descriptions. Do not create links to nonexistent artifacts as
though they already exist. Examples here describe hypothetical saved files.

Optional `log.md` uses a flat list of prose entries grouped under ISO date
headings (`## 2026-11-05`), newest first, with links to changes; no concept
frontmatter. These date headings are dates, not timestamp-valued metadata.

Lookup/resume creates no navigation. `/north` creates only the output directory.
When saving, create/update relevant listings lazily, preserve unrelated entries,
and serialize shared navigation through the primary agent. A missing index is
never a reason to reject a bundle or block a known plan path.

## Links and evidence

Author file-relative Markdown links by default. Consumers also support `/`-prefixed
bundle-root links: from `memories/account-settings.md`, both
`../plans/account-settings/plan.md` and `/plans/account-settings/plan.md` identify
the same concept. Do not interpret the latter as a filesystem or repository path.
Resolve relative path-valued fields from the containing document as well;
`resource` and `sources[].resource` accept URLs, bundle-root paths, or relative
paths. Source scope descriptors are not paths and need not resolve.

Relative references that leave the bundle, such as `../../src/settings.ts` from
that memory, are project evidence, not portable bundle concept edges. Check
local evidence claimed to exist against the checkout; report missing evidence.
Exporting North alone does not include external code, OpenSpec, or remote sources.
Do not copy authority merely to make an export self-contained. An optional
`references/` directory may hold deliberately saved material; its Markdown
concepts follow the same rules. Links express relationships through their prose.

## Provenance, trust, freshness, lifecycle

All these families are optional. Omit unknown facts rather than fill templates
with invented actors, timestamps, verification, counts, or source dates.

- `sources` is a list; each entry requires `resource`. Add a stable `id` when
  citing it. A claim's footnote label joins to that exact `sources[].id`, not
  its list position or the footnote prose. Preserve IDs when reordering sources.
- `generated` records authorship of current content, with required `by` and a
  known last meaningful change time in `at` when available. `verified` records
  actual confirmation against sources/resource: events with `by` and `at`.
  Accept either a list or one mapping (normalize the mapping as a one-item list).
- Actors use `<producer>/<version>` for agents/tools, `human:<id>` for people,
  or `process:<id>` for processes. Use actual identities only. Agent authorship,
  a passing code test, or `task_status: done` does not itself verify a document.
- No `verified` means unverified; non-human verifiers mean machine-confirmed;
  a `human:` verifier means human-reviewed. These are derived advisory signals,
  not stored credibility scores or access-control grants. Retain history but
  identify verification invalidated by content changes; do not imply reconfirmation.
- `status` is only `draft`, `stable`, or `deprecated`; absent means `stable`
  under OKF, not reviewed, verified, or implemented. `stale_after` is an optional
  absolute instant; stale means now is at or after it.
- Every timestamp-valued field uses ISO 8601 with explicit UTC offset, e.g.
  `2026-11-05T10:00:00Z` or `2026-11-05T12:00:00+02:00`, never a date alone or
  naive local time. Quote timestamps if needed to preserve their string form.
- Source `author`, `last_modified`, and `usage_count` are optional observed facts.
  Counts use a known `usage_window: {from, to}` alongside `sources` (or an
  entry-specific override), not fabricated popularity. No attestation runtime
  is introduced; preserve unfamiliar computation fields and consult pinned §10
  before authoring an `Attested Computation` (which also requires `runtime`).

## Concept examples

These are illustrative templates, not evidence of completed implementation.
Only include source links and coverage assertions supported by the actual save.
For `plans/account-settings/research.md`:

```markdown
---
type: Research
title: Account settings research
description: Contract options and remaining evidence gaps.
sources:
  - id: contract
    resource: tasks/T01-contract.md
---
# Findings

The contract task defines the acceptance questions.[^contract]
Plan: [Account settings](plan.md).

[^contract]: Contract task and its acceptance criteria.
```

For `memories/account-settings.md`, after reviewing only the contract:

```markdown
---
type: Implementation Memory
title: Account settings contract
description: Contract coverage only; endpoint implementation remains unverified.
sources:
  - id: contract
    resource: /plans/account-settings/tasks/T01-contract.md
---
# Coverage

This record covers the contract, not endpoint completion.[^contract]
See the [plan](../plans/account-settings/plan.md) for remaining tasks.

[^contract]: Contract task; check its acceptance evidence against current code.
```

For `skills/focused-checks/SKILL.md`, only after the preference owner's required
observations support promotion:

```markdown
---
type: Agent Skill
name: focused-checks
description: Use focused checks when validating a bounded change.
---
# Focused checks

Run checks scoped to the changed behavior before broader validation.
Evidence: [Preference observations](../../dry/focused-checks.md).
```

Keep native skill `name`/`description` and direct local consumption; this profile
does not claim compatibility with every third-party skill validator. Preference
observations use `type: Preference Observation` and their owner's
`preference_status` extension, not OKF `status` for promotion progress.

## Compatibility and truthful reporting

Read legacy records in place; preserve provenance, extensions, history, and
incoming references. An explicit format update may adapt the selected record;
ordinary reading or progress updates do not mandate conversion. Never bulk
migrate unrelated records or infer conversion from a version declaration.
Use the plan contract's selection/conversion rules for live plans. Its explicit
legacy-extension exception keeps added tasks in the selected legacy format until
conversion is requested; those records remain legacy, not OKF-certified output.

Claim full bundle conformance only after checking **every** non-reserved Markdown
file has parseable frontmatter with non-empty `type`, and all reserved files
follow their defined structure. A root version targets a spec, not certification.
Report legacy/new mixtures as mixed; distinguish checked files from unchecked
bundle contents and static checks from observed writer/consumer behavior.

Consumers tolerate missing optional fields, unknown types/extensions, missing
indexes, and dangling cross-links. Missing trust is not grounds for rejection.
This tolerance does not authorize North execution with missing task metadata,
unresolved required prerequisites, or uncertain writer state: block dispatch,
not general reading. Nor does it excuse claiming a broken evidence link was
validated. An unfamiliar declared OKF version calls for best-effort reading,
not silent rewriting or a claim of verified conformance.
