package install

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SBakolis/north/internal/platform"
	"github.com/SBakolis/north/internal/plugins"
)

type pluginTestFiles struct{ data map[string][]byte }

func (f *pluginTestFiles) ReadFile(path string) ([]byte, error) {
	data, ok := f.data[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

func (f *pluginTestFiles) WriteFile(path string, data []byte, _ fs.FileMode) error {
	f.data[path] = append([]byte(nil), data...)
	return nil
}

type pluginTestRunner func(context.Context, string, ...string) error

func (runner pluginTestRunner) Run(ctx context.Context, command string, args ...string) error {
	return runner(ctx, command, args...)
}

func testPaths(root string) platform.Paths {
	return platform.Paths{
		ConfigDir: filepath.Join(root, "config", "north"), DataDir: filepath.Join(root, "data", "north"),
		StateDir: filepath.Join(root, "state", "north"), CacheDir: filepath.Join(root, "cache", "north"),
		OpenCodeDir: filepath.Join(root, "config", "opencode"),
	}
}

func TestInstallIsIdempotentAndUninstallRestoresCanonicalInstructions(t *testing.T) {
	paths := testPaths(t.TempDir())
	if err := os.MkdirAll(paths.OpenCodeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte("Preserve this byte-for-byte.\n")
	active := filepath.Join(paths.OpenCodeDir, "AGENTS.md")
	if err := os.WriteFile(active, original, 0o600); err != nil {
		t.Fatal(err)
	}
	options := Options{Paths: paths, Version: "0.1.0", NonInteractive: true, Now: func() time.Time {
		return time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	}}
	if _, err := Install(options); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(filepath.Join(paths.OpenCodeDir, "AGENTS-backup.md"))
	if err != nil || !bytes.Equal(backup, original) {
		t.Fatalf("backup = %q, error = %v", backup, err)
	}
	generatedBefore, err := os.ReadFile(active)
	if err != nil || !strings.Contains(string(generatedBefore), managedBegin) || !strings.Contains(string(generatedBefore), string(original)) {
		t.Fatalf("generated instructions = %q, error = %v", generatedBefore, err)
	}
	manifestBefore, err := os.ReadFile(ManifestPath(paths))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Install(options); err != nil {
		t.Fatal(err)
	}
	generatedAfter, _ := os.ReadFile(active)
	manifestAfter, _ := os.ReadFile(ManifestPath(paths))
	if !bytes.Equal(generatedAfter, generatedBefore) || !bytes.Equal(manifestAfter, manifestBefore) {
		t.Fatal("identical reinstall changed generated content or manifest")
	}
	for _, name := range []string{"north-planner.md", "north-worker.md", "north-verifier.md", "north-conflict-resolver.md"} {
		info, err := os.Lstat(filepath.Join(paths.OpenCodeDir, "agents", name))
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("agent %s is not an installed symlink: %v", name, err)
		}
	}
	if _, err := Uninstall(paths, false); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(active)
	if err != nil || !bytes.Equal(restored, original) {
		t.Fatalf("restored instructions = %q, error = %v", restored, err)
	}
	if _, err := os.Stat(ManifestPath(paths)); !os.IsNotExist(err) {
		t.Fatalf("manifest remains after uninstall: %v", err)
	}
}

func TestReconfigureMarkerConflictWritesProposalAndPreservesActive(t *testing.T) {
	paths := testPaths(t.TempDir())
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true}); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(paths.OpenCodeDir, "AGENTS.md")
	userContent := []byte("replacement user instructions\n")
	if err := os.WriteFile(active, userContent, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true}); err == nil || !strings.Contains(err.Error(), "north-proposed") {
		t.Fatalf("error = %v", err)
	}
	got, err := os.ReadFile(active)
	if err != nil || !bytes.Equal(got, userContent) {
		t.Fatalf("instructions = %q, error = %v", got, err)
	}
	proposed, err := os.ReadFile(active + ".north-proposed")
	if err != nil || !bytes.Contains(proposed, userContent) || !bytes.Contains(proposed, []byte(managedBegin)) {
		t.Fatalf("proposed instructions = %q, error = %v", proposed, err)
	}
}

