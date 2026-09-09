---
name: north-explore
description: Explore similar implementations, best practices, and useful libraries for a North feature plan. Use during /north-plan or when a North implementation approach needs repository and external research.
---

# North explore

Load `research` and apply its source and version checks to the feature being
planned. Research should resolve concrete implementation choices within the
user's scope, not produce a broad technology survey.

1. Read the goal, constraints, relevant memories, repository patterns, dependency
   manifests, and existing requirements. Find analogous local implementations
   and identify what can be reused, extended, or must change.
2. Search for relevant external implementations, official guidance and best
   practices, and libraries that could solve the problem. Prefer original
   repositories and official documentation. Check compatibility with this
   project's versions, maintenance, licensing constraints, and the cost of an
   added dependency. A useful outcome can be retaining existing dependencies.
3. Compare the plausible approaches against acceptance criteria and conventions.
   Record the recommended approach, why it fits, meaningful alternatives, and
   any experiment or unresolved decision that affects the task graph. Separate
   observed facts from inference.
4. Return concise findings with local paths and direct external source links,
   version context, and retrieval dates where freshness matters. Honor browsing
   restrictions; if tools or sources are unavailable, state the gap and its
   impact instead of inventing research or treating it as completed.

The primary agent saves or updates the selected feature's `research.md` under
the resolved `north/plans/<feature>/` directory and links it to `index.md`.
Reference existing authoritative research instead of copying it. A standalone
exploration without a feature plan may use `north/research/`. Delegated or
read-only exploration returns findings and proposed updates for the primary.
Do not install libraries or implement the feature during exploration.
