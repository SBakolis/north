package install

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/SBakolis/north/internal/platform"
	"github.com/SBakolis/north/internal/plugins"
)

type Status struct {
	Installed  bool     `json:"installed"`
	Version    string   `json:"version,omitempty"`
	Components []string `json:"components,omitempty"`
	Healthy    bool     `json:"healthy"`
	Issues     []string `json:"issues,omitempty"`
}

func Inspect(paths platform.Paths) (Status, error) {
	manifest, err := LoadManifestIfExists(ManifestPath(paths))
	if err != nil {
		return Status{}, err
	}
	if manifest == nil {
		return Status{Healthy: true}, nil
	}
	status := Status{Installed: true, Version: manifest.NorthVersion, Components: manifest.Components, Healthy: true}
	for _, file := range manifest.ManagedFiles {
		if err := verifyManagedFile(file); err != nil {
			status.Issues = append(status.Issues, err.Error())
		}
	}
	for _, plugin := range manifest.Plugins {
		snapshots, snapshotErr := snapshotPluginPaths(plugins.OSFiles{}, plugin.Candidates)
		if snapshotErr != nil {
			status.Issues = append(status.Issues, fmt.Sprintf("plugin %s: %v", plugin.Module, snapshotErr))
			continue
		}
		verification, verifyErr := plugins.Verify(plugin.Module, snapshots)
		if verifyErr != nil {
			status.Issues = append(status.Issues, fmt.Sprintf("plugin %s: %v", plugin.Module, verifyErr))
			continue
		}
		if len(verification.Registrations) == 0 {
			status.Issues = append(status.Issues, fmt.Sprintf("plugin %s registration missing", plugin.Module))
		}
		for _, diagnostic := range verification.Diagnostics {
			status.Issues = append(status.Issues, fmt.Sprintf("plugin %s [%s]: %s", plugin.Module, diagnostic.Code, diagnostic.Message))
		}
		for _, owned := range plugin.Owned {
			if !registrationPresent(verification.Registrations, owned) {
				status.Issues = append(status.Issues, fmt.Sprintf("plugin %s owned registration changed at %s", plugin.Module, owned.Path))
			}
		}
	}
	active, err := os.ReadFile(manifest.Instructions.ActivePath)
	if err != nil {
		status.Issues = append(status.Issues, fmt.Sprintf("instructions: %v", err))
	} else if _, err := RemoveManagedInstructions(active); err != nil {
		status.Issues = append(status.Issues, "instructions: "+err.Error())
	}
	if manifest.Instructions.GeneratedSnapshotPath != "" {
		generated, err := os.ReadFile(manifest.Instructions.GeneratedSnapshotPath)
		if err != nil {
			status.Issues = append(status.Issues, "generated instruction snapshot: "+err.Error())
		} else if FileSHA256(generated) != manifest.Instructions.GeneratedSHA256 {
			status.Issues = append(status.Issues, "generated instruction snapshot hash does not match manifest")
		}
	}
	for _, original := range manifest.Instructions.Originals {
		if original.StableBackupPath != "" {
			if _, err := os.Stat(original.StableBackupPath); err != nil {
				status.Issues = append(status.Issues, fmt.Sprintf("stable instruction backup %s: %v", original.StableBackupPath, err))
			}
		}
		if original.PrivateSnapshotPath == "" {
			status.Issues = append(status.Issues, "private instruction snapshot is missing from manifest for "+original.Path)
			continue
		}
		if original.Kind == "symlink" {
			if _, err := os.Stat(original.PrivateSnapshotPath); err != nil {
				status.Issues = append(status.Issues, fmt.Sprintf("private instruction snapshot %s: %v", original.PrivateSnapshotPath, err))
			}
			continue
		}
		data, err := os.ReadFile(original.PrivateSnapshotPath)
		if err != nil {
			status.Issues = append(status.Issues, fmt.Sprintf("private instruction snapshot %s: %v", original.PrivateSnapshotPath, err))
		} else if FileSHA256(data) != original.SHA256 {
			status.Issues = append(status.Issues, "private instruction snapshot hash does not match manifest for "+original.Path)
		}
	}
	sort.Strings(status.Issues)
	status.Healthy = len(status.Issues) == 0
	return status, nil
}

