---
name: domain-modeling
description: Clarify product terminology, entities, states, relationships, and business invariants when inconsistent meanings or rules affect a task. Use for domain ambiguity or explicit modeling work, not to turn ordinary implementation details or working preferences into a new model.
---

# Domain modeling

Build the smallest useful account of what the product's concepts mean and which
rules must hold. Model the part of the domain needed for the task; do not invent
a comprehensive ontology or rename the codebase to match a new vocabulary.

## Recover the existing meaning

Read relevant user decisions, project documentation, OpenSpec artifacts, North
context, code, and tests. Follow a concrete user action through the affected
entities and states. Distinguish documented requirements from observed behavior
and inferred intent; a database column or class name is evidence, not a definition.

Find ambiguous terms, competing names for one concept, and one name used for
different concepts. Preserve valid differences between contexts. For example,
an order being cancelled and a payment being refunded may describe separate
events with different rules; do not merge them because they often happen together.

## Capture decisions that change behavior

Use only the model elements needed to resolve the task:

- Terms with concise meanings, relevant aliases, and the context where each applies.
- Entities with identity and ownership, plus relationships that affect behavior.
- States and permitted transitions, including their preconditions and outcomes.
- Invariants expressed as observable rules, with relevant exceptions and examples.
- Consequential decisions with their reason, source, status, and affected behavior.

Make an invariant checkable. Instead of "invites are safe," describe who can
redeem an invite, when it expires, and whether it can be redeemed again, but do
not invent those policies if the evidence does not establish them. Label an
unconfirmed interpretation as proposed and ask only about unresolved meanings
that change implementation or validation. Keep independent work moving.

Check the model against a representative example and a meaningful boundary case.
Record contradictions between requirements and current behavior so implementation
can resolve them explicitly. Do not let a model silently change accepted behavior.

## Keep one authoritative record

Product terms, business invariants, and design decisions are task or domain
knowledge. They are distinct from preferences about how agents should work;
repetition does not turn a business rule into a learned preference skill.

Use the existing glossary, specification, or design document when it owns these
facts, and follow the project's OpenSpec workflow where applicable. Persist only
when useful and within the task's write scope. If a supporting North record is
needed, load `north-sources` and apply the shared artifact rules. Include source
paths and explicitly replace superseded decisions.
