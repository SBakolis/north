---
name: handoff
description: Prepare or consume a concise task handoff when work changes sessions, agents, or owners. Preserve the current objective, authoritative artifacts, actual execution state, and safe next actions without redispatching work implicitly.
---

# Handoff

Preserve enough state for another agent to continue without reconstructing the
conversation or duplicating active work. A handoff describes work; it does not
start a session, dispatch a task, or prove that a task is complete.

## Establish the current state

Read the user's latest scope and constraints, the relevant plan or task artifact,
and the current checkout. Reconcile saved status with changed files, validation
evidence, and available native session state. Record when execution state was
observed and identify anything that could not be established.

Keep the authoritative plan where the project requires it. In North plans, retain
stable task IDs and the existing pending, running, needs-review, done, and blocked
states. A returned worker result still needs primary review; a status label alone
does not establish completion.

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

On receipt, reread the authoritative artifacts and compare them with the current
workspace and available session state. Check that cited evidence still applies
to the current changes. Review partial edits before continuing or retrying.

Do not redispatch a running task until the previous execution is known to have
stopped. If its state cannot be established, report a blocker; independent work
may continue if it cannot conflict. Revise stale dependencies or acceptance
evidence before relying on completed tasks.

Only the primary agent updates a shared plan or consolidated handoff record.
Delegated agents return proposed updates within their assigned scope. Preparing
a handoff does not authorize sending it to another person or starting more work.

Return the handoff in the response unless persistent output is requested or part
of the established workflow. When saving, honor the configured North directory
or use `<project-root>/north/handoffs/`, referencing the existing plan instead of
creating a competing one. Consult `north-sources` if available; no other skill is
required. For read-only requests, inspect and report without modifying records.