func Uninstall(paths platform.Paths, dryRun bool, pluginOptions ...PluginLifecycleOptions) (result Result, returnErr error) {
	manifest, err := LoadManifestIfExists(ManifestPath(paths))
	if err != nil {
		return Result{}, err
	}
	if manifest == nil {
		return Result{}, nil
	}
	active, err := os.ReadFile(manifest.Instructions.ActivePath)
	if err != nil {
		return Result{}, fmt.Errorf("read active instructions: %w", err)
	}
	restoreOriginal := FileSHA256(active) == manifest.Instructions.GeneratedSHA256
	var stripped []byte
	if !restoreOriginal {
		if manifest.Instructions.GeneratedSnapshotPath == "" {
			return Result{}, errors.New("cannot safely uninstall modified instructions without the previous generated snapshot")
		}
		base, readErr := os.ReadFile(manifest.Instructions.GeneratedSnapshotPath)
		if readErr != nil || FileSHA256(base) != manifest.Instructions.GeneratedSHA256 {
			return Result{}, errors.Join(errors.New("cannot safely uninstall modified instructions: generated snapshot is unavailable or invalid"), readErr)
		}
		unchanged, compareErr := ManagedBlockUnchanged(base, active)
		if compareErr != nil {
			return Result{}, compareErr
		}
		if !unchanged {
			return Result{}, errors.New("managed instruction block changed since installation; refusing uninstall to preserve user content")
		}
		stripped, err = RemoveManagedInstructions(active)
		if err != nil {
			return Result{}, err
		}
	}
	for _, file := range manifest.ManagedFiles {
		if err := verifyManagedFile(file); err != nil {
			return Result{}, fmt.Errorf("refuse unsafe uninstall: %w", err)
		}
		result.Operations = append(result.Operations, "remove "+file.Path)
	}
	for _, plugin := range manifest.Plugins {
		if len(plugin.Owned) > 0 {
			result.Operations = append(result.Operations, "remove owned plugin registration "+plugin.Module)
		}
	}
	if restoreOriginal {
		if len(manifest.Instructions.Originals) > 0 {
			for _, original := range manifest.Instructions.Originals {
				if original.Displaced {
					result.Operations = append(result.Operations, "restore "+original.Path)
				}
			}
		} else if manifest.Instructions.SourcePath == "" {
			result.Operations = append(result.Operations, "remove "+manifest.Instructions.ActivePath)
		} else {
			result.Operations = append(result.Operations, "restore "+manifest.Instructions.SourcePath)
		}
	} else {
		result.Operations = append(result.Operations, "remove North block from "+manifest.Instructions.ActivePath)
	}
	result.Operations = append(result.Operations, "remove "+ManifestPath(paths))
	if dryRun {
		return result, nil
	}

	transaction := NewTransaction()
	var configured PluginLifecycleOptions
	if len(pluginOptions) > 0 {
		configured = pluginOptions[0]
	}
	manager, pluginFiles := pluginRuntime(paths, configured.Manager, configured.Files, configured.Paths)
	var pluginRollback []plugins.Snapshot
	committed := false
	defer func() {
		if !committed {
			returnErr = errors.Join(returnErr, restorePluginSnapshots(pluginFiles, pluginRollback))
			returnErr = errors.Join(returnErr, transaction.Rollback())
		}
	}()
	mutations := []string{manifest.Instructions.ActivePath, manifest.Instructions.SourcePath, ManifestPath(paths)}
	for _, original := range manifest.Instructions.Originals {
		mutations = append(mutations, original.Path)
	}
	for _, file := range manifest.ManagedFiles {
		mutations = append(mutations, file.Path)
	}
	for _, path := range uniqueStrings(mutations) {
		if _, err := snapshotExisting(transaction, filepath.Join(paths.StateDir, "backups"), path); err != nil {
			return result, err
		}
	}
	for _, plugin := range manifest.Plugins {
		if len(plugin.Owned) == 0 {
			continue
		}
		before, err := snapshotPluginPaths(pluginFiles, plugin.Candidates)
		if err != nil {
			return result, err
		}
		pluginRollback = mergePluginSnapshots(pluginRollback, before)
		if err := manager.RemoveOwned(plugin.Owned); err != nil {
			return result, fmt.Errorf("remove plugin %s: %w", plugin.Module, err)
		}
	}
	if len(manifest.Instructions.Originals) > 0 {
		if restoreOriginal {
			if err := transaction.Remove(manifest.Instructions.ActivePath); err != nil {
				return result, err
			}
			for _, original := range manifest.Instructions.Originals {
				if original.Displaced {
					if err := restoreInstructionOriginal(transaction, original); err != nil {
						return result, err
					}
				}
			}
		} else {
			if len(bytes.TrimSpace(stripped)) == 0 {
				if err := transaction.Remove(manifest.Instructions.ActivePath); err != nil {
					return result, err
				}
			} else if err := transaction.WriteFile(manifest.Instructions.ActivePath, stripped, 0o600); err != nil {
				return result, err
			}
			for _, original := range manifest.Instructions.Originals {
				if original.Displaced && original.Path != manifest.Instructions.ActivePath {
					if err := restoreInstructionOriginal(transaction, original); err != nil {
						return result, err
					}
				}
			}
		}
	} else if restoreOriginal {
		if manifest.Instructions.SourcePath == "" {
			if err := transaction.Remove(manifest.Instructions.ActivePath); err != nil {
				return result, err
			}
		} else {
			if manifest.Instructions.SourcePath != manifest.Instructions.ActivePath {
				if _, err := os.Lstat(manifest.Instructions.SourcePath); err == nil {
					return result, fmt.Errorf("refuse to overwrite restored instruction path %s", manifest.Instructions.SourcePath)
				} else if !errors.Is(err, os.ErrNotExist) {
					return result, err
				}
				if err := transaction.Remove(manifest.Instructions.ActivePath); err != nil {
					return result, err
				}
			}
			if manifest.Instructions.SourceKind == "symlink" {
				if manifest.Instructions.SourceSymlinkTarget == "" {
					return result, errors.New("instruction symlink target is absent from manifest")
				}
				if err := transaction.Symlink(manifest.Instructions.SourceSymlinkTarget, manifest.Instructions.SourcePath); err != nil {
					return result, err
				}
			} else {
				backupPath := manifest.Instructions.PrivateSnapshotPath
				if backupPath == "" {
					backupPath = manifest.Instructions.StableBackupPath
				}
				if backupPath == "" {
					return result, errors.New("manifest has no instruction snapshot for restoration")
				}
				backup, err := os.ReadFile(backupPath)
				if err != nil {
					return result, fmt.Errorf("read instruction snapshot: %w", err)
				}
				if installHash := FileSHA256(backup); manifest.Instructions.SourceSHA256 != "" && installHash != manifest.Instructions.SourceSHA256 {
					return result, errors.New("instruction snapshot hash does not match manifest")
				}
				if err := transaction.WriteFile(manifest.Instructions.SourcePath, backup, 0o600); err != nil {
					return result, err
				}
			}
		}
	} else if len(bytes.TrimSpace(stripped)) == 0 {
		if err := transaction.Remove(manifest.Instructions.ActivePath); err != nil {
			return result, err
		}
	} else if err := transaction.WriteFile(manifest.Instructions.ActivePath, stripped, 0o600); err != nil {
		return result, err
	}
	for index := len(manifest.ManagedFiles) - 1; index >= 0; index-- {
		if err := transaction.Remove(manifest.ManagedFiles[index].Path); err != nil {
			return result, err
		}
	}
	if err := transaction.Remove(ManifestPath(paths)); err != nil {
		return result, err
	}
	committed = true
	return result, nil
}

