---
name: explain-code
description: Explain existing source code in standard technical English with verified file and line references, definitions of relevant symbols, and a walkthrough of control and data flow. Use when the user asks what a function, class, snippet, or code path does, how it works, or requests a line-by-line explanation. Do not apply merely to implementation, refactoring, or review requests that do not ask for an explanation.
---

# Explain code

Read the requested code and enough surrounding definitions and call sites to
explain its behavior accurately. Keep the explanation focused on the requested
part; do not modify code unless the user also asks for changes.

Use standard technical English: precise programming terms, direct sentences,
and concrete descriptions of behavior. Define unfamiliar terms when introduced.
Avoid metaphors, conversational filler, and vague descriptions such as "does
some magic." Preserve identifiers exactly as written in the source.

Start with the code's purpose, inputs, and outputs. Then walk through its
meaningful lines or blocks in execution order. Explain relevant declarations,
conditions, calls, state changes, return values, and error paths, including why
they affect the result. Group related lines unless the user requests each line
individually; do not merely translate syntax into prose.

Attach verified file paths and one-based line numbers to the explanation.
Define each important symbol at its declaration: what it represents, its type
when known, and its role. Reference the declaration's location when it is outside
the selected code. Use clickable source links where the environment supports
them. For a pasted snippet without source locations, label references as snippet
line numbers starting at 1; never invent repository paths or line numbers.

Distinguish behavior visible in the code from inferred intent or behavior of
uninspected dependencies. State uncertainty where it affects the explanation.
Use a short concrete input/output trace when it clarifies a non-obvious branch
or state transition, without claiming the example was executed unless it was.
