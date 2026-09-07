---
name: code-review
description: Review a defined code change for correctness, requirement coverage, and actionable regressions using repository context and validation evidence. Use for requested reviews or acceptance review of completed implementation.
---

# Code review

Evaluate the assigned change against its intended behavior and the surrounding
contracts. Use the authoritative requirements, plan task IDs, and supplied
evidence when present. Establish the review scope from the request and actual
diff; do not assume unrelated working-tree changes belong to the task.

A review request does not authorize repairs. Remain read-only unless a separate
implementation task is explicitly assigned. Respect the active agent's tool
permissions. In `north-verifier`, inspect files, permitted diffs and status, and
supplied evidence; ask the primary agent to execute missing checks. Do not edit
the execution plan, mark tasks done, run prohibited commands, or delegate further.
Other reviewers may run relevant checks only within their execution permissions
and the requested scope, accounting for checks that write generated artifacts.

## Establish enough context

Confirm the comparison base or changed-file set. Read the relevant requirements
and surrounding callers, data contracts, and tests needed to understand the diff.
Consult project instructions that apply to these files. If the baseline or a
required dependency is unavailable, explain the resulting review limit rather
than guessing about code or changes that cannot be inspected.

Use two complementary questions throughout the review:

- Does the change deliver the requested behavior, including relevant boundaries,
  error cases, and acceptance examples? Identify omissions as well as incorrect
  implementations; passing existing tests may leave a new requirement uncovered.
- Could the change break behavior that callers already depend on? Trace concrete
  affected paths, including state and data lifetime, compatibility, and failure
  handling where relevant to the diff.

Check that supplied validation covers those claims and applies to the reviewed
revision. Distinguish inspection, tests, and unsupported implementation claims.
Do not treat test presence, a worker report, or a clean diff as execution evidence.
Flag meaningful gaps without demanding unrelated checks or universal coverage.

## Return actionable findings

Report an issue when there is a concrete trigger, a demonstrable problematic path,
and a meaningful consequence. Include the relevant file and narrow line location,
expected behavior, observed defect or supporting reasoning, and impact-based
severity. Reference a task ID or requirement when it clarifies what is violated.

Keep uncertainty explicit. An unresolved question can request evidence without
being presented as a proven defect. Avoid hypothetical issues unsupported by the
code, duplicate findings with the same cause, and personal style preferences
unless they violate an applicable convention or materially affect the task.
Suggest a repair direction only when useful; avoid implementing it during review.

Lead the result with findings ordered by impact. Then state which acceptance
criteria are supported, which fail, and which remain unverified, citing actual
evidence. If no actionable findings remain, say so and retain material limits;
absence of findings does not prove unobserved behavior is correct. The primary
agent owns repairs, validation execution when needed, shared records, and the
final decision to accept delegated work.
