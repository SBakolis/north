package install

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SBakolis/north/assets"
	"github.com/SBakolis/north/internal/components"
	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/platform"
	"github.com/SBakolis/north/internal/plugins"
)

const manifestFilename = "install-manifest.json"

type Options struct {
	Paths          platform.Paths
	Version        string
	Selected       []string
	AgentSource    string
	NonInteractive bool
	DryRun         bool
	Now            func() time.Time
	PluginModules  []string
	PluginManager  PluginManager
	PluginFiles    plugins.Files
	PluginPaths    plugins.Paths
	Context        context.Context
}

type PluginManager interface {
	Enable(context.Context, string) (plugins.Action, error)
	RemoveOwned([]plugins.Ownership) error
}

type PluginLifecycleOptions struct {
	Manager PluginManager
	Files   plugins.Files
	Paths   plugins.Paths
	Context context.Context
}

type Result struct {
	Operations []string
	Manifest   *Manifest
}

type assetOperation struct {
	source, stored, destination string
	data                        []byte
}

func ManifestPath(paths platform.Paths) string {
	return filepath.Join(paths.StateDir, manifestFilename)
}

func Install(options Options) (result Result, returnErr error) {
	registry := components.BuiltinRegistry()
	selected := options.Selected
	if len(selected) == 0 {
		selected = components.Defaults(registry)
	}
	pluginModules, pluginComponentIDs, err := resolvePluginModules(options.PluginModules)
	if err != nil {
		return Result{}, err
	}
	selected = append(append([]string(nil), selected...), pluginComponentIDs...)
	resolved, err := components.Resolve(registry, selected)
	if err != nil {
		return Result{}, fmt.Errorf("resolve components: %w", err)
	}
	existing, err := LoadManifestIfExists(ManifestPath(options.Paths))
	if err != nil {
		return Result{}, err
	}
	if err := verifyExistingManagedFiles(existing); err != nil {
		return Result{}, err
	}

	activePath := filepath.Join(options.Paths.OpenCodeDir, "AGENTS.md")
	current, activeExists, err := readIfExists(activePath)
	if err != nil {
		return Result{}, err
	}
	if existing != nil && !activeExists {
		return Result{}, fmt.Errorf("active instruction file %s is missing; restore it or uninstall before reconfiguring", activePath)
	}
	instructionRecord := InstructionManifest{ActivePath: activePath}
	var sourceContent []byte
	var sourceOriginals []InstructionOriginalSource
	if existing == nil {
		source, err := ResolveInstructionSource(options.Paths.OpenCodeDir, options.AgentSource, options.NonInteractive)
		if err != nil {
			return Result{}, err
		}
		sourceContent = source.Content
		sourceOriginals = source.Originals
		instructionRecord.SourcePath = source.Path
		instructionRecord.StableBackupPath = source.BackupPath
		instructionRecord.StableBackupPaths = append([]string(nil), source.BackupPaths...)
		instructionRecord.SourceSHA256 = FileSHA256(source.Content)
		if source.Path != "" {
			info, err := os.Lstat(source.Path)
			if err != nil {
				return Result{}, err
			}
			instructionRecord.SourceKind = "file"
			if info.Mode()&os.ModeSymlink != 0 {
				instructionRecord.SourceKind = "symlink"
				instructionRecord.SourceSymlinkTarget, err = os.Readlink(source.Path)
				if err != nil {
					return Result{}, err
				}
			}
		}
		for _, original := range source.Originals {
			instructionRecord.Originals = append(instructionRecord.Originals, InstructionOriginal{
				Path: original.Path, StableBackupPath: original.BackupPath, SHA256: FileSHA256(original.Content),
				Kind: original.Kind, SymlinkTarget: original.SymlinkTarget,
				Displaced: original.Path == activePath || original.Path == source.Path && source.Path != activePath,
			})
		}
	} else {
		instructionRecord = existing.Instructions
	}
	managed, err := renderInstructions(resolved)
	if err != nil {
		return Result{}, err
	}
	var generated []byte
	if existing == nil {
		if bytes.Contains(sourceContent, []byte(managedBegin)) || bytes.Contains(sourceContent, []byte(managedEnd)) {
			proposed, composeErr := ComposeInstructions(sourceContent, nil, managed)
			if composeErr != nil {
				proposed, composeErr = ComposeInstructions(nil, sourceContent, managed)
			}
			if composeErr != nil {
				return Result{}, composeErr
			}
			return writeInstructionConflict(options, proposed, errors.New("pre-existing North markers are not owned by this installation"))
		}
		generated, err = ComposeInstructions(nil, sourceContent, managed)
	} else {
		base := current
		if FileSHA256(current) != existing.Instructions.GeneratedSHA256 {
			if existing.Instructions.GeneratedSnapshotPath == "" {
				return instructionConflict(options, current, sourceContent, managed, errors.New("previous generated instruction snapshot is unavailable"))
			}
			base, err = os.ReadFile(existing.Instructions.GeneratedSnapshotPath)
			if err != nil || FileSHA256(base) != existing.Instructions.GeneratedSHA256 {
				return instructionConflict(options, current, sourceContent, managed, errors.Join(errors.New("previous generated instruction snapshot is unavailable or invalid"), err))
			}
		}
		var proposed []byte
		var conflict bool
		generated, proposed, conflict, err = MergeInstructions(base, current, managed)
		if err == nil && conflict {
			return writeInstructionConflict(options, proposed, errors.New("managed instruction block changed since installation"))
		}
	}
	if err != nil {
		return Result{}, err
	}
	instructionRecord.GeneratedSHA256 = FileSHA256(generated)
	instructionRecord.GeneratedSnapshotPath = generatedInstructionSnapshotPath(options.Paths, instructionRecord.GeneratedSHA256)

	version := sanitizeVersion(options.Version)
	if version == "" {
		version = "dev"
	}
	manifest := Manifest{
		SchemaVersion: ManifestSchemaVersion,
		NorthVersion:  options.Version,
		InstalledAt:   now(options).UTC(),
		Scope:         "global",
		Instructions:  instructionRecord,
	}
	if existing != nil {
		manifest.InstalledAt = existing.InstalledAt
	}
	for _, component := range resolved {
		manifest.Components = append(manifest.Components, component.ID)
	}

	var assetOperations []assetOperation
	for _, component := range resolved {
		for _, destination := range component.ManagedDestinations {
			source := assetSource(destination)
			data, err := assets.FS.ReadFile(source)
			if err != nil {
				return Result{}, fmt.Errorf("read embedded asset %s: %w", source, err)
			}
			stored := filepath.Join(options.Paths.DataDir, "versions", version, "assets", filepath.FromSlash(source))
			destinationPath := filepath.Join(options.Paths.OpenCodeDir, filepath.FromSlash(destination))
			assetOperations = append(assetOperations, assetOperation{source, stored, destinationPath, data})
			manifest.ManagedFiles = append(manifest.ManagedFiles,
				ManagedFile{Path: stored, Kind: "file", SHA256: FileSHA256(data)},
				ManagedFile{Path: destinationPath, Kind: "symlink", Target: stored, SHA256: FileSHA256(data)},
			)
		}
	}
	configPath := filepath.Join(options.Paths.ConfigDir, "config.yaml")
	config := renderConfig(resolved)
	if err := preflightNorthPaths(existing, configPath, assetOperations); err != nil {
		return Result{}, err
	}
	manifest.ManagedFiles = append(manifest.ManagedFiles, ManagedFile{Path: configPath, Kind: "file", SHA256: FileSHA256(config)})
	removedFiles, err := deselectedManagedFiles(existing, manifest.ManagedFiles)
	if err != nil {
		return Result{}, err
	}

	result.Operations = append(result.Operations, "write "+configPath)
	if existing == nil && instructionRecord.SourcePath != "" {
		for _, backupPath := range instructionBackupPaths(instructionRecord) {
			if _, backupExists, err := readIfExists(backupPath); err != nil {
				return Result{}, err
			} else if !backupExists {
				result.Operations = append(result.Operations, "create stable backup "+backupPath)
			}
		}
		if instructionRecord.SourcePath != activePath {
			result.Operations = append(result.Operations, "move legacy instructions to stable backup")
		}
	}
	result.Operations = append(result.Operations, "write "+activePath)
	result.Operations = append(result.Operations, "write generated instruction snapshot "+instructionRecord.GeneratedSnapshotPath)
	for _, operation := range assetOperations {
		result.Operations = append(result.Operations, "write "+operation.stored, "link "+operation.destination+" -> "+operation.stored)
	}
	for _, file := range removedFiles {
		result.Operations = append(result.Operations, "remove "+file.Path)
	}
	for _, module := range pluginModules {
		result.Operations = append(result.Operations, "enable plugin "+module)
	}
	if existing != nil {
		for _, plugin := range existing.Plugins {
			if !containsString(pluginModules, plugin.Module) && len(plugin.Owned) > 0 {
				result.Operations = append(result.Operations, "remove owned plugin registration "+plugin.Module)
			}
		}
	}
	result.Operations = append(result.Operations, "write "+ManifestPath(options.Paths))
	result.Manifest = &manifest
	if options.DryRun {
		return result, nil
	}

	if err := preflightDestinations(assetOperations, existing); err != nil {
		return Result{}, err
	}
	transaction := NewTransaction()
	manager, pluginFiles := pluginRuntime(options.Paths, options.PluginManager, options.PluginFiles, options.PluginPaths)
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	var pluginRollback []plugins.Snapshot
	committed := false
	defer func() {
		if !committed {
			returnErr = errors.Join(returnErr, restorePluginSnapshots(pluginFiles, pluginRollback))
			returnErr = errors.Join(returnErr, transaction.Rollback())
		}
	}()
	snapshotDir := filepath.Join(options.Paths.StateDir, "backups")
	mutations := []string{configPath, activePath, ManifestPath(options.Paths)}
	for _, operation := range assetOperations {
		mutations = append(mutations, operation.stored, operation.destination)
	}
	for _, file := range removedFiles {
		mutations = append(mutations, file.Path)
	}
	if instructionRecord.SourcePath != "" {
		mutations = append(mutations, instructionRecord.SourcePath)
		mutations = append(mutations, instructionBackupPaths(instructionRecord)...)
	}
	for _, original := range instructionRecord.Originals {
		mutations = append(mutations, original.Path, original.StableBackupPath)
	}
	if existing == nil {
		for index := range instructionRecord.Originals {
			snapshot, err := snapshotExisting(transaction, snapshotDir, instructionRecord.Originals[index].Path)
			if err != nil {
				return result, err
			}
			instructionRecord.Originals[index].PrivateSnapshotPath = snapshot
		}
	}
	for _, path := range uniqueStrings(mutations) {
		snapshot, err := snapshotExisting(transaction, snapshotDir, path)
		if err != nil {
			return result, err
		}
		if existing == nil && path == instructionRecord.SourcePath && snapshot != "" {
			instructionRecord.PrivateSnapshotPath = snapshot
		}
	}
	if err := transaction.WriteFile(configPath, config, 0o600); err != nil {
		return result, err
	}
	if existing == nil && instructionRecord.SourcePath != "" {
		for _, original := range sourceOriginals {
			_, backupExists, err := readIfExists(original.BackupPath)
			if err != nil {
				return result, err
			}
			if !backupExists {
				if err := transaction.WriteFile(original.BackupPath, original.Content, 0o600); err != nil {
					return result, err
				}
			}
		}
		if instructionRecord.SourcePath != activePath {
			if err := transaction.Remove(instructionRecord.SourcePath); err != nil {
				return result, err
			}
		}
	}
	if err := transaction.WriteFile(activePath, generated, 0o600); err != nil {
		return result, err
	}
	storedGenerated, generatedExists, err := readIfExists(instructionRecord.GeneratedSnapshotPath)
	if err != nil {
		return result, err
	}
	if generatedExists && !bytes.Equal(storedGenerated, generated) {
		return result, fmt.Errorf("generated instruction snapshot collision at %s", instructionRecord.GeneratedSnapshotPath)
	}
	if !generatedExists {
		if err := transaction.WriteFile(instructionRecord.GeneratedSnapshotPath, generated, 0o600); err != nil {
			return result, err
		}
	}
	for _, operation := range assetOperations {
		if err := transaction.WriteFile(operation.stored, operation.data, 0o600); err != nil {
			return result, err
		}
		if err := transaction.Symlink(operation.stored, operation.destination); err != nil {
			return result, err
		}
	}
	for _, file := range removedFiles {
		if err := transaction.Remove(file.Path); err != nil {
			return result, fmt.Errorf("remove deselected managed path %s: %w", file.Path, err)
		}
	}
	if existing != nil {
		for _, previous := range existing.Plugins {
			if containsString(pluginModules, previous.Module) || len(previous.Owned) == 0 {
				continue
			}
			before, err := snapshotPluginPaths(pluginFiles, previous.Candidates)
			if err != nil {
				return result, err
			}
			pluginRollback = mergePluginSnapshots(pluginRollback, before)
			if err := manager.RemoveOwned(previous.Owned); err != nil {
				return result, fmt.Errorf("remove deselected plugin %s: %w", previous.Module, err)
			}
		}
	}
	for _, module := range pluginModules {
		action, err := manager.Enable(ctx, module)
		pluginRollback = mergePluginSnapshots(pluginRollback, action.Before)
		if err != nil {
			return result, err
		}
		for _, diagnostic := range action.Verification.Diagnostics {
			if diagnostic.Severity == plugins.Error {
				return result, fmt.Errorf("verify plugin %s [%s]: %s", module, diagnostic.Code, diagnostic.Message)
			}
		}
		record := pluginManifest(action)
		if previous := findPlugin(existing, module); previous != nil && !previous.PreExisting && action.PreExisting {
			record.PreExisting = false
			record.Owned = append([]plugins.Ownership(nil), previous.Owned...)
			record.Candidates = append([]PluginCandidateSnapshot(nil), previous.Candidates...)
		}
		manifest.Plugins = append(manifest.Plugins, record)
	}
	manifest.Instructions = instructionRecord
	if manifest.Instructions.PrivateSnapshotPath == "" {
		manifest.Instructions.PrivateSnapshotPath = instructionRecord.PrivateSnapshotPath
	}
	if err := writeManifestTransaction(transaction, ManifestPath(options.Paths), manifest); err != nil {
		return result, err
	}
	committed = true
	result.Manifest = &manifest
	return result, nil
}

