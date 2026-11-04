# Open Knowledge Format evaluation

- [Evaluation report](report.md) - Scope, observed outcomes, replay commands, and unverified acceptance.
- [Raw evidence](raw-evidence.json) - Fixture inputs/final outputs, producer reports, criteria, session IDs, and guidance snapshots.
- [Validation results](validation.json) - Replayable targeted YAML, path, graph, and hash checks.
- [Command results](command-results.json) - Consolidator's focused Python check and Git whitespace-check results.
- [Capture script](capture.py) - Serialize isolated fixtures without adding legacy Markdown to this bundle.
- [Replay script](validate.rb) - Evaluation-specific Ruby Psych assertions over the captured JSON.
