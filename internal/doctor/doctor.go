// Package doctor performs read-only diagnostics for a North installation.
package doctor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/SBakolis/north/internal/application"
	"github.com/SBakolis/north/internal/config"
	"github.com/SBakolis/north/internal/install"
	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/platform"
	"github.com/SBakolis/north/internal/plugins"
	"github.com/SBakolis/north/internal/state"
	"gopkg.in/yaml.v3"
)

type Severity string

const (
	Pass     Severity = "pass"
	Advisory Severity = "advisory"
	Error    Severity = "error"
)

type Check struct {
	ID       string   `json:"id"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

type Report struct {
	SchemaVersion int            `json:"schemaVersion"`
	NorthVersion  string         `json:"northVersion"`
	Platform      string         `json:"platform"`
	Paths         platform.Paths `json:"paths"`
	Checks        []Check        `json:"checks"`
	Healthy       bool           `json:"healthy"`
}

var openSpecCheckTimeout = 5 * time.Second

func Run(ctx context.Context, version string, paths platform.Paths, projectDir string) Report {
	report := Report{SchemaVersion: 1, NorthVersion: version, Platform: runtime.GOOS + "/" + runtime.GOARCH, Paths: paths, Healthy: true}
	add := func(id string, severity Severity, message string) {
		report.Checks = append(report.Checks, Check{ID: id, Severity: severity, Message: message})
		if severity == Error {
			report.Healthy = false
		}
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		add("platform", Pass, report.Platform+" is supported")
	} else {
		add("platform", Error, report.Platform+" is unsupported; use macOS, Linux, or WSL2")
	}
	for _, command := range []struct {
		id   string
		name string
		args []string
	}{{"git", "git", []string{"--version"}}, {"opencode", "opencode", []string{"--version"}}} {
		output, err := exec.CommandContext(ctx, command.name, command.args...).CombinedOutput()
		if err != nil {
			add(command.id, Error, fmt.Sprintf("%s unavailable: %v", command.name, err))
		} else {
			add(command.id, Pass, strings.TrimSpace(string(output)))
		}
	}
	if output, err := exec.CommandContext(ctx, "opencode", "run", "--help").CombinedOutput(); err != nil {
		add("opencode-run-flags", Error, fmt.Sprintf("cannot inspect opencode run: %v", err))
	} else {
		var missing []string
		for _, flag := range []string{"--dir", "--agent", "--format", "--session"} {
			if !bytes.Contains(output, []byte(flag)) {
				missing = append(missing, flag)
			}
		}
		if len(missing) > 0 {
			add("opencode-run-flags", Error, "missing required flags: "+strings.Join(missing, ", "))
		} else {
			add("opencode-run-flags", Pass, "required non-interactive flags are available")
		}
	}
	status, err := install.Inspect(paths)
	if err != nil {
		add("installation", Error, err.Error())
	} else if !status.Installed {
		add("installation", Advisory, "North is not installed")
	} else if !status.Healthy {
		for _, issue := range status.Issues {
			add("installation", Error, issue)
		}
	} else {
		add("installation", Pass, "manifest and managed files are healthy")
	}
	if status.Installed {
		checkConfig(paths, add)
		checkInstructionsAndHook(paths, status, add)
		checkOpenCodeConfigs(paths, add)
		if contains(status.Components, "knowledge.openspec") {
			output, err := localOpenSpecVersion(ctx)
			if err != nil {
				add("openspec", Error, fmt.Sprintf("selected provider unavailable: %v", err))
			} else if strings.TrimSpace(string(output)) == "" {
				add("openspec", Error, "selected provider returned an empty version")
			} else {
				add("openspec", Pass, strings.TrimSpace(string(output)))
			}
		}
	}
	for id, path := range map[string]string{"config-path": paths.ConfigDir, "state-path": paths.StateDir, "cache-path": paths.CacheDir, "data-path": paths.DataDir} {
		if err := writableAncestor(path); err != nil {
			add(id, Error, err.Error())
		} else {
			add(id, Pass, path)
		}
	}
	if projectDir != "" {
		command := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
		command.Dir = projectDir
		if output, err := command.CombinedOutput(); err != nil {
			add("repository", Advisory, "current directory is not a Git repository")
		} else {
			root := strings.TrimSpace(string(output))
			add("repository", Pass, root)
			checkRunState(ctx, paths, root, add)
		}
	}
	return report
}

func localOpenSpecVersion(ctx context.Context) ([]byte, error) {
	checkCtx, cancel := context.WithTimeout(ctx, openSpecCheckTimeout)
	defer cancel()
	command := exec.CommandContext(checkCtx, "npx", "--no-install", "openspec", "--version")
	command.Env = append(os.Environ(), "npm_config_offline=true", "npm_config_yes=false", "NO_UPDATE_NOTIFIER=1")
	boundCommand(command)
	return command.CombinedOutput()
}

func checkRunState(ctx context.Context, paths platform.Paths, repositoryRoot string, add func(string, Severity, string)) {
	projectID := application.ProjectID(repositoryRoot)
	runsRoot := filepath.Join(paths.StateDir, "projects", projectID, "runs")
	lockPath := filepath.Join(runsRoot, ".locks", "repository.lock")
	if metadata, err := readLock(lockPath); err == nil {
		if state.OwnerAlive(metadata) {
			add("repository-lock", Advisory, fmt.Sprintf("active owner pid %d since %s", metadata.PID, metadata.CreatedAt.Format(time.RFC3339)))
		} else {
			add("repository-lock", Error, "stale repository lock owned by dead process")
		}
	} else if !os.IsNotExist(err) {
		add("repository-lock", Error, err.Error())
	}
	entries, err := os.ReadDir(runsRoot)
	if os.IsNotExist(err) {
		checkManagedWorktrees(ctx, paths, projectID, repositoryRoot, nil, add)
		return
	}
	if err != nil {
		add("run-state", Error, err.Error())
		return
	}
	knownWorktrees := make(map[string]bool)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(runsRoot, entry.Name(), "run.json"))
		if err != nil {
			add("run-state", Error, fmt.Sprintf("run %s: %v", entry.Name(), err))
			continue
		}
		var run model.RunState
		if err := json.Unmarshal(data, &run); err != nil {
			add("run-state", Error, fmt.Sprintf("run %s has corrupt JSON: %v", entry.Name(), err))
			continue
		}
		for _, issue := range validateDurableRunFiles(runsRoot, run) {
			add("run-state", Error, fmt.Sprintf("run %s: %v", run.ID, issue))
		}
		if run.IntegrationWorkspace != "" {
			knownWorktrees[filepath.Clean(run.IntegrationWorkspace)] = true
		}
		for _, stage := range run.Stages {
			if stage.Workspace != "" {
				knownWorktrees[filepath.Clean(stage.Workspace)] = true
			}
		}
		for _, orphan := range state.ClassifyOrphanedStages(run, time.Now(), 10*time.Minute) {
			add("orphaned-stage", Error, fmt.Sprintf("run %s stage %s has been %s for %s", run.ID, orphan.Stage.ID, orphan.Stage.Status, orphan.StaleFor.Round(time.Second)))
		}
		if run.Status == model.RunCompleted || run.Status == model.RunCancelled {
			var retained []string
			for _, stage := range run.Stages {
				if stage.Workspace != "" {
					if _, err := os.Stat(stage.Workspace); err == nil {
						retained = append(retained, "worktree "+stage.Workspace)
					}
				}
				if stage.Branch != "" {
					command := exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+stage.Branch)
					command.Dir = repositoryRoot
					if command.Run() == nil {
						retained = append(retained, "branch "+stage.Branch)
					}
				}
			}
			if len(retained) > 0 {
				add("cleanup-candidate", Advisory, fmt.Sprintf("run %s retains %s; run `north cleanup %s`", run.ID, strings.Join(retained, ", "), run.ID))
			}
		}
		runLock := filepath.Join(runsRoot, entry.Name(), ".run.lock")
		if metadata, err := readLock(runLock); err == nil && !state.OwnerAlive(metadata) {
			add("run-lock", Error, fmt.Sprintf("run %s has a stale lock", entry.Name()))
		} else if err != nil && !os.IsNotExist(err) {
			add("run-lock", Error, fmt.Sprintf("run %s: %v", entry.Name(), err))
		}
	}
	checkManagedWorktrees(ctx, paths, projectID, repositoryRoot, knownWorktrees, add)
}

func validateDurableRunFiles(runsRoot string, run model.RunState) []error {
	runDir := filepath.Join(runsRoot, run.ID)
	var issues []error
	for _, aggregate := range run.Stages {
		data, err := os.ReadFile(filepath.Join(runDir, "stages", aggregate.ID+".json"))
		if err != nil {
			issues = append(issues, fmt.Errorf("stage %s snapshot: %w", aggregate.ID, err))
			continue
		}
		var stage model.StageState
		if err := json.Unmarshal(data, &stage); err != nil {
			issues = append(issues, fmt.Errorf("stage %s snapshot: %w", aggregate.ID, err))
		} else if !reflect.DeepEqual(stage, aggregate) {
			issues = append(issues, fmt.Errorf("stage %s snapshot differs from run aggregate", aggregate.ID))
		}
	}
	file, err := os.Open(filepath.Join(runDir, "events.ndjson"))
	if os.IsNotExist(err) {
		return issues
	}
	if err != nil {
		return append(issues, fmt.Errorf("event log: %w", err))
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var sequence uint64
	for scanner.Scan() {
		var event model.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			issues = append(issues, fmt.Errorf("event log sequence %d: %w", sequence+1, err))
			break
		}
		sequence++
		if event.Sequence != sequence {
			issues = append(issues, fmt.Errorf("event sequence is %d, expected %d", event.Sequence, sequence))
			break
		}
	}
	if err := scanner.Err(); err != nil {
		issues = append(issues, fmt.Errorf("event log: %w", err))
	}
	return issues
}

func checkManagedWorktrees(ctx context.Context, paths platform.Paths, projectID, repositoryRoot string, known map[string]bool, add func(string, Severity, string)) {
	command := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	command.Dir = repositoryRoot
	output, err := command.Output()
	if err != nil {
		add("git-worktrees", Error, err.Error())
		return
	}
	managedRoot := filepath.Join(paths.CacheDir, "worktrees", projectID)
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		worktree := filepath.Clean(strings.TrimPrefix(line, "worktree "))
		relative, err := filepath.Rel(managedRoot, worktree)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		if !known[worktree] {
			add("unsafe-worktree", Error, "untracked North worktree "+worktree+" requires manual inspection")
			continue
		}
		status := exec.CommandContext(ctx, "git", "status", "--porcelain=v1", "--untracked-files=all")
		status.Dir = worktree
		if dirty, err := status.Output(); err != nil {
			add("unsafe-worktree", Error, fmt.Sprintf("inspect %s: %v", worktree, err))
		} else if len(dirty) > 0 {
			add("unsafe-worktree", Error, "managed worktree has uncommitted changes: "+worktree)
		}
	}
}

func readLock(path string) (state.LockMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return state.LockMetadata{}, err
	}
	var metadata state.LockMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return metadata, fmt.Errorf("invalid lock %s: %w", path, err)
	}
	return metadata, nil
}

func checkConfig(paths platform.Paths, add func(string, Severity, string)) {
	data, err := os.ReadFile(filepath.Join(paths.ConfigDir, "config.yaml"))
	if err != nil {
		add("north-config", Error, err.Error())
		return
	}
	_, warnings, err := config.Parse(data)
	if err != nil {
		add("north-config", Error, err.Error())
		return
	}
	for _, warning := range warnings {
		add("north-config-warning", Advisory, warning.Message)
	}
	var decoded map[string]any
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		add("north-config", Error, "invalid YAML: "+err.Error())
		return
	}
	if _, exists := decoded["skills"]; exists {
		add("north-config", Error, "unsupported skills key is present")
		return
	}
	if _, exists := decoded["mcp"]; exists {
		add("north-config", Error, "unsupported mcp key is present")
		return
	}
	add("north-config", Pass, "configuration conforms to the North schema")
}

func checkInstructionsAndHook(paths platform.Paths, status install.Status, add func(string, Severity, string)) {
	manifest, err := install.LoadManifest(install.ManifestPath(paths))
	if err != nil {
		return
	}
	if manifest.Instructions.PrivateSnapshotPath != "" && manifest.Instructions.SourceKind != "symlink" {
		data, err := os.ReadFile(manifest.Instructions.PrivateSnapshotPath)
		if err != nil {
			add("instruction-backup", Error, err.Error())
		} else if install.FileSHA256(data) != manifest.Instructions.SourceSHA256 {
			add("instruction-backup", Error, "private instruction snapshot hash does not match the manifest")
		} else {
			add("instruction-backup", Pass, "private instruction snapshot is intact")
		}
	}
	backupPaths := append([]string(nil), manifest.Instructions.StableBackupPaths...)
	if manifest.Instructions.StableBackupPath != "" && !contains(backupPaths, manifest.Instructions.StableBackupPath) {
		backupPaths = append(backupPaths, manifest.Instructions.StableBackupPath)
	}
	for _, path := range backupPaths {
		if _, err := os.Stat(path); err != nil {
			add("instruction-backup", Error, fmt.Sprintf("stable backup %s: %v", path, err))
		}
	}
	if !contains(status.Components, "parallelization") {
		return
	}
	hook := filepath.Join(paths.OpenCodeDir, "plugins", "north-guardrails.ts")
	data, err := os.ReadFile(hook)
	if err != nil {
		add("north-hook", Error, err.Error())
		return
	}
	text := string(data)
	if !strings.Contains(text, "export const NorthGuardrails") || !strings.Contains(text, `"tool.execute.before"`) || strings.Count(text, "{") != strings.Count(text, "}") || strings.Count(text, "(") != strings.Count(text, ")") {
		add("north-hook", Error, "North guardrail hook failed structural parse checks")
		return
	}
	if _, err := exec.LookPath("node"); err != nil {
		add("north-hook-parse", Advisory, "Node.js is unavailable; North guardrail hook passed structural checks only")
		return
	}
	command := exec.Command("node", "--check", "--input-type=module")
	command.Stdin = bytes.NewReader(data)
	if output, err := command.CombinedOutput(); err != nil {
		add("north-hook", Error, fmt.Sprintf("North guardrail hook does not parse: %v: %s", err, strings.TrimSpace(string(output))))
		return
	}
	add("north-hook", Pass, "North guardrail hook parses successfully")
}

func checkOpenCodeConfigs(paths platform.Paths, add func(string, Severity, string)) {
	checked := false
	valid := true
	candidates := []string{
		os.Getenv("OPENCODE_CONFIG"), os.Getenv("OPENCODE_TUI_CONFIG"),
		filepath.Join(paths.OpenCodeDir, "opencode.jsonc"), filepath.Join(paths.OpenCodeDir, "opencode.json"),
		filepath.Join(paths.OpenCodeDir, "tui.jsonc"), filepath.Join(paths.OpenCodeDir, "tui.json"),
	}
	seen := make(map[string]bool)
	for _, path := range candidates {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		checked = true
		if err != nil {
			valid = false
			add("opencode-config", Error, err.Error())
		} else if err := plugins.ValidateConfig(data); err != nil {
			valid = false
			add("opencode-config", Error, fmt.Sprintf("%s: %v", path, err))
		}
	}
	if checked && valid {
		add("opencode-config", Pass, "OpenCode candidate configuration parses as JSON/JSONC")
	} else {
		add("opencode-config", Advisory, "no OpenCode JSON/JSONC candidate configuration exists")
	}
}

func writableAncestor(path string) error {
	current := filepath.Clean(path)
	for {
		info, err := os.Stat(current)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("%s is not a directory", current)
			}
			if info.Mode().Perm()&0o200 == 0 {
				return fmt.Errorf("%s is not owner-writable", current)
			}
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return fmt.Errorf("no existing parent for %s", path)
		}
		current = parent
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