func deselectedManagedFiles(existing *Manifest, current []ManagedFile) ([]ManagedFile, error) {
	if existing == nil {
		return nil, nil
	}
	wanted := make(map[string]bool, len(current))
	for _, file := range current {
		wanted[file.Path] = true
	}
	var removed []ManagedFile
	for _, file := range existing.ManagedFiles {
		if wanted[file.Path] {
			continue
		}
		if err := verifyManagedFile(file); err != nil {
			return nil, fmt.Errorf("refuse to remove deselected managed path: %w", err)
		}
		removed = append(removed, file)
	}
	return removed, nil
}

func verifyExistingManagedFiles(existing *Manifest) error {
	if existing == nil {
		return nil
	}
	for _, file := range existing.ManagedFiles {
		if err := verifyManagedFile(file); err != nil {
			return fmt.Errorf("refuse to reconfigure modified managed path: %w", err)
		}
	}
	return nil
}

func instructionBackupPaths(record InstructionManifest) []string {
	paths := append([]string(nil), record.StableBackupPaths...)
	if record.StableBackupPath != "" && !containsString(paths, record.StableBackupPath) {
		paths = append(paths, record.StableBackupPath)
	}
	return paths
}

func generatedInstructionSnapshotPath(paths platform.Paths, hash string) string {
	return filepath.Join(paths.StateDir, "backups", "instructions-generated-"+hash+".md")
}

