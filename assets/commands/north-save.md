---
description: Save verified implementation knowledge for future North work
agent: build
subtask: false
---

Save implementation memory for the North feature selected here or in the
current conversation:

$ARGUMENTS

The selector may be a feature name, plan directory, or plan index path. Load
`north-save` through the native skill tool and follow it. If it is unavailable,
explain that enabling **North pipeline** in North's installer adds the commands,
pipeline skills, and their dependencies. Stop until it is available; do not
claim to have loaded it.

Finish with the saved memory paths and what was recorded. Suggest
`/north-plan <next feature prompt>` to begin the next implementation cycle.
If this feature is unfinished, suggest `/north-execute <feature-name>` to
finish it before starting the next cycle. Do not launch another phase yourself.