func TestReconfigureMergesEditsOutsideManagedBlock(t *testing.T) {
	paths := testPaths(t.TempDir())
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true}); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(paths.OpenCodeDir, "AGENTS.md")
	current, _ := os.ReadFile(active)
	current = append([]byte("user edit\n"), current...)
	if err := os.WriteFile(active, current, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Paths: paths, Version: "dev", Selected: []string{"core", "knowledge.none"}, NonInteractive: true}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(active)
	if !bytes.Contains(got, []byte("user edit")) {
		t.Fatalf("outside edit was lost: %q", got)
	}
}

func TestDeselectRemovesPreviouslyManagedAssets(t *testing.T) {
	paths := testPaths(t.TempDir())
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true}); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(paths.OpenCodeDir, "agents", "north-worker.md")
	if _, err := Install(Options{Paths: paths, Version: "dev", Selected: []string{"core", "knowledge.none"}, NonInteractive: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deselected asset remains: %v", err)
	}
	manifest, err := LoadManifest(ManifestPath(paths))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range manifest.ManagedFiles {
		if file.Path == destination {
			t.Fatalf("deselected asset remains owned: %+v", file)
		}
	}
}

func TestReconfigureRefusesModifiedManagedAsset(t *testing.T) {
	paths := testPaths(t.TempDir())
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true}); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(paths.OpenCodeDir, "agents", "north-worker.md")
	if err := os.Remove(destination); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("user replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Paths: paths, Version: "dev", Selected: []string{"core", "knowledge.none"}, NonInteractive: true}); err == nil || !strings.Contains(err.Error(), "modified managed path") {
		t.Fatalf("error = %v", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil || string(got) != "user replacement" {
		t.Fatalf("replacement = %q, error = %v", got, err)
	}
}

func TestEqualCanonicalAndLegacySourcesCreateBothBackups(t *testing.T) {
	paths := testPaths(t.TempDir())
	if err := os.MkdirAll(paths.OpenCodeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := []byte("same instructions\n")
	for _, name := range []string{"AGENTS.md", "AGENT.md"} {
		if err := os.WriteFile(filepath.Join(paths.OpenCodeDir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"AGENTS-backup.md", "AGENT-backup.md"} {
		got, err := os.ReadFile(filepath.Join(paths.OpenCodeDir, name))
		if err != nil || !bytes.Equal(got, content) {
			t.Fatalf("%s = %q, error = %v", name, got, err)
		}
	}
}

func TestSelectingLegacyPreservesAndRestoresBothDifferentInstructionFiles(t *testing.T) {
	paths := testPaths(t.TempDir())
	if err := os.MkdirAll(paths.OpenCodeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(paths.OpenCodeDir, "AGENTS.md")
	legacy := filepath.Join(paths.OpenCodeDir, "AGENT.md")
	if err := os.WriteFile(canonical, []byte("canonical rules\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("legacy rules\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Paths: paths, Version: "dev", AgentSource: "AGENT.md", NonInteractive: true}); err != nil {
		t.Fatal(err)
	}
	active, err := os.ReadFile(canonical)
	if err != nil || !bytes.Contains(active, []byte("legacy rules")) || bytes.Contains(active, []byte("canonical rules")) {
		t.Fatalf("selected legacy source was not authoritative: %q error=%v", active, err)
	}
	manifest, err := LoadManifest(ManifestPath(paths))
	if err != nil || len(manifest.Instructions.Originals) != 2 {
		t.Fatalf("manifest originals = %+v error=%v", manifest.Instructions.Originals, err)
	}
	for name, want := range map[string]string{"AGENTS-backup.md": "canonical rules\n", "AGENT-backup.md": "legacy rules\n"} {
		got, err := os.ReadFile(filepath.Join(paths.OpenCodeDir, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q error=%v", name, got, err)
		}
	}
	if _, err := Uninstall(paths, false); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{canonical: "canonical rules\n", legacy: "legacy rules\n"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q error=%v", path, got, err)
		}
	}
}

func TestInstallRestoresDanglingCanonicalInstructionSymlink(t *testing.T) {
	paths := testPaths(t.TempDir())
	if err := os.MkdirAll(paths.OpenCodeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(paths.OpenCodeDir, "AGENTS.md")
	if err := os.Symlink("missing-instructions.md", active); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(paths, false); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(active)
	if err != nil || target != "missing-instructions.md" {
		t.Fatalf("restored symlink = %q error=%v", target, err)
	}
}

func TestUninstallRefusesManagedBlockEdits(t *testing.T) {
	paths := testPaths(t.TempDir())
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true}); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(paths.OpenCodeDir, "AGENTS.md")
	current, _ := os.ReadFile(active)
	changed := bytes.Replace(current, []byte("# North"), []byte("user text inside managed block\n# North"), 1)
	if err := os.WriteFile(active, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(paths, false); err == nil || !strings.Contains(err.Error(), "refusing uninstall") {
		t.Fatalf("error = %v", err)
	}
	got, err := os.ReadFile(active)
	if err != nil || !bytes.Equal(got, changed) {
		t.Fatalf("managed edit changed during refused uninstall: %q error=%v", got, err)
	}
}

func TestFirstInstallRefusesUnownedNorthMarkers(t *testing.T) {
	paths := testPaths(t.TempDir())
	if err := os.MkdirAll(paths.OpenCodeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(paths.OpenCodeDir, "AGENTS.md")
	original := []byte("user\n" + managedBegin + "\nunowned\n" + managedEnd + "\n")
	if err := os.WriteFile(active, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true}); err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("error = %v", err)
	}
	got, err := os.ReadFile(active)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("active instructions changed: %q error=%v", got, err)
	}
	if _, err := os.Stat(active + ".north-proposed"); err != nil {
		t.Fatalf("proposed file missing: %v", err)
	}
}

func TestUninstallRestoresCanonicalInstructionSymlink(t *testing.T) {
	paths := testPaths(t.TempDir())
	if err := os.MkdirAll(paths.OpenCodeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(paths.OpenCodeDir, "shared-instructions.md")
	if err := os.WriteFile(target, []byte("shared rules\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(paths.OpenCodeDir, "AGENTS.md")
	if err := os.Symlink(filepath.Base(target), active); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(paths, false); err != nil {
		t.Fatal(err)
	}
	got, err := os.Readlink(active)
	if err != nil || got != filepath.Base(target) {
		t.Fatalf("restored symlink = %q, error = %v", got, err)
	}
}

func TestInstallLegacyInstructionsAndPreserveLaterUserEdits(t *testing.T) {
	paths := testPaths(t.TempDir())
	if err := os.MkdirAll(paths.OpenCodeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(paths.OpenCodeDir, "AGENT.md")
	if err := os.WriteFile(legacy, []byte("legacy rules\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy source was not moved: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(paths.OpenCodeDir, "AGENT-backup.md")); err != nil || string(got) != "legacy rules\n" {
		t.Fatalf("legacy backup = %q, error = %v", got, err)
	}
	active := filepath.Join(paths.OpenCodeDir, "AGENTS.md")
	current, _ := os.ReadFile(active)
	current = append([]byte("user edit after install\n"), current...)
	if err := os.WriteFile(active, current, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(paths, false); err != nil {
		t.Fatal(err)
	}
	remaining, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(remaining), "user edit after install") || strings.Contains(string(remaining), managedBegin) {
		t.Fatalf("unexpected remaining instructions: %q", remaining)
	}
}

func TestUninstallUsesPrivateSnapshotWhenStableBackupPredatesNorth(t *testing.T) {
	paths := testPaths(t.TempDir())
	if err := os.MkdirAll(paths.OpenCodeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(paths.OpenCodeDir, "AGENTS.md")
	stable := filepath.Join(paths.OpenCodeDir, "AGENTS-backup.md")
	if err := os.WriteFile(active, []byte("current user rules\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stable, []byte("older unrelated backup\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(paths, false); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(active)
	if err != nil || string(got) != "current user rules\n" {
		t.Fatalf("restored instructions = %q, error = %v", got, err)
	}
	got, err = os.ReadFile(stable)
	if err != nil || string(got) != "older unrelated backup\n" {
		t.Fatalf("stable backup changed = %q, error = %v", got, err)
	}
}

func TestInstallDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	paths := testPaths(root)
	result, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Operations) == 0 {
		t.Fatal("dry-run returned no operations")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("dry-run created entries: %v", entries)
	}
}

func TestInstallRefusesUserOwnedAgentCollisionBeforeWriting(t *testing.T) {
	paths := testPaths(t.TempDir())
	agentDir := filepath.Join(paths.OpenCodeDir, "agents")
	if err := os.MkdirAll(agentDir, 0o700); err != nil {
		t.Fatal(err)
	}
	collision := filepath.Join(agentDir, "north-worker.md")
	if err := os.WriteFile(collision, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true}); err == nil || !strings.Contains(err.Error(), "user-owned") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(ManifestPath(paths)); !os.IsNotExist(err) {
		t.Fatalf("manifest written despite collision: %v", err)
	}
	if got, _ := os.ReadFile(collision); string(got) != "mine" {
		t.Fatalf("collision changed to %q", got)
	}
}

func TestInstallRefusesUnownedNorthConfig(t *testing.T) {
	paths := testPaths(t.TempDir())
	if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(paths.ConfigDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("user config\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true}); err == nil || !strings.Contains(err.Error(), "unowned North path") {
		t.Fatalf("error = %v", err)
	}
	if got, _ := os.ReadFile(configPath); string(got) != "user config\n" {
		t.Fatalf("config changed to %q", got)
	}
}

func TestInspectReportsManagedFileDamage(t *testing.T) {
	paths := testPaths(t.TempDir())
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true}); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(paths.OpenCodeDir, "plugins", "north-guardrails.ts")
	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	status, err := Inspect(paths)
	if err != nil {
		t.Fatal(err)
	}
	if status.Healthy || len(status.Issues) == 0 {
		t.Fatalf("status = %+v", status)
	}
}

func TestPluginsAreOffByDefaultAndDryRunDoesNotInvokeManager(t *testing.T) {
	paths := testPaths(t.TempDir())
	called := 0
	files := &pluginTestFiles{data: map[string][]byte{"config.jsonc": []byte(`{"plugin":[]}`)}}
	manager := plugins.NewManager(pluginTestRunner(func(context.Context, string, ...string) error {
		called++
		return nil
	}), files, plugins.Paths{Global: []string{"config.jsonc"}})
	result, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true, DryRun: true, PluginModules: []string{plugins.CodexMeter}, PluginManager: manager, PluginFiles: files})
	if err != nil {
		t.Fatal(err)
	}
	if called != 0 {
		t.Fatalf("dry-run invoked plugin manager %d times", called)
	}
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true, PluginManager: manager, PluginFiles: files}); err != nil {
		t.Fatal(err)
	}
	if called != 0 {
		t.Fatalf("default install invoked plugin manager %d times", called)
	}
	manifest, err := LoadManifest(ManifestPath(paths))
	if err != nil || len(manifest.Plugins) != 0 {
		t.Fatalf("default plugin manifest = %#v, error = %v", manifest.Plugins, err)
	}
	if !containsString(result.Manifest.Components, "plugin.opencode-codex-meter") {
		t.Fatalf("dry-run selected components = %v", result.Manifest.Components)
	}
}

func TestPluginInstallIsIdempotentAndDeselectRemovesOwnedRegistration(t *testing.T) {
	paths := testPaths(t.TempDir())
	configPath := "config.jsonc"
	tuiPath := "tui.jsonc"
	original := []byte("{\n  // keep this comment\n  \"plugin\": []\n}\n")
	files := &pluginTestFiles{data: map[string][]byte{configPath: append([]byte(nil), original...), tuiPath: append([]byte(nil), original...)}}
	var commands [][]string
	runner := pluginTestRunner(func(_ context.Context, command string, args ...string) error {
		commands = append(commands, append([]string{command}, args...))
		files.data[configPath] = []byte("{\n  // keep this comment\n  \"plugin\": [\"opencode-codex-meter\"]\n}\n")
		files.data[tuiPath] = []byte("{\n  // keep this comment\n  \"plugin\": [\"opencode-codex-meter\"]\n}\n")
		return nil
	})
	pluginPaths := plugins.Paths{Global: []string{configPath}, TUI: []string{tuiPath}}
	manager := plugins.NewManager(runner, files, pluginPaths)
	options := Options{Paths: paths, Version: "dev", NonInteractive: true, PluginModules: []string{plugins.CodexMeter}, PluginManager: manager, PluginFiles: files, PluginPaths: pluginPaths}
	if _, err := Install(options); err != nil {
		t.Fatal(err)
	}
	manifestBefore, err := os.ReadFile(ManifestPath(paths))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Install(options); err != nil {
		t.Fatal(err)
	}
	manifestAfter, _ := os.ReadFile(ManifestPath(paths))
	wantCommand := []string{"opencode", "plugin", plugins.CodexMeter, "--global"}
	if len(commands) != 1 || !reflect.DeepEqual(commands[0], wantCommand) {
		t.Fatalf("commands = %v, want [%v]", commands, wantCommand)
	}
	if !bytes.Equal(manifestBefore, manifestAfter) {
		t.Fatal("idempotent plugin install changed manifest")
	}
	manifest, err := LoadManifest(ManifestPath(paths))
	if err != nil || len(manifest.Plugins) != 1 || manifest.Plugins[0].PreExisting || len(manifest.Plugins[0].Owned) != 2 {
		t.Fatalf("plugin ownership = %#v, error = %v", manifest.Plugins, err)
	}
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true, PluginManager: manager, PluginFiles: files, PluginPaths: pluginPaths}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(files.data[configPath], original) {
		t.Fatalf("deselect did not preserve JSONC bytes: %q", files.data[configPath])
	}
	if !bytes.Equal(files.data[tuiPath], original) {
		t.Fatalf("deselect did not restore TUI JSONC bytes: %q", files.data[tuiPath])
	}
}

func TestPluginPreExistingRegistrationIsPreservedOnDeselect(t *testing.T) {
	paths := testPaths(t.TempDir())
	configPath := "config.jsonc"
	original := []byte("{\n  // custom\n  \"plugin\": [[\"@sbakolis/open-loop\", {\"mode\":\"custom\"}]]\n}\n")
	files := &pluginTestFiles{data: map[string][]byte{configPath: append([]byte(nil), original...)}}
	called := false
	pluginPaths := plugins.Paths{Global: []string{configPath}}
	manager := plugins.NewManager(pluginTestRunner(func(context.Context, string, ...string) error {
		called = true
		return nil
	}), files, pluginPaths)
	options := Options{Paths: paths, Version: "dev", NonInteractive: true, PluginModules: []string{plugins.OpenLoop}, PluginManager: manager, PluginFiles: files, PluginPaths: pluginPaths}
	if _, err := Install(options); err != nil {
		t.Fatal(err)
	}
	manifest, _ := LoadManifest(ManifestPath(paths))
	if called || len(manifest.Plugins) != 1 || !manifest.Plugins[0].PreExisting || len(manifest.Plugins[0].Owned) != 0 {
		t.Fatalf("pre-existing plugin ownership = %#v, called = %t", manifest.Plugins, called)
	}
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true, PluginManager: manager, PluginFiles: files, PluginPaths: pluginPaths}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(files.data[configPath], original) {
		t.Fatalf("pre-existing registration changed: %q", files.data[configPath])
	}
}

func TestUninstallRemovesOnlyUnchangedOwnedPluginRegistration(t *testing.T) {
	paths := testPaths(t.TempDir())
	configPath := "config.jsonc"
	original := []byte("{\n  // retained\n  \"plugin\": []\n}\n")
	files := &pluginTestFiles{data: map[string][]byte{configPath: append([]byte(nil), original...)}}
	pluginPaths := plugins.Paths{Global: []string{configPath}}
	manager := plugins.NewManager(pluginTestRunner(func(context.Context, string, ...string) error {
		files.data[configPath] = []byte("{\n  // retained\n  \"plugin\": [\"@sbakolis/open-loop\"]\n}\n")
		return nil
	}), files, pluginPaths)
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true, PluginModules: []string{plugins.OpenLoop}, PluginManager: manager, PluginFiles: files, PluginPaths: pluginPaths}); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(paths, false, PluginLifecycleOptions{Manager: manager, Files: files, Paths: pluginPaths}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(files.data[configPath], original) {
		t.Fatalf("uninstall did not preserve JSONC: %q", files.data[configPath])
	}
}

