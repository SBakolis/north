---
name: clarify-requirements
description: Resolve consequential ambiguity in a feature or behavior change by grounding requirements in existing evidence and concrete acceptance examples. Use when missing product decisions could materially change implementation or validation, not for routine edits with clear outcomes.
---

# Clarify requirements

Produce enough shared understanding to implement and verify the requested
outcome. Scale the clarification to the uncertainty; a small change may need
only one stated assumption and one acceptance example.

## Establish what is already known

Read the request, previous answers, relevant repository instructions, existing
behavior, and applicable tests and documentation before asking questions.
Consult relevant saved North context and OpenSpec requirements, design, and
tasks when present. Separate the requested outcome from the current behavior;
an implementation detail or old test does not settle an intended product change.

Identify the affected users or callers, their intended result, constraints,
explicit exclusions, and the evidence that would demonstrate success. Preserve
decisions already made in this task. When sources conflict, describe the precise
conflict and its consequence instead of silently selecting a convenient answer.

## Resolve only decisions that matter

Focus on unknowns that change observable behavior, compatibility, scope, data
handling, or acceptance checks. Resolve repository facts by inspection. Choose
routine implementation details using project conventions and explain a material
assumption when needed; do not turn them into a product interview.

For a decision that requires the user's intent, ask a concise question with
the relevant consequence and, when useful, a recommended default. Reuse answers
across follow-ups. Continue independent work while an essential answer is pending;
do not treat silence as approval or proceed with work that depends on that answer.
Stop clarifying when remaining uncertainty can be handled within the agreed scope.

## Make the outcome verifiable

Summarize the resulting behavior, non-goals, important constraints, acceptance
examples, and any unresolved decision with the work it blocks. Use concrete
inputs or actions and observable results, including a meaningful boundary or
failure case when it changes the implementation. For example, an expired invite
must show an expiry message and leave membership unchanged; the resend flow is
excluded unless requested. Avoid implementation-shaped acceptance criteria.

Use an existing relevant requirement or task artifact as the authoritative
record. Respect the project's OpenSpec workflow and requested editing scope.
When a separate supporting record is useful and writing is permitted, the primary
agent saves it under the configured North output directory, otherwise under
`<project-root>/north/`, resolved from the working project rather than the kit.
Create directories only when saving; link to authoritative artifacts instead of
duplicating them. A planner or read-only reviewer returns proposed updates.
An adequate requirements summary does not introduce a new approval gate before
already authorized implementation.
