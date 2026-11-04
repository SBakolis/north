# Bounded replay of the legacy-plan extension regression, not a general validator.
require 'json'
require 'psych'
require 'digest'
require 'pathname'

raw = JSON.parse(File.read(File.join(__dir__, 'legacy-extension.json')))
files = raw.fetch('files')
files.each { |p, r| raise "hash #{p}" unless Digest::SHA256.hexdigest(r['text']) == r['sha256'] }
report = JSON.parse(files.fetch('report.json')['text'])
tasks = {}
files.each do |path, record|
  next unless path.include?('/tasks/')
  meta = Psych.safe_load(record['text'].match(/\A---\n(.*?)\n---\n/m)[1])
  raise 'new schema in legacy authority' if meta.key?('north_schema') || meta.key?('task_status')
  raise 'duplicate task' if tasks.key?(meta['id'])
  tasks[meta['id']] = meta
end
raise 'unexpected tasks' unless tasks.keys.sort == %w[T01 T02 T03]
raise 'existing state lost' unless tasks['T01']['status'] == 'done' && tasks['T01']['custom_evidence'] == 'retain-original'
raise 'dependency mismatch' unless tasks['T02']['depends_on'] == ['T01'] && tasks['T03']['depends_on'] == %w[T01 T02]
raise 'pending state lost' unless %w[T02 T03].all? { |id| tasks[id]['status'] == 'pending' }
raise 'verifier writes' unless tasks['T03']['write_scope'] == []
%w[base.txt north/plans/sample/tasks/T01-base.md].each do |path|
  raise 'input changed' unless files[path]['text'] == report['original_input_snapshots'][path]['content']
end
overview = files.fetch('north/plans/sample/index.md')['text']
raise 'authority converted' if files.key?('north/plans/sample/plan.md') || overview.start_with?('---')
raise 'execution started' if files.key?('result.txt')
raise 'preparation mutated files' unless report['second_request_hashes']['before'] == report['second_request_hashes']['after']
files.each do |path, record|
  next unless path.end_with?('.md')
  record['text'].scan(/\[[^\]\n]+\]\(([^)]+)\)/).flatten.each do |target|
    resolved = Pathname.new(File.join(File.dirname(path), target)).cleanpath.to_s
    raise "broken link #{target}" unless files.key?(resolved)
  end
end
raw['guidance'].each do |path, record|
  current = File.expand_path('../../../' + path, __dir__)
  raise "guidance drift #{path}" unless File.read(current) == record['text']
end
puts 'Legacy extension replay passed: retained authority/state/extensions, pending dependent tasks, preserved inputs, links and read-only preparation.'
