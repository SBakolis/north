# Architecture

North separates kit composition from orchestration execution. The installer owns
OpenCode configuration, North assets, backups, and ownership metadata. The Go
runtime owns plan execution, worker isolation, verification, durable state, and
integration.

Core orchestration depends only on interfaces declared near the orchestration
consumer. OpenCode, Git, filesystem state, verification, and OpenSpec packages
implement those contracts from the outside. Adapter packages must not be imported
by the orchestration core.

## Dependency Direction

```text
CLI -> application composition -> orchestration contracts and core
                                      ^
                                      |
                    concrete adapters implement contracts
```

OpenSpec content is normalized into `knowledge.Snapshot`; OpenSpec-specific types
never enter plans, scheduler state, or worker contracts.
