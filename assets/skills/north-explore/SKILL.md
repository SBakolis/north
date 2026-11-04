---
name: north-explore
description: Investigate similar implementations, best practices, and useful libraries to choose an approach for a North feature plan. Use during /north-plan or when a feature's implementation approach is uncertain.
---

# North explore

Load `research` for investigation methods and evidence standards. Frame the
investigation around the feature's goal, acceptance criteria, and constraints.

Cover the choices that can change this implementation:

- Analogous repository features and existing components that can be reused.
- Relevant external implementations and official guidance for the approach.
- Useful libraries, considering compatibility, maintenance, license constraints,
  and the cost of another dependency. Keeping existing dependencies is an option.
- The recommended approach, meaningful alternatives, and experiments or unresolved
  decisions that affect task boundaries and dependencies.

Load `north-sources` and its [OKF profile](../north-sources/references/okf-format.md)
for saved research. Return findings for the primary to save as `type: Research`,
linking the selected authoritative overview (`plan.md` for new plans, the
existing detailed index or single file for legacy plans) and affected tasks.
Include source resources, stable source IDs for claim footnotes, applicability,
observed freshness facts, and evidence gaps sufficient to author provenance.
Footnote labels must match source IDs, not source-list positions. Omit unknown
actors, dates, and trust data; investigation or authorship alone is not verification.
The primary maintains navigation listings lazily. Without a feature plan, use the
standalone research location from `north-sources`. Follow `research` for source
selection, citations, freshness checks, and unavailable evidence.
Do not install libraries or implement the feature during exploration.
