## Open Loop Interoperability

The `plan`, `north-planner`, `north-worker`, `north-verifier`, and
`north-conflict-resolver` agents must not be autonomously continued by Open Loop.
North workers must never invoke `/loop`; Open Loop is separate from North's DAG,
retry, liveness, and scheduling lifecycle.
