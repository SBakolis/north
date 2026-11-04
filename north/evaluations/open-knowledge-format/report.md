---
type: Skill Evaluation
title: Open Knowledge Format artifact workflow evidence
description: Bounded OpenCode fixture execution and independent artifact checks, with cross-runtime and historical evidence limits.
sources:
  - id: fixtures
    resource: raw-evidence.json
  - id: validation
    resource: validation.json
  - id: profile
    resource: ../../../assets/skills/north-sources/references/okf-format.md
---
# T05 evidence consolidation

## Scope and identity

Evaluated North's OKF v0.2 profile at kit commit `69746373ad37f009fc7d5fbd968ed8ef2a8783ab`, pinned upstream specification `ad30107c31c06aec8a7d5636e0d1058118604e6f`. Consolidation checkout: `north-okf-t05`, branch `feature/okf-t05`. Only this evaluation directory was written; no plan, skill, memory, or implementation repair was made. No commit or delegation occurred.

Primary-defined criteria precede execution in captured `criteria.json`. Native OpenCode general agents executed five bounded fixture workflows; the consolidator inspected all actual fixture files, not just their reports. Requests/responses and tool summaries are retained verbatim inside JSON. Exact guidance text and SHA-256 identities are captured from the assigned revision; identities recorded by the legacy/preferences producer match. Installed skill guidance used by this consolidator is not represented as the tested revision.[^fixtures]

Native sessions supplied by the primary:

- Fresh plan/research: `ses_e5745d897ffeZtDTDdQC1PfgIS`.
- Legacy memory/preferences: `ses_e5745d894ffeNuweiQRl5M3OpK`.
- Boundaries: `ses_e57429398ffepEtCK5mSGz2t7u`.

No prior project memories or generated preference skills were found in the supplied planning context. The authoritative T05 task, index, and research were read from the source checkout; no execution-plan changes were made.

## Observed results

| Case | Inspected outcome | Qualification |
| --- | --- | --- |
| Fresh plan | Four typed concepts and a listing index; schema/task status separated; unique T01/T02 graph, layers 1/2, synchronized table; final read-only verifier depends on implementation. | Planning only; no helper was implemented. Existing-evidence paths resolve; proposed code paths are explicitly prospective. |
| Research | Typed note, stable source ID and matching footnote; source exists outside bundle. | Producer's three identical SHA-256 snapshots support chat-only/read-only nonmutation; final bytes match. |
| Legacy directory and single-file | Legacy execution statuses, detailed index authority, blocked single-file authority, and `user_note` remain intact. Partial memory cites actual helper, checks, done T01 and unfinished T02. | No task dispatch/resumption through completion was exercised. Legacy/new mixture is not a conformant complete bundle. |
| Memory retrieval/follow-up | Before/after maps match; final memory and reciprocal handoff link resolve; legacy tasks unchanged. | Initial reciprocal-link omission followed an ambiguous no-shared-plan-edits constraint. Explicit follow-up authorized fixture plan edits and corrected it. Original report entries and hashes remain in JSON; this is not an unresolved skill defect. |
| Preferences | Three distinct supplied observations produce typed promoted preference and generated `SKILL.md` with native name/description, reciprocal evidence links, and no invented verification. | Actual direct file read in OpenCode; consumption under Claude core instructions was simulated in that same OpenCode session. No live Claude runtime or future parser task ran. |
| Boundaries | Minimal unknown type/extension and mapping-form machine verification remain readable; optional dangling bundle-root edge tolerated; source bundle-root path resolves. Missing T99 and invalid lifecycle/task state block execution; reserved log untouched. | Deliberately incomplete/invalid plans remain invalid, not repaired or counted as producer failures. Unknown extension retained after an authorized body edit. |
| Saved handoff/native requirements | Typed handoff and research links resolve; Log concept uses safe `input-policy-log.md`; native OpenSpec gains only requested policy paragraph, without YAML. | Handoff/research explicitly describe pre-edit state; later report records completion. Documentary policy does not establish implemented bool rejection. |

