---
name: dry-skillify
description: Record recurring user preferences in north/dry Markdown files and turn supported patterns into reusable preference skills after three distinct observations. Use during North work when the user expresses, repeats, or corrects a preference about how work should be done.
---

# DRY skillify

Use the project output directory resolved by `north-sources`. Record reusable
preferences about how the user wants work performed, keeping each preference
within the context supported by the evidence. Task requirements that happen to
repeat are not automatically general preferences.

## Record evidence

Maintain one Markdown record per candidate at `north/dry/<behavior-slug>.md`.
Before adding a record, look for an existing candidate or skill covering the
same behavior. Each record should contain:

- The proposed preference and its scope, including situations where it does
  not apply.
- Status: `observing`, `promoted`, or `retired`.
- Distinct observations, each with a date, task or conversation reference when
  available, a concise description of the user's request or correction, and
  the context in which it applied. If a durable reference is unavailable, record
  an honest context summary; never invent an identifier or quote.
- Contradictions or exceptions, the count of qualifying observations, and the
  generated skill path when promoted.

Count separate user-supported instances: explicit requests, corrections, or
user-approved choices in distinct tasks or independently made decisions. A
repeated quote, retry, delegated-agent report, or multiple mentions of the same
decision counts once. Agent-chosen behavior and user silence do not count as
evidence. Log only observations actually available in the current context or
verified saved sources; do not invent earlier instances to reach a threshold.

## Promote a supported pattern

After **three distinct qualifying observations** support the same scoped
preference without an unresolved contradiction, automatically create or update
`north/skills/<preference-slug>/SKILL.md`. Use a more specific threshold if the
user supplies one. An explicit request to save a preference as a skill can be
acted on immediately; record that instruction as the reason for early promotion.

Use lowercase hyphenated skill names. Include YAML frontmatter with `name` and
a concise `description` explaining the preference and when it applies. The body
should describe the desired behavior, relevant exceptions, and a relative link
to its evidence record (`../../dry/<behavior-slug>.md`). Keep the skill focused
on decisions future agents need to make; omit conversation transcripts and
generic advice. Check the frontmatter, name, scope, and evidence link before
marking the record `promoted`. Mention the created or updated skill briefly in
the task's completion report.

Update an existing matching generated skill instead of producing duplicates.
If a bundled or unrelated existing skill overlaps, record the scoped preference
locally rather than silently rewriting the shared kit or global configuration.
The `north-sources` workflow makes locally generated skills available to future
North tasks by reading them directly; no global installation is required.

When evidence conflicts, preserve the observations and narrow the scope if the
contexts explain the difference. Otherwise leave the candidate unpromoted until
the preference is clear. A current explicit correction overrides older evidence:
revise or retire an already promoted skill and update its record. For retirement,
remove obsolete behavioral instructions from the skill and identify it as retired
in its description so future readers do not apply it. Never turn a learned
preference into authority for unrelated actions or broader permissions.