func instructionConflict(options Options, current, user, managed []byte, cause error) (Result, error) {
	proposed, err := ComposeInstructions(current, user, managed)
	if err != nil {
		proposed, err = ComposeInstructions(nil, current, managed)
	}
	if err != nil {
		return Result{}, errors.Join(cause, err)
	}
	return writeInstructionConflict(options, proposed, cause)
}

func writeInstructionConflict(options Options, proposed []byte, cause error) (Result, error) {
	path := filepath.Join(options.Paths.OpenCodeDir, "AGENTS.md.north-proposed")
	result := Result{Operations: []string{"write proposed instructions " + path}}
	if !options.DryRun {
		current, exists, err := readIfExists(path)
		if err != nil {
			return Result{}, err
		}
		if exists && !bytes.Equal(current, proposed) {
			return Result{}, fmt.Errorf("instruction conflict and existing proposed file differs at %s: %w", path, cause)
		}
		if !exists {
			if err := platform.WriteFileAtomic(path, proposed, 0o600); err != nil {
				return Result{}, err
			}
		}
	}
	return result, fmt.Errorf("instruction conflict; active file left unchanged; review proposed file %s: %w", path, cause)
}

func preflightDestinations(operations []assetOperation, existing *Manifest) error {
	owned := make(map[string]bool)
	if existing != nil {
		for _, file := range existing.ManagedFiles {
			owned[file.Path] = true
		}
	}
	for _, operation := range operations {
		info, err := os.Lstat(operation.destination)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !owned[operation.destination] {
			kind := "file"
			if info.Mode()&os.ModeSymlink != 0 {
				kind = "symlink"
			}
			return fmt.Errorf("refuse to replace user-owned %s %s", kind, operation.destination)
		}
	}
	return nil
}

