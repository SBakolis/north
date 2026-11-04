# Evaluation-specific replay over JSON bytes; not a public OKF validator.
require 'json'
require 'psych'
require 'digest'
require 'pathname'

raw = JSON.parse(File.read(File.join(__dir__, 'raw-evidence.json')))
files = raw.fetch('files')
checks = []
check = lambda do |name, value|
  checks << {'check' => name, 'passed' => !!value}
end
text = ->(p) { files.fetch(p).fetch('text') }
sha = ->(p) { Digest::SHA256.hexdigest(text.call(p)) }
resolve = lambda do |path, target|
  target = target.split('#').first
  next nil if target.nil? || target.empty? || target.match?(/\A[a-z]+:/i)
  if target.start_with?('/')
    Pathname.new(path.split('/').first + '/north' + target).cleanpath.to_s
  else
    Pathname.new(File.join(File.dirname(path), target)).cleanpath.to_s
  end
end
metadata = {}
issues = {}
links = []
files.each do |path, record|
  check.call("snapshot SHA-256 #{path}", sha.call(path) == record['sha256'])
  next unless path.include?('/north/') && path.end_with?('.md')
  content = record['text']
  match = content.match(/\A---\n(.*?)\n---\n/m)
  data = match ? Psych.safe_load(match[1], aliases: false) : nil
  metadata[path] = data
  body = match ? content[match.end(0)..-1] : content
  errors = []
  case File.basename(path)
  when 'index.md'
    root_index = path == path.split('/').first + '/north/index.md'
    errors << 'index frontmatter' if data && !(root_index && data == {'okf_version' => '0.2'})
    errors << 'index not listing-only' unless body.lines.all? { |l| l.strip.empty? || l.start_with?('#') || l.match?(/\A- \[[^\]]+\]\([^)]+\) - .+/) }
  when 'log.md'
    errors << 'log frontmatter' if data
    errors << 'log date heading absent' unless body.match?(/^## \d{4}-\d{2}-\d{2}$/)
  else
    errors << 'missing type' unless data.is_a?(Hash) && data['type'].is_a?(String) && !data['type'].strip.empty?
    if data.is_a?(Hash) && data['type'] && data.key?('status') && !%w[draft stable deprecated].include?(data['status'])
      errors << 'invalid lifecycle'
    end
  end
  sources = data.is_a?(Hash) ? data.fetch('sources', []) : []
  check.call("sources list #{path}", sources.is_a?(Array))
  ids = sources.map { |s| s['id'] }.compact
  check.call("unique source IDs #{path}", ids.uniq == ids)
  sources.each do |source|
    check.call("source resource #{path}:#{source['id']}", source['resource'].is_a?(String) && !source['resource'].empty?)
    target = resolve.call(path, source.fetch('resource'))
    check.call("existing source #{path}:#{source['id']}", target.nil? || files.key?(target))
  end
  refs = body.scan(/\[\^([^\]]+)\](?!:)/).flatten.uniq
  definitions = body.scan(/^\[\^([^\]]+)\]:/).flatten
  refs.each { |id| check.call("source footnote #{path}:#{id}", ids.include?(id) && definitions.include?(id)) }
  body.scan(/(?<!!)\[[^\]\n]+\]\(([^)]+)\)/).flatten.each do |target|
    resolved = resolve.call(path, target)
    next unless resolved
    exists = files.key?(resolved)
    optional = path == 'boundaries/north/memories/minimal.md' && target == '/research/future.md'
    links << {'from' => path, 'target' => target, 'resolved' => resolved, 'exists' => exists, 'optional' => optional}
    check.call("link #{path} -> #{target}", exists || optional)
  end
  issues[path] = errors unless errors.empty?
end
expected_issues = {
  'legacy-memory/north/plans/counter/index.md' => ['index not listing-only'],
  'legacy-memory/north/plans/counter/tasks/T01-helper.md' => ['missing type'],
  'legacy-memory/north/plans/counter/tasks/T02-docs.md' => ['missing type'],
  'legacy-memory/north/plans/old-single.md' => ['missing type'],
  'boundaries/north/plans/ambiguous/tasks/T01.md' => ['invalid lifecycle']
}
check.call('only deliberately legacy/invalid format findings', issues == expected_issues)

