# Execution Plan Format

North plans are strict YAML or JSON documents. Unknown fields, multiple YAML
documents, multiple JSON values, unsafe paths, invalid dependencies, and cycles
are rejected before execution. The JSON Schema is at
`schemas/north-plan.schema.json`.

```yaml
apiVersion: north/v1alpha1
kind: ExecutionPlan
metadata:
  name: add-health-check
spec:
  goal: Add an HTTP health check
  baseRef: main
  policy:
    maxParallel: 2
    failFast: true
    maxAttemptsPerStage: 1
    finalVerificationRequired: true
  stages:
    - id: server
      title: Implement endpoint
      description: Add the health handler and route.
      dependsOn: []
      agent: north-worker
      writeScope: [internal/server]
      acceptance:
        - id: server-tests
          type: command
          command: [go, test, ./internal/server]
          timeout: 2m
    - id: docs
      title: Document endpoint
      description: Document the health endpoint.
      dependsOn: [server]
      agent: north-worker
      writeScope: [docs/api.md]
      acceptance:
        - id: docs-exist
          type: file-exists
          path: docs/api.md
```

## Defaults

Omitted policy fields default to `maxParallel: 1`, `failFast: true`,
`maxAttemptsPerStage: 1`, and `finalVerificationRequired: true`. Explicit boolean
values are preserved. Duration values are Go duration strings such as `30s` or
`2m`; deterministic JSON serialization also writes duration strings.

## Acceptance Types

`command` and `exec` require a non-empty argument array. `shell` requires exactly
one command string and is subject to the verifier's shell policy. `file-exists`
and `file-not-exists` require a path. `contains` and `matches` require a path and
a non-empty value; `matches` also validates the regular expression.
`git-diff-not-empty` accepts an optional base ref in `value`. Every type may set
a timeout. Acceptance IDs must be unique within their stage.

## Safety and Validation

Stage IDs are unique and dependencies must name another stage. Self-dependencies
and dependency cycles are invalid. Paths in write scopes and acceptance criteria
must be repository-relative and cannot contain a `..` segment. Both `/` and `\`
are treated as separators during validation.

Agent references are optional, allowing the runtime's default agent to be used.
Applications may provide resolvers to validate base refs, agents, and provider-qualified agent references such as
`provider:agent`. Resolver checks are optional so plans remain parseable without
a checked-out repository or configured runtime.

Warnings identify overlapping or repository-wide write scopes, stages with no
acceptance criteria, stages that allow no changes or do not declare a write
scope, graphs that cannot execute any stages in parallel, and stages whose
combined write scopes and acceptance checks exceed 12 recovery units.

## Determinism and Approval

Serialization preserves the plan's declared order and always uses a stable field
order. Graph traversal breaks ties lexically by stage ID. The approval hash is a
lowercase SHA-256 digest of compact canonical JSON after defaults are applied and
stages, dependencies, write scopes, and acceptance criteria are sorted by ID or
value. Reordering those semantically unordered collections does not change the
hash.
