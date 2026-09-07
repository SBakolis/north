---
name: architecture-review
description: Assess system boundaries, responsibilities, dependencies, and design tradeoffs for a requested architecture review or a consequential design choice. Use when structural decisions need evidence, not as a trigger to broadly refactor ordinary changes.
---

# Architecture review

Evaluate whether the relevant structure supports the requested behavior and
constraints. Keep recommendations proportional to the demonstrated problem;
do not expand an implementation task into a redesign.

## Establish the review boundary

Identify the question being decided, affected components, constraints, and
success criteria. Read project instructions, existing design decisions, relevant
North context, and applicable OpenSpec artifacts before proposing alternatives.
Respect decisions the user has already fixed and distinguish required constraints
from assumptions that could change with new evidence.

Trace a representative operation through callers, interfaces, state ownership,
and dependencies. Inspect consequential failure and recovery paths as relevant.
Follow effects beyond the immediate module when callers, stored data, or external
contracts depend on it; state where the inspection stops and what remains unknown.

## Compare structures using concrete consequences

Look for responsibilities split across conflicting owners, hidden coupling,
interfaces that expose unnecessary internals, and changes that require unrelated
components to move together. Connect each concern to a verified path and practical
effect, such as inconsistent authorization or duplicated state transitions.
File size, abstraction count, or stylistic preference alone is not a defect.

For a consequential choice, compare the current approach with a small set of
plausible alternatives, including the smallest sufficient change. Consider only
criteria that matter here, such as caller compatibility, consistency, operational
cost, testability, migration effort, or reversibility. Explain the tradeoff that
drives the recommendation and the evidence that could change it. Do not present
unmeasured performance or predicted future needs as established facts.

## Return an actionable assessment

Lead with the recommendation, then cite the relevant code or design artifacts,
affected callers, remaining uncertainties, and the alternatives considered.
Separate defects that obstruct the requested outcome from optional improvements.
For a recommended change, identify its bounded scope and the acceptance evidence
needed. Include sequencing or compatibility measures when migration requires them;
do not create a migration project merely because a cleaner design is imaginable.

Review alone authorizes an assessment, not code changes. A read-only agent or
`north-planner` returns proposed decisions and tasks to the primary agent.
If persistence is within scope, update the existing authoritative design record
through its project workflow. The primary agent may save supporting analysis under
the configured North output directory, otherwise `<project-root>/north/`, resolved
from the working project rather than the kit. Create it only when needed and link
to existing decisions instead of maintaining a competing design document.
