---
name: diagnosing-bugs
description: Diagnose and fix reported failures using symptom-specific reproduction, competing explanations, and targeted regression checks. Use when behavior is incorrect or a failure has an uncertain cause.
---

# Diagnosing bugs

Treat the reported behavior as the target. A nearby suspicious line, a passing
test suite, or a plausible explanation does not establish that the bug is fixed.
Use the current task's acceptance criteria and authoritative plan when present.
Keep investigation and repairs within the assigned scope and execution permissions.
For a read-only diagnosis, return findings and proposed repairs without editing.

## Establish the failure

- Identify the triggering input or action, expected result, actual result, and
  relevant environment. Consult the existing implementation and available reports
  before asking the user for information the repository can supply.
- Reproduce the reported scenario through the closest practical observable
  boundary. Confirm that the failure matches the report, including state and
  timing when those affect the symptom.
- Reduce the case enough to isolate the cause while retaining the conditions
  that produce the failure. Keep a working reproduction available for comparison.
- When reproduction is unavailable, separate observed evidence from assumptions.
  Use logs, traces, or a focused model of the failing path, and state what these
  cannot establish. Request missing access or input only when progress depends on it.

## Test explanations

Trace the input and state until behavior first diverges from the expected result.
Form a small set of plausible explanations and choose a check that distinguishes
them. Inspect relevant history or dependencies when that would resolve a concrete
uncertainty; avoid collecting unrelated context.

Change one relevant condition at a time where practical. Record what each check
rules in or out, including environmental failures that invalidate an experiment.
Do not repeat an unchanged failing attempt. If a check adds no evidence, revise
the hypothesis or choose a different observation before retrying. When further
checks require unavailable information or exceed scope, report the blocker and
the specific evidence needed to continue instead of making speculative repairs.

## Repair and verify

- Correct the demonstrated cause with a bounded change. Do not hide the symptom
  by swallowing errors, relaxing expectations, or bypassing the failing behavior.
- Add a focused regression check when it will reliably distinguish the failure
  from correct behavior. For substantive code fixes, prefer observing that check
  fail for the original reason before applying the repair when practical.
- Exercise the original scenario after the repair, then relevant adjacent cases
  and project-required checks. Scale coverage to the affected behavior and risk.
- If the original environment remains unavailable, label verification as partial;
  a substitute reproduction does not prove the reported scenario now succeeds.

Return the demonstrated cause or remaining hypotheses, changed files, checks
actually performed and their outcomes, and unresolved limitations. Reference
existing task IDs and evidence where available. Delegated workers return evidence
to the primary agent, who owns shared plan updates and final acceptance.
