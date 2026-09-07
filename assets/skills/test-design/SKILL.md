---
name: test-design
description: Choose and implement meaningful behavior tests for substantive changes, regressions, and coverage gaps. Use when deciding what evidence a test should provide or whether existing tests adequately cover a change.
---

# Test design

Design tests around externally meaningful behavior and the ways a change could
fail. Use the existing acceptance criteria, project test conventions, and available
evidence. Do not create a second plan when an authoritative task plan exists.
Keep test changes within the assigned scope; a review request remains read-only.

## Choose what needs proof

Identify the behavior being added or repaired and what callers rely on. Separate
requirements established by the user or project from assumptions inferred from
the current implementation. Resolve consequential ambiguity before encoding it
as an expectation; reuse decisions already made in the task.

Inspect existing coverage before adding cases. Select checks for plausible risks,
such as a state transition, boundary value, failure response, or interaction with
another component. Cover the important distinctions without enumerating every
input. A low-impact reversible edit may need only an existing check or inspection;
do not add tests solely because a file changed.

Choose the narrowest test boundary that can reveal the relevant failure. A unit
test suits isolated rules; an integration test suits wiring and persistence; a
user-flow test suits behavior that depends on the assembled system. Add broader
checks when their extra coverage justifies their cost and instability.

## Make expectations independent

- Derive expected results from requirements, examples, invariants, or a separately
  justified calculation. Do not compute the expected value with the same logic
  under test or accept current output merely because it was recorded.
- Prefer observable results and public interfaces. Assert internal details only
  when they are themselves a required contract or essential to expose the risk.
- Use small fixtures that preserve the condition responsible for the behavior.
  Control time, randomness, and external dependencies when they obscure results.
- Substitute dependencies at a meaningful boundary. Avoid mocks that reproduce
  the implementation so closely that the real integration can fail unnoticed.
- Make failure messages identify the violated behavior. Snapshots are useful only
  when their contents can be reviewed and an unwanted change would be noticed.

## Demonstrate and report coverage

For a substantive regression or new behavior, use a focused failing test before
implementation when practical. Confirm it fails for the intended reason, rather
than a broken fixture, missing dependency, or invalid setup. Existing suitable
tests can supply this evidence; no universal test-first or approval step is needed.

After implementation, run the focused checks and applicable project checks.
Investigate failures instead of weakening assertions to match the code. Distinguish
pre-existing failures, environment limits, and defects introduced by the change.
Expand testing only when new changes, failures, or unresolved risks warrant it.

Report which behavior each check establishes, what ran and with what outcome,
and material gaps. Do not describe an unexecuted test as verification. Delegated
workers send evidence and existing task IDs to the primary agent; the primary
owns shared records and completion decisions. Read-only reviewers request missing
execution evidence from the primary agent instead of exceeding their permissions.
