# ADR 0003: Invoke OpenSpec Through npx

## Status

Accepted.

## Context

The implementation plan names an `openspec` executable. The project-level agent
instructions require OpenSpec commands to use `npx`, and installations commonly
provide OpenSpec as a project-local npm dependency rather than a global binary.

## Decision

North invokes OpenSpec exclusively as an argv array beginning with `npx
openspec`. It never invokes a bare `openspec` command and never interpolates
OpenSpec arguments into a shell command. Installation checks `npx openspec
--version`; the read-only doctor check uses `npx --no-install openspec --version`
with npm offline mode and a timeout so diagnostics cannot fetch or cache it.

## Consequences

OpenSpec can resolve from the project dependency graph without a global install.
`npx` becomes the executable dependency for this optional provider. This is a
deliberate deviation from the executable spelling in the implementation plan;
the provider contract and read-only behavior remain unchanged.
