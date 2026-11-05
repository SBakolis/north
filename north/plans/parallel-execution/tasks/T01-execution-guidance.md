---
type: North Task
north_schema: north-plan-v1
id: T01
task_status: done
depends_on: []
agent: north-worker
write_scope: [assets/skills/north-execute/SKILL.md, assets/skills/subagent-usage/SKILL.md]
---
# T01: Strengthen execution and delegation guidance

Plan: [Parallel execution](../plan.md)
Prerequisites: None.
Research: [Owner mapping and recommendation](../research.md)

## Outcome

Make compatible, worthwhile concurrent dispatch the clear preference, using concise operational guidance in the existing two owners.

## Acceptance

- Explain identifying ready independent tasks and launching the selected compatible set before waiting, bounded by configured concurrency and actual native capabilities.
- Clarify same-layer slot refill and useful non-conflicting primary work while workers run; preserve accepted prerequisites, layer barriers, serialized primary acceptance/progress/merges and primary ownership. Independent read-only review assignments may overlap when their inputs are stable and the concurrency budget permits.
- Distinguish shared immutable reads from write/read or shared-resource conflicts; disjoint paths/worktrees alone do not prove resource independence.
- Preserve direct execution for tiny tasks, serial dependency chains, explicit cap one, unavailable concurrency and interrupted-writer recovery. Do not require artificial task splitting, nested workers or fixed host-specific invocation syntax.
- Preserve `task_status`, legacy execution fields and existing plan/integration behavior; no default limit change or new schema.
- Review the two skills together against every example in the plan. Run diff/link/frontmatter checks; actual concurrency evidence belongs to T03.

## Execution

Dispatch/session: `ses_e5274772afferJNhUXFBIdp2PJ`, completed.
Checkout/branch/base commit: `/Users/stamatis/Documents/projects/north` / `main` / `6d548be`; shared checkout, primary plan edits disjoint from worker scope.
Changed files: Both scoped execution/delegation SKILL.md files.
Validation: Worker scoped diff check and Ruby YAML/unchanged-header/four-link check passed. Primary full diff review confirms retained default cap, layer barrier, native permissions, serialized acceptance and useful-overlap guidance.
Review/integration: Accepted after primary Ruby metadata/link check and staged diff check passed; committed on source main as `d32898f`. Behavioral evidence follows in T03.
Blockers: None known.

## History

Created during planning; no implementation started.