# Independent graph expectations derive from the feature request and case criteria.
tasks = metadata.select { |p, _| p.start_with?('fresh-plan/') && p.include?('/tasks/') }.values
by_id = tasks.to_h { |t| [t.fetch('id'), t] }
check.call('fresh unique IDs', by_id.length == tasks.length)
layers = {}
visit = lambda do |id, stack|
  raise "cycle or missing prerequisite #{id}" if stack.include?(id) || !by_id.key?(id)
  layers[id] ||= 1 + (by_id[id]['depends_on'].map { |dep| visit.call(dep, stack + [id]) }.max || 0)
end
by_id.each_key { |id| visit.call(id, []) }
check.call('fresh graph layers', layers == {'T01' => 1, 'T02' => 2})
tasks.each do |t|
  check.call("fresh task metadata #{t['id']}", t['north_schema'] == 'north-plan-v1' && t['task_status'] == 'pending' && !t.key?('status') && t['write_scope'].is_a?(Array) && !t['agent'].empty?)
  row = text.call('fresh-plan/north/plans/integer-increment/plan.md').lines.find { |l| l.start_with?("| #{t['id']} |") }
  cells = row.split('|').map(&:strip)
  check.call("task table agrees #{t['id']}", cells[3] == (t['depends_on'].empty? ? '—' : t['depends_on'].join(', ')) && cells[4].to_i == layers[t['id']] && cells[5] == t['task_status'])
end
check.call('fresh final review depends on implementation', by_id['T02']['depends_on'] == ['T01'] && by_id['T02']['write_scope'] == [] && by_id['T02']['agent'] == 'north-verifier')
check.call('fresh no implementation emitted', !files.key?('fresh-plan/counter.py'))
missing = metadata.fetch('boundaries/north/plans/missing/tasks/T01.md')
check.call('deliberate missing prerequisite T99', missing['depends_on'] == ['T99'] && !files.keys.any? { |p| p.include?('/missing/tasks/T99') })
check.call('blocked boundary emits no result', !files.key?('boundaries/result.txt'))
check.call('unknown field survives explicit body update', metadata.fetch('boundaries/north/memories/minimal.md')['custom_hint'] == 'retain-me')
check.call('legacy extension survives', metadata.fetch('legacy-memory/north/plans/counter/tasks/T01-helper.md')['user_note'] == 'preserve-this')
check.call('single mapping verification accepted', metadata.fetch('boundaries/north/memories/confirmed.md')['verified'] == {'by' => 'process:fixture-check', 'at' => '2026-11-01T10:00:00Z'})
legacy_helper = metadata.fetch('legacy-memory/north/plans/counter/tasks/T01-helper.md')
legacy_docs = metadata.fetch('legacy-memory/north/plans/counter/tasks/T02-docs.md')
check.call('legacy graph and execution status retained', legacy_helper['status'] == 'done' && legacy_helper['depends_on'] == [] && legacy_docs['status'] == 'pending' && legacy_docs['depends_on'] == ['T01'] && !legacy_helper.key?('task_status'))
check.call('legacy missing docs remain unfinished', !files.key?('legacy-memory/docs.md'))
check.call('single-file blocked authority retained', text.call('legacy-memory/north/plans/old-single.md').include?('| A | Investigate | — | 1 | blocked |') && !files.key?('legacy-memory/north/plans/old-single/index.md'))
skill = metadata.fetch('preferences/north/skills/focused-checks-first/SKILL.md')
check.call('generated native skill fields retained', skill['type'] == 'Agent Skill' && skill['name'] == 'focused-checks-first' && !skill['description'].empty?)
check.call('preference progress separate', metadata.fetch('preferences/north/dry/focused-checks-first.md')['preference_status'] == 'promoted' && !metadata.fetch('preferences/north/dry/focused-checks-first.md').key?('status'))