func TestUninstallPreservesPreExistingPluginRegistration(t *testing.T) {
	paths := testPaths(t.TempDir())
	configPath := "config.jsonc"
	original := []byte(`{"plugin":[["@sbakolis/open-loop",{"mode":"custom"}]]}`)
	files := &pluginTestFiles{data: map[string][]byte{configPath: append([]byte(nil), original...)}}
	pluginPaths := plugins.Paths{Global: []string{configPath}}
	manager := plugins.NewManager(pluginTestRunner(func(context.Context, string, ...string) error {
		t.Fatal("runner invoked for pre-existing plugin")
		return nil
	}), files, pluginPaths)
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true, PluginModules: []string{plugins.OpenLoop}, PluginManager: manager, PluginFiles: files, PluginPaths: pluginPaths}); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(paths, false, PluginLifecycleOptions{Manager: manager, Files: files, Paths: pluginPaths}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(files.data[configPath], original) {
		t.Fatalf("uninstall changed pre-existing plugin: %q", files.data[configPath])
	}
}

func TestPluginFailureRollsBackInstallerAndCandidateConfig(t *testing.T) {
	paths := testPaths(t.TempDir())
	configPath := "config.jsonc"
	original := []byte("{\n  // original\n  \"plugin\": []\n}\n")
	files := &pluginTestFiles{data: map[string][]byte{configPath: append([]byte(nil), original...)}}
	pluginPaths := plugins.Paths{Global: []string{configPath}}
	manager := plugins.NewManager(pluginTestRunner(func(context.Context, string, ...string) error {
		files.data[configPath] = []byte(`{"plugin":["opencode-codex-meter"]}`)
		return errors.New("plugin failed")
	}), files, pluginPaths)
	_, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true, PluginModules: []string{plugins.CodexMeter}, PluginManager: manager, PluginFiles: files, PluginPaths: pluginPaths})
	if err == nil || !strings.Contains(err.Error(), "plugin failed") {
		t.Fatalf("error = %v", err)
	}
	if !bytes.Equal(files.data[configPath], original) {
		t.Fatalf("candidate config was not restored byte-for-byte: %q", files.data[configPath])
	}
	for _, path := range []string{ManifestPath(paths), filepath.Join(paths.ConfigDir, "config.yaml"), filepath.Join(paths.OpenCodeDir, "AGENTS.md")} {
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("installer mutation remains at %s: %v", path, statErr)
		}
	}
}

