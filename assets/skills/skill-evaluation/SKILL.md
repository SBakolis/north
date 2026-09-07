---
name: skill-evaluation
description: Evaluate whether a skill selects the right tasks and improves observable agent behavior. Use when validating a new or revised skill or investigating a skill failure, beyond checking its file structure or wording.
---

# Skill evaluation

Test the decisions and results a skill is meant to improve. A valid skill file
can still misroute requests, add unnecessary work, or produce weak evidence.

## Choose meaningful cases

Read the skill's stated purpose and identify the behavior under evaluation.
Select a small set of realistic user requests with concrete observable outcomes.
Include a request that should use the skill and a nearby request that should not.
Add boundary cases when they address a plausible failure, such as an interrupted
task, incomplete evidence, or an already authorized action.

Define success and failure before running the cases. Judge behavior and artifacts:
whether the reported defect is reproduced, a public interface is tested, a
requirement is omitted, or an active writer is duplicated. Exact wording, section
headings, and regular-expression matches are not proof of those behaviors.

Keep routing and execution separate. A case that explicitly invokes a skill can
test its workflow but cannot prove automatic selection. To test selection, use
ordinary task wording with the skill available through normal discovery.

## Run within a bounded environment

Use a fresh temporary workspace or isolated fixture for each case. Supply the
minimum raw artifacts required, realistic project instructions, and clear limits
on permitted resources and side effects. Keep generated artifacts out of the
working project and prevent earlier runs from contaminating later ones.

When independent evaluation is available, give the executing agent the request,
skill, and raw fixture without the expected answer, suspected failure, or proposed
fix. Give the reviewer the success criteria and actual outputs separately. Respect
the primary agent's delegation limits and permissions; independence is useful
evidence, not a requirement to launch extra processes or unapproved services.

Record the skill version or content identity, case inputs, environment, actual
tool use, output artifacts, and outcome. If comparing a baseline, use equivalent
inputs and conditions. Do not attribute differences to the skill when other
material conditions changed.

## Judge and improve

Inspect actual results against the predefined criteria. Separate selection errors,
instruction problems, task ambiguity, and environment failures. An unavailable
tool is a limitation of the run, not evidence that the intended workflow passed.

Label static inspection, simulated walkthroughs, and actual agent executions
accurately. If no execution facility is available, provide a static review and
unrun cases; do not claim behavioral validation. Report which cases ran, their
outcomes, consequential limitations, and the confidence those cases support.

Change instructions only when an observed failure or clear contradiction supports
the change. Recheck affected cases after a revision and keep claims bounded to
the scenarios exercised. Do not turn every isolated example into a universal rule.

The primary agent consolidates shared results. Reuse the relevant evaluation
record, honoring the configured North output directory or using
`<project-root>/north/evaluations/`; use `north-sources` when available without
requiring its installation. In a read-only task, review existing evidence and
return proposed cases or changes without writing fixtures or running mutations.
