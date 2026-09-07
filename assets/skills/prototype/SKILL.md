---
name: prototype
description: Build a bounded experiment to answer an uncertain implementation or interaction question. Use when observing a runnable example will inform a decision, rather than for routine implementation with settled requirements.
---

# Prototype

Make the smallest experiment that can change a real decision. A prototype is
evidence about a question; its existence does not establish production readiness.

## Define the experiment

State the uncertainty, the alternatives under consideration, and the observable
result that would favor or reject each alternative. Reuse known requirements and
constraints; ask only about decisions that cannot be inferred from the project.

Set a practical limit on effort and scope. Choose representative inputs and an
environment capable of exposing the uncertainty. For example, a layout experiment
needs realistic content and viewport sizes; a concurrency experiment needs the
relevant overlapping operations rather than a static mockup.

Inspect existing project examples and tools before introducing dependencies.
Use an isolated scratch location or clearly bounded project files as appropriate.
Preserve user changes and stay within the authorized environment and resources.
If the task is read-only, propose the experiment without writing or running it.

## Build and observe

Implement only the path needed to observe the result. Keep shortcuts explicit,
including fake data, omitted error handling, simplified state, and missing scale.
Do not spend the experiment's budget polishing unrelated functionality.

Make the result reproducible with an entry point, required inputs, and concise
run instructions. Use the actual runtime or interaction when that is central to
the question. A screenshot cannot establish response time or state transitions;
a successful build cannot establish that the interaction works.

Run the relevant scenario and record what happened. Include counterexamples or
boundary conditions that could overturn the conclusion. If the environment
prevents observation, report the prototype as unverified and name the missing
check; do not replace observed results with expected behavior.

## Close the experiment

Report the question, observed result, decision it supports, and remaining limits.
An inconclusive result should identify why the experiment could not distinguish
the alternatives and whether another bounded attempt would be useful.

Identify generated files and whether they are disposable or proposed for reuse.
Do not delete work outside the experiment's scope. Explain any production work
needed before reuse, such as integration, accessibility, resilience, or realistic
performance checks, only where it matters to the experiment's result.

The primary agent owns retention and integration decisions under the existing
task and commit workflow. Creating a prototype does not by itself authorize a
commit, publication, deployment, or expansion into production implementation.

Reuse an existing experiment record when appropriate. Save supporting notes in
the configured North output directory or `<project-root>/north/prototypes/` when
writes are allowed. Keep runnable files in the location their tooling requires
and link to them. `north-sources`, when available, supplies the storage convention;
without it, resolve the project root from the established workspace or repository.
Delegated agents return evidence for the primary to consolidate into shared notes.