func restoreInstructionOriginal(transaction *Transaction, original InstructionOriginal) error {
	if _, err := os.Lstat(original.Path); err == nil {
		return fmt.Errorf("refuse to overwrite restored instruction path %s", original.Path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if original.Kind == "symlink" {
		if original.SymlinkTarget == "" {
			return fmt.Errorf("instruction symlink target is absent for %s", original.Path)
		}
		return transaction.Symlink(original.SymlinkTarget, original.Path)
	}
	backupPath := original.PrivateSnapshotPath
	if backupPath == "" {
		backupPath = original.StableBackupPath
	}
	if backupPath == "" {
		return fmt.Errorf("manifest has no instruction snapshot for %s", original.Path)
	}
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("read instruction snapshot for %s: %w", original.Path, err)
	}
	if original.SHA256 != "" && FileSHA256(data) != original.SHA256 {
		return fmt.Errorf("instruction snapshot hash does not match manifest for %s", original.Path)
	}
	return transaction.WriteFile(original.Path, data, 0o600)
}

func registrationPresent(registrations []plugins.Registration, owned plugins.Ownership) bool {
	for _, registration := range registrations {
		if registration.Path == owned.Path && registration.Module == owned.Module && registration.Method == owned.Method && registration.Fingerprint == owned.Fingerprint {
			return true
		}
	}
	return false
}

func verifyManagedFile(file ManagedFile) error {
	info, err := os.Lstat(file.Path)
	if err != nil {
		return fmt.Errorf("managed path %s: %w", file.Path, err)
	}
	switch file.Kind {
	case "symlink":
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("managed path %s is no longer a symlink", file.Path)
		}
		target, err := os.Readlink(file.Path)
		if err != nil {
			return err
		}
		if target != file.Target {
			return fmt.Errorf("managed symlink %s target changed", file.Path)
		}
	case "file":
		if !info.Mode().IsRegular() {
			return fmt.Errorf("managed path %s is no longer a file", file.Path)
		}
		data, err := os.ReadFile(file.Path)
		if err != nil {
			return err
		}
		if FileSHA256(data) != file.SHA256 {
			return fmt.Errorf("managed file %s content changed", file.Path)
		}
	default:
		return fmt.Errorf("managed path %s has unknown kind %q", file.Path, file.Kind)
	}
	return nil
}
