---
description: Clarify and explore a feature, then save a linked implementation plan
agent: build
subtask: false
---

Plan the user's requested change in the current working project:

$ARGUMENTS

Use the arguments together with the user's prompt and established conversation
context. If no goal can be established, ask what they want to build.

Load `north-plan` through the native skill tool and follow it. It must invoke
`invoke-memory`, `clarify-requirements`, and `north-explore`; exploration also
uses `research`. If a required skill is unavailable, identify it and explain
that enabling **North pipeline** in North's installer adds the commands,
pipeline skills, and their dependencies. Stop until it is available; do not
claim to have loaded unavailable guidance.

This is the planning phase: save the plan and research, without implementing the
feature. Finish with the plan's path, any unresolved decisions, and the concrete
next command: `/north-execute <feature-name>`. If decisions still block execution,
state that they must be resolved first. Suggest the command; do not run it.
