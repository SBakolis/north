# ADR 0001: Core and Adapter Boundaries

Status: accepted

North's scheduler lifecycle is core product behavior. Prioritization, runtime,
isolation, verification, integration, state, and knowledge access are replaceable
behind small interfaces declared by their consumer.

This prevents OpenCode event formats, Git commands, OpenSpec artifact shapes, and
filesystem details from becoming scheduler concepts. The initial adapters will be
OpenCode CLI, Git worktrees, host verification, progressive Git integration,
filesystem state, and optional OpenSpec knowledge.