func TestUninstallDoesNotRemoveCustomizedOwnedRegistration(t *testing.T) {
	paths := testPaths(t.TempDir())
	configPath := "config.jsonc"
	tuiPath := "tui.jsonc"
	files := &pluginTestFiles{data: map[string][]byte{configPath: []byte(`{"plugin":[]}`), tuiPath: []byte(`{"plugin":[]}`)}}
	pluginPaths := plugins.Paths{Global: []string{configPath}, TUI: []string{tuiPath}}
	manager := plugins.NewManager(pluginTestRunner(func(context.Context, string, ...string) error {
		files.data[configPath] = []byte(`{"plugin":["opencode-codex-meter"]}`)
		files.data[tuiPath] = []byte(`{"plugin":["opencode-codex-meter"]}`)
		return nil
	}), files, pluginPaths)
	if _, err := Install(Options{Paths: paths, Version: "dev", NonInteractive: true, PluginModules: []string{plugins.CodexMeter}, PluginManager: manager, PluginFiles: files, PluginPaths: pluginPaths}); err != nil {
		t.Fatal(err)
	}
	customized := []byte("{\n  // user changed North's entry\n  \"plugin\": [[\"opencode-codex-meter\", {\"display\":\"compact\"}]]\n}\n")
	files.data[configPath] = append([]byte(nil), customized...)
	if _, err := Uninstall(paths, false, PluginLifecycleOptions{Manager: manager, Files: files, Paths: pluginPaths}); err == nil || !strings.Contains(err.Error(), "was changed") {
		t.Fatalf("error = %v", err)
	}
	if !bytes.Equal(files.data[configPath], customized) {
		t.Fatalf("uninstall changed customized plugin registration: %q", files.data[configPath])
	}
}

func TestLoadManifestMigratesLegacyUnversionedManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(`{"northVersion":"0.0.1","installedAt":"2026-01-01T00:00:00Z","scope":"global","components":[],"managedFiles":[],"instructions":{"activePath":"/tmp/AGENTS.md","generatedSha256":"abc"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != ManifestSchemaVersion || manifest.NorthVersion != "0.0.1" {
		t.Fatalf("manifest = %+v", manifest)
	}
}
