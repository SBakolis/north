---
description: Implement a North plan with parallel subagents in dependency layers
agent: build
subtask: false
---

Execute the North plan selected by these arguments or the current conversation:

$ARGUMENTS

The selector may be a feature name, plan directory, or plan index path. Load
`north-execute` through the native skill tool and follow it, including
`invoke-memory` before implementation and `subagent-usage` for delegation.
If a required skill is unavailable, identify it and explain that rerunning
North's installer with **North pipeline** enabled adds the commands, pipeline
skills, and their dependencies. Stop until it is available; do not claim to
have loaded unavailable guidance.

Finish with the actual implementation and validation result and the concrete
next command: `/north-save <feature-name>`. If execution is incomplete, report
the unfinished task IDs and how to resume `/north-execute <feature-name>`;
make clear that `/north-save` is the next phase after completion. Suggest the
next phase; do not run it automatically.