research = JSON.parse(text.call('research/report.json'))
check.call('reported research read-only snapshots equal', research['snapshots'].values.uniq.length == 1)
research['snapshots']['snapshot-3'].each { |p, h| check.call("research final hash #{p}", sha.call('research/' + p) == h) }
legacy = JSON.parse(text.call('legacy-memory/report.json'))
check.call('initial legacy retrieval maps equal', legacy['retrieval_hashes_before'] == legacy['retrieval_hashes_after'])
followup = legacy.fetch('followup')
original_overview = text.call('legacy-memory/north/plans/counter/index.md').split("\n## Handoff\n").first
check.call('legacy overview original content preserved before appended handoff', Digest::SHA256.hexdigest(original_overview) == legacy['retrieval_hashes_before']['north/plans/counter/index.md'])
check.call('followup legacy retrieval maps equal', followup['retrieval_hashes_before'] == followup['retrieval_hashes_after'])
followup['retrieval_hashes_after'].each do |p, h|
  next if p == 'report.json' # Report was subsequently updated; historical bytes not retained.
  check.call("legacy final hash #{p}", sha.call('legacy-memory/' + p) == h)
end
check.call('reciprocal link added in legacy authority', text.call('legacy-memory/north/plans/counter/index.md').include?('[partial implementation memory](../../memories/counter.md)'))
boundary = JSON.parse(text.call('boundaries/report.json'))
boundary['requests'][0]['hashes_before_and_after_read'].each { |p, h| check.call("boundary historical read-only #{p}", h['before'] == h['after']) }
boundary['requests'][1]['unchanged_sha256_before_after'].each { |p, h| check.call("boundary plans unchanged #{p}", sha.call('boundaries/' + p) == h) }
check.call('reserved log unchanged', sha.call('boundaries/north/research/log.md') == boundary['requests'][2]['preserved_history_sha256'])
check.call('minimal authorized update hash', sha.call('boundaries/north/memories/minimal.md') == boundary['requests'][4]['sha256_after_authorized_edit'])
check.call('OpenSpec remains native', !text.call('boundaries/openspec/specs/input/spec.md').start_with?('---') && sha.call('boundaries/openspec/specs/input/spec.md') == boundary['requests'][3]['sha256_after'])

raw['guidance'].each do |p, record|
  check.call("guidance hash #{p}", Digest::SHA256.hexdigest(record['text']) == record['sha256'])
end
legacy['skill_sha256'].each { |name, h| check.call("executed guidance identity #{name}", raw['guidance'].fetch("assets/skills/#{name}/SKILL.md")['sha256'] == h) }
prefs = JSON.parse(text.call('preferences/report.json'))
check.call('executed dry-skillify identity', raw['guidance'].fetch('assets/skills/dry-skillify/SKILL.md')['sha256'] == prefs['skill_sha256'])
prefs['tool_cores_read'].each { |tool, r| check.call("core identity #{tool}", raw['guidance'].fetch("assets/#{tool}/instructions/core.md")['sha256'] == r['sha256']) }
prefs['output_sha256'].each { |p, h| check.call("preference output #{p}", sha.call('preferences/' + p) == h) }

# Check the two actual Markdown deliverables, not just captured fixtures.
report = File.read(File.join(__dir__, 'report.md'))
front = Psych.safe_load(report.match(/\A---\n(.*?)\n---\n/m)[1], aliases: false)
check.call('evaluation report typed', front['type'] == 'Skill Evaluation')
front['sources'].each do |s|
  check.call("evaluation source #{s['id']}", File.file?(File.expand_path(s['resource'], __dir__)) && report.include?("[^#{s['id']}]:"))
end
%w[report.md index.md].each do |p|
  content = File.read(File.join(__dir__, p))
  content.scan(/\[[^\]\n]+\]\(([^)]+)\)/).flatten.each do |target|
    check.call("deliverable link #{p}:#{target}", File.exist?(File.expand_path(target, __dir__)))
  end
  if p == 'index.md'
    check.call('evaluation index listing-only', content.lines.all? { |l| l.strip.empty? || l.start_with?('#') || l.match?(/\A- \[[^\]]+\]\([^)]+\) - .+/) })
  end
end

result = {'ruby' => RUBY_DESCRIPTION, 'psych' => Psych::VERSION, 'revision' => raw['revision'],
          'checks' => checks, 'format_findings_expected' => issues, 'links' => links, 'fresh_layers' => layers,
          'failures' => checks.reject { |c| c['passed'] },
          'limits' => ['Targeted fixture checks, not exhaustive OKF validation.', 'Historical before/after maps are producer-recorded, not independently observed temporal snapshots.', 'No root version fixture or index-title collision workflow was executed.']}
puts JSON.pretty_generate(result)
exit(result['failures'].empty? ? 0 : 1)
