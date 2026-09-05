# ADR 0002: Host-Owned State and Safety

Status: accepted

North treats model output as untrusted. The host validates changed paths, runs
approved acceptance commands, creates commits, serializes integration, and records
state outside the repository. User branches are updated only by an explicit final
integration command.
