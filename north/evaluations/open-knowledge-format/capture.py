"""Capture bounded fixture bytes as JSON, not live Markdown in the North bundle."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys

root = Path(sys.argv[1]).resolve()
checkout = Path(__file__).resolve().parents[3]
out = Path(__file__).parent
files = {}
for p in sorted(root.rglob('*')):
    if p.is_file():
        data = p.read_bytes()
        files[p.relative_to(root).as_posix()] = {
            'text': data.decode('utf-8'), 'sha256': hashlib.sha256(data).hexdigest()
        }
guidance = {}
for pattern in ['assets/skills/*/SKILL.md', 'assets/skills/north-sources/references/*.md',
                'assets/*/instructions/core.md']:
    for p in sorted(checkout.glob(pattern)):
        data = p.read_bytes()
        guidance[p.relative_to(checkout).as_posix()] = {
            'text': data.decode(), 'sha256': hashlib.sha256(data).hexdigest()
        }
result = {
    'fixture_root': str(root),
    'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=checkout, text=True).strip(),
    'sessions': {
        'fresh-plan/research': 'ses_e5745d897ffeZtDTDdQC1PfgIS',
        'legacy-memory/preferences': 'ses_e5745d894ffeNuweiQRl5M3OpK',
        'boundaries': 'ses_e57429398ffepEtCK5mSGz2t7u'
    },
    'provenance': 'Final fixture bytes inspected by consolidator; requests and historical tool/hash evidence are retained verbatim in agent report.json files, not independent session exports.',
    'files': files, 'guidance': guidance
}
(out / 'raw-evidence.json').write_text(json.dumps(result, indent=2) + '\n')
print(json.dumps({'files': len(files), 'guidance_files': len(guidance), 'revision': result['revision']}))
