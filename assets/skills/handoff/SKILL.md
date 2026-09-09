---
name: handoff
description: Prepare or consume a concise task handoff when work changes sessions, agents, or owners. Preserve the current objective, authoritative artifacts, actual execution state, and safe next actions without redispatching work implicitly.
---

# Handoff

Preserve enough state for another agent to continue without reconstructing the
conversation or duplicating active work. A handoff describes work; it does not
start a session, dispatch a task, or prove that a task is complete.

## Establish the current state

Load `north-sources` for artifact conventions. Read the user's latest scope,
constraints, relevant plan, and current checkout. For a North plan, follow its
[plan contract](../north-sources/references/plan-format.md) to reconcile evidence
and execution state. Record when state was observed and what remains unknown.

## Prepare the handoff

Include the information that changes how the recipient should continue:

- The current objective, acceptance conditions, explicit exclusions, and user
  decisions or permissions that remain relevant.
- The working project, relevant branch or checkout identity, and paths to the
  authoritative requirements, plan, decisions, and evidence.
- Completed work with its verification evidence, partial changes, unfinished task
  IDs, and known user changes that the recipient must preserve.
- Active or interrupted execution, including dispatch or session references when
  available. Distinguish observed stopped work from work with unknown state.
- Blockers, unresolved decisions, invalidated evidence, and the next actions in
  dependency order. State what must be checked before another writer can begin.

Summarize decisions and their consequences, then link to the underlying artifacts.
Do not duplicate a plan, copy whole logs, include secrets, or reproduce historical
discussion that no longer affects the task. Distinguish facts from assumptions.

## Resume safely

When resuming a North plan, use the contract's recovery procedure before relying
on saved status. For other work, recheck cited artifacts and observed execution
state against the current workspace. The active execution workflow decides which
tasks may proceed; a handoff does not bypass a pipeline layer barrier. Preparing
one does not authorize sending it to another person or starting more work.

Return the handoff in the response unless persistent output is requested or part
of the established workflow. When saving, use the handoff location from
`north-sources` and reference the existing plan under the shared artifact rules.