All newly emitted concepts have parsed YAML mappings with non-empty `type`; newly emitted indexes are listing-only. Source resources, footnote IDs/definitions, existing-evidence links and graph checks pass the targeted replay. No unexpected format finding or owning-skill repair is identified. Five expected findings remain: three legacy non-concepts, one legacy detailed index, and one deliberately invalid lifecycle field. The missing T99 is separately confirmed as a dispatch blocker. No full-conformance claim applies to the overall fixtures or this repository's mixed North root.[^validation]

## Reproduction and exact commands

Run from the assigned checkout root (macOS, Python 3, Ruby 2.6.10 / Psych 3.1.0; PyYAML was not installed or used):

```sh
python3 north/evaluations/open-knowledge-format/capture.py /private/var/folders/6r/9ct1j5t10yn29560jzlfdxd80000gn/T/opencode/north-okf-evals
ruby north/evaluations/open-knowledge-format/validate.rb > north/evaluations/open-knowledge-format/validation.json
git diff --check
```

Capture serializes 35 fixture files and 27 guidance files. Replay needs only the retained JSON and Ruby standard libraries; it never extracts legacy Markdown into the bundle or executes captured Python. The JSON contains exact runnable fixture `counter.py` / `check.py` and request texts for replay in a separately authorized isolated directory. Capture requires the original temporary fixtures; re-running it replaces the snapshot, so retain this evidence before a new run.

The fixture producer ran `python3 -B check.py` in `legacy-memory`: exit 0, `counter behavior passed`. Consolidation re-ran this same read-only command, with bytecode disabled. The check covers integer 3 and rejection of True, string `3`, and None, not exhaustive numeric behavior. Fresh-plan proposed tests were not executed because no implementation exists.[^fixtures]

The replay script is a bounded evidence check, not a public validator CLI or complete Markdown/OKF parser. YAML is genuinely parsed by Psych; regexes inspect the simple links/tables emitted in these fixtures. Human inspection additionally checks semantics, required acceptance/evidence sections, honest implementation boundaries, and trust descriptions.[^profile]

Consolidation outcomes: Ruby replay exits 0 with no failed assertions; the focused Python check exits 0; `git diff --check` exits 0 (new untracked evidence files are not covered by that Git check). The final JSON records every assertion. An initial consolidation assertion incorrectly added a second trailing newline when reconstructing the original legacy overview; testing zero/one/two trailing newlines against its historical SHA-256 established the original one-newline content. The script was corrected, not the fixture or expected hash; the rerun passes. No producer repair was needed.

## Evidence strength and remaining acceptance gaps

- Historical before/after hashes are producer-recorded temporal evidence; the consolidator verifies equality and current final hashes, not the past filesystem independently. Initial memory bytes and pre-follow-up report bytes are not retained, only their hashes and report history. The original legacy overview content is verified by removing the appended Handoff section and matching its initial hash. Unknown-field preservation is evidenced by unchanged legacy task hashes and the explicitly updated minimal concept retaining `custom_hint`.
- Raw session message/tool-event exports were not available through the supplied tools. This archive retains actual final bytes, raw fixture source files, guidance, criteria, and producer requests/responses/tool summaries, not a claim of complete session transcripts or every intermediate output. Replaying agent behavior is not deterministic.
- Missing optional indexes/root version were exercised. No emitted root `okf_version` declaration, unfamiliar-version consumption, or concept titled Index collision workflow was run. Existing index navigation and the executed Log collision cover related rules only; these additional behavioral cases remain unverified.
- Only OpenCode native-agent execution is observed. Both core instruction texts were read and archived, but Claude runtime loading is unverified. Automatic routing, arbitrary external skill validators, upstream viewer/reference-agent interoperability, Linux execution, and active-writer session recovery are unverified.
- Legacy single-file preparation stops at its missing contract. Missing prerequisite and ambiguous state tests demonstrate refusal and byte preservation, not successful recovery or migration. General preservation of every unknown metadata shape is not proven by these two extensions.
- T06/primary review must decide acceptance of these limits. This report neither marks T05 done nor certifies full upstream conformance.

[^fixtures]: [Captured criteria, fixture files, and producer reports](raw-evidence.json); filenames are keys under `files`, with text and SHA-256.
[^validation]: [Targeted validation results](validation.json), generated by the retained [Ruby script](validate.rb).
[^profile]: [Checked-out North OKF profile](../../../assets/skills/north-sources/references/okf-format.md); links leaving the North bundle are checkout evidence, not portable bundle contents.