func preflightNorthPaths(existing *Manifest, configPath string, operations []assetOperation) error {
	owned := make(map[string]bool)
	if existing != nil {
		for _, file := range existing.ManagedFiles {
			owned[file.Path] = true
		}
	}
	paths := []string{configPath}
	for _, operation := range operations {
		paths = append(paths, operation.stored)
	}
	for _, path := range uniqueStrings(paths) {
		if _, err := os.Lstat(path); err == nil {
			if !owned[path] {
				return fmt.Errorf("refuse to overwrite unowned North path %s", path)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func renderInstructions(resolved []model.Component) ([]byte, error) {
	var output bytes.Buffer
	output.WriteString("# North\n\n## Installed Capabilities\n\n")
	for _, component := range resolved {
		output.WriteString("- " + component.Name + ": enabled\n")
	}
	for _, component := range resolved {
		for _, fragment := range component.InstructionFragments {
			data, err := assets.FS.ReadFile("instructions/" + fragment)
			if err != nil {
				return nil, fmt.Errorf("read instruction fragment %s: %w", fragment, err)
			}
			output.WriteString("\n" + strings.TrimSpace(string(data)) + "\n")
		}
	}
	return output.Bytes(), nil
}

func renderConfig(resolved []model.Component) []byte {
	parallelization := false
	knowledge := "none"
	codexMeter := false
	openLoop := false
	for _, component := range resolved {
		parallelization = parallelization || component.ID == "parallelization"
		if component.ID == "knowledge.openspec" {
			knowledge = "openspec"
		}
		codexMeter = codexMeter || component.ID == "plugin.opencode-codex-meter"
		openLoop = openLoop || component.ID == "plugin.open-loop"
	}
	return []byte(fmt.Sprintf(`apiVersion: north/v1alpha1
kind: NorthConfig
installation:
  scope: global
parallelization:
  enabled: %t
  runtime: opencode-cli
  isolation: git-worktree
  integration: progressive
  maxParallel: 4
  failFast: false
  autoIntegrateTarget: false
knowledge:
  provider: %s
plugins:
  opencode-codex-meter:
    enabled: %t
  "@sbakolis/open-loop":
    enabled: %t
`, parallelization, knowledge, codexMeter, openLoop))
}

func resolvePluginModules(selected []string) ([]string, []string, error) {
	moduleToComponent := map[string]string{
		plugins.CodexMeter: "plugin.opencode-codex-meter",
		plugins.OpenLoop:   "plugin.open-loop",
	}
	modules := uniqueStrings(selected)
	components := make([]string, 0, len(modules))
	for _, module := range modules {
		component, ok := moduleToComponent[module]
		if !ok {
			return nil, nil, fmt.Errorf("unsupported plugin %q", module)
		}
		components = append(components, component)
	}
	return modules, components, nil
}

func defaultPluginPaths(paths platform.Paths) plugins.Paths {
	return plugins.Paths{
		Global: []string{filepath.Join(paths.OpenCodeDir, "opencode.jsonc"), filepath.Join(paths.OpenCodeDir, "opencode.json")},
		TUI:    []string{filepath.Join(paths.OpenCodeDir, "tui.jsonc"), filepath.Join(paths.OpenCodeDir, "tui.json")},
	}
}

func pluginRuntime(paths platform.Paths, manager PluginManager, files plugins.Files, candidates plugins.Paths) (PluginManager, plugins.Files) {
	if files == nil {
		files = plugins.OSFiles{}
	}
	if len(candidates.Global)+len(candidates.Server)+len(candidates.TUI) == 0 {
		candidates = defaultPluginPaths(paths)
	}
	if manager == nil {
		manager = plugins.NewManager(nil, files, candidates)
	}
	return manager, files
}

func pluginManifest(action plugins.Action) PluginManifest {
	record := PluginManifest{Module: action.Module, PreExisting: action.PreExisting, Owned: append([]plugins.Ownership(nil), action.Owned...)}
	for _, snapshot := range action.Before {
		record.Candidates = append(record.Candidates, PluginCandidateSnapshot{Path: snapshot.Path, Role: snapshot.Role, Exists: snapshot.Exists, Data: append([]byte(nil), snapshot.Data...)})
	}
	return record
}

func findPlugin(manifest *Manifest, module string) *PluginManifest {
	if manifest == nil {
		return nil
	}
	for index := range manifest.Plugins {
		if manifest.Plugins[index].Module == module {
			return &manifest.Plugins[index]
		}
	}
	return nil
}

func snapshotPluginPaths(files plugins.Files, candidates []PluginCandidateSnapshot) ([]plugins.Snapshot, error) {
	var snapshots []plugins.Snapshot
	for _, candidate := range candidates {
		data, err := files.ReadFile(candidate.Path)
		if errors.Is(err, os.ErrNotExist) {
			snapshots = append(snapshots, plugins.Snapshot{Path: candidate.Path, Role: candidate.Role})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("snapshot plugin config %s: %w", candidate.Path, err)
		}
		snapshots = append(snapshots, plugins.Snapshot{Path: candidate.Path, Role: candidate.Role, Exists: true, Data: append([]byte(nil), data...)})
	}
	return snapshots, nil
}

func mergePluginSnapshots(current, added []plugins.Snapshot) []plugins.Snapshot {
	seen := make(map[string]bool, len(current))
	for _, snapshot := range current {
		seen[snapshot.Path] = true
	}
	for _, snapshot := range added {
		if !seen[snapshot.Path] {
			current = append(current, snapshot)
			seen[snapshot.Path] = true
		}
	}
	return current
}

func restorePluginSnapshots(files plugins.Files, snapshots []plugins.Snapshot) error {
	var restoreErrors []error
	for index := len(snapshots) - 1; index >= 0; index-- {
		snapshot := snapshots[index]
		if snapshot.Exists {
			if err := files.WriteFile(snapshot.Path, snapshot.Data, 0o600); err != nil {
				restoreErrors = append(restoreErrors, fmt.Errorf("restore plugin config %s: %w", snapshot.Path, err))
			}
		} else {
			switch files.(type) {
			case plugins.OSFiles, *plugins.OSFiles:
				if err := os.Remove(snapshot.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
					restoreErrors = append(restoreErrors, fmt.Errorf("remove plugin config %s: %w", snapshot.Path, err))
				}
			}
		}
	}
	return errors.Join(restoreErrors...)
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func assetSource(destination string) string {
	if strings.HasPrefix(destination, "plugins/") {
		return "hooks/" + filepath.Base(destination)
	}
	return filepath.ToSlash(destination)
}

func snapshotExisting(transaction *Transaction, backupDir, path string) (string, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var data []byte
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		data = []byte("symlink:" + target)
	} else if info.Mode().IsRegular() {
		data, err = os.ReadFile(path)
		if err != nil {
			return "", err
		}
	} else {
		return "", fmt.Errorf("refuse to snapshot non-file path %s", path)
	}
	hash := FileSHA256(append([]byte(path+"\x00"), data...))
	snapshot := filepath.Join(backupDir, hash+"-"+filepath.Base(path))
	if _, err := os.Stat(snapshot); err == nil {
		return snapshot, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := transaction.WriteFile(snapshot, data, 0o600); err != nil {
		return "", err
	}
	return snapshot, nil
}

func writeManifestTransaction(transaction *Transaction, path string, manifest Manifest) error {
	data, err := marshalManifest(manifest)
	if err != nil {
		return err
	}
	return transaction.WriteFile(path, data, 0o600)
}

func marshalManifest(manifest Manifest) ([]byte, error) {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func now(options Options) time.Time {
	if options.Now != nil {
		return options.Now()
	}
	return time.Now()
}

func sanitizeVersion(version string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._-", r) {
			return r
		}
		return '-'
	}, version)
}

func uniqueStrings(values []string) []string {
	set := make(map[string]bool)
	var result []string
	for _, value := range values {
		if value != "" && !set[value] {
			set[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
