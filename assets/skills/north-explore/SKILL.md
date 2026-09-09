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

Return findings for the primary to save in the selected plan's research record,
with links from the index and affected tasks. Without a feature plan, use the
standalone research location from `north-sources`. Follow `research` for source
selection, citations, freshness checks, and unavailable evidence.
Do not install libraries or implement the feature during exploration.
