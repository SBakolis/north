---
name: retrospective
description: Review a finished or derailed coding session and propose evidence-backed improvements to the agent's environment, such as steering files, checks, review standards, skills, plans, and memory. Use for /retrospective; it suggests changes rather than editing code.
disable-model-invocation: true
---

# Retrospective

Improve future runs by changing the agent's **environment**, not by revisiting
the session's code. A retrospective proposes changes; it does not apply them,
redo the task, or judge the user's requests.

## Gather the evidence

Default to the current session. When the user names another session, plan, or
time range, read that session's log from the tool's local storage (Claude Code
keeps per-project JSONL transcripts under `~/.claude/projects/`). If it cannot
be found, say so; never reconstruct a session from memory.

Load `north-sources` to locate related North artifacts: the plan and its task
records, handoffs, memories, `dry/` preference records, and generated skills.
Read the steering files that were in context: the global and project
`CLAUDE.md` or `AGENTS.md`, loaded skills, and North agent definitions. Read the
repository's own check commands (build-tool scripts, pre-commit hooks, CI
workflows) before proposing new ones.

Find the moments that cost the most: repeated searches, wrong files edited,
failed checks, rejected reviews, user corrections, retries, abandoned
approaches, and expensive tool calls. Record each with a concrete reference
(message, tool call, task ID, or file) so the user can verify it.

## Look for improvements

Consider each category only when the evidence supports it.

- **Navigation**: the agent spent long finding a file, symbol, or hidden
  dependency. Propose a short pointer in the closest steering file or doc.
- **Automated checks**: an error was mechanical (a type, a banned API, an import
  shape, a file location, formatting). Prefer a deterministic check in the repo's
  existing linter, hook, or CI job over a written rule. An existing check that is
  unwired or broken is the finding. A repository with no guardrail running its
  lint, typecheck, or tests is itself a finding.
- **Review standards**: `code-review` or `north-verifier` missed a mistake, or
  enforced a rule that no longer applies. Reserve written standards for judgement
  calls; route mechanical rules to automated checks instead.
- **Steering files**: global or project instructions are large, contradictory,
  or contain no-ops (text that does not change behavior). Propose moving rules
  to review standards, checks, or on-demand skills and docs, or deleting them.
- **Plans and delegation**: tasks failed verification because of missing
  acceptance checks, wrong dependencies, oversized scope, vague worker briefs,
  or worker reports accepted without evidence. Point to the plan contract,
  `north-plan`, or `subagent-usage` guidance that would have prevented it.
- **Memory and preferences**: the agent rediscovered a known decision, relied on
  a stale memory, or the user repeated a correction. Propose a `north-save`
  update or a `dry-skillify` observation; do not write them here.
- **Skills**: a relevant skill was not loaded, was loaded for the wrong task, or
  led the agent astray. Propose a description or body change and a
  `skill-evaluation` scenario that would catch the regression.
- **Tool economy**: a tool call or custom CLI/MCP was disproportionately
  expensive or repeated. Propose a cheaper call, a narrower query, or delegation
  to an exploration agent.
- **Information access**: a crucial fact was unavailable, such as server logs,
  runtime output, or read-only access to a third-party service. Propose how the
  agent could see it next time.

## Place each change deliberately

Implementation agents carry the most context pressure; reviewers receive a diff
and can afford standards. Put each change where it is cheapest to apply:

- `CLAUDE.md` or `AGENTS.md` loads into every session. Use it sparingly, mostly
  for navigation pointers.
- Review standards and the verifier's acceptance criteria apply during review,
  not implementation.
- Docs are references that other files point to; reuse existing docs first.
- Skills fit on-demand guidance and user-invoked commands, since only their
  descriptions occupy context.
- Separate project changes from North kit changes. A change to North's shared
  skills, agents, or instructions belongs in the North repository, not in an
  installed symlink's target edited from another project.

## Report

Present candidates in order of severity. For each, give the category, the
session evidence, the proposed change with its target file, and why that
location fits. Note candidates considered and rejected only when the reason is
useful. Keep the report concise; do not paste transcripts.

Return the report in the response. Save it under `retrospectives/` in the
North directory only when the user asks. Apply a proposed change only after the
user selects it, following the owning skill for memories, preferences, plans,
and commits.
