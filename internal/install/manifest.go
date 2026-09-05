package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/SBakolis/north/internal/platform"
	"github.com/SBakolis/north/internal/plugins"
)

const ManifestSchemaVersion = 1

type Manifest struct {
	SchemaVersion int                 `json:"schemaVersion"`
	NorthVersion  string              `json:"northVersion"`
	InstalledAt   time.Time           `json:"installedAt"`
	Scope         string              `json:"scope"`
	Components    []string            `json:"components"`
	ManagedFiles  []ManagedFile       `json:"managedFiles"`
	Instructions  InstructionManifest `json:"instructions"`
	Plugins       []PluginManifest    `json:"plugins,omitempty"`
}

type PluginManifest struct {
	Module      string                    `json:"module"`
	PreExisting bool                      `json:"preExisting"`
	Owned       []plugins.Ownership       `json:"owned,omitempty"`
	Candidates  []PluginCandidateSnapshot `json:"candidates"`
}

type PluginCandidateSnapshot struct {
	Path   string             `json:"path"`
	Role   plugins.ConfigRole `json:"role"`
	Exists bool               `json:"exists"`
	Data   []byte             `json:"data,omitempty"`
}

type ManagedFile struct {
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	Target      string `json:"target,omitempty"`
	SHA256      string `json:"sha256"`
	PreExisting bool   `json:"preExisting"`
}

type InstructionManifest struct {
	ActivePath            string                `json:"activePath"`
	SourcePath            string                `json:"sourcePath,omitempty"`
	StableBackupPath      string                `json:"stableBackupPath,omitempty"`
	StableBackupPaths     []string              `json:"stableBackupPaths,omitempty"`
	PrivateSnapshotPath   string                `json:"privateSnapshotPath,omitempty"`
	SourceSHA256          string                `json:"sourceSha256,omitempty"`
	SourceKind            string                `json:"sourceKind,omitempty"`
	SourceSymlinkTarget   string                `json:"sourceSymlinkTarget,omitempty"`
	GeneratedSHA256       string                `json:"generatedSha256"`
	GeneratedSnapshotPath string                `json:"generatedSnapshotPath,omitempty"`
	Originals             []InstructionOriginal `json:"originals,omitempty"`
}

type InstructionOriginal struct {
	Path                string `json:"path"`
	StableBackupPath    string `json:"stableBackupPath,omitempty"`
	PrivateSnapshotPath string `json:"privateSnapshotPath,omitempty"`
	SHA256              string `json:"sha256"`
	Kind                string `json:"kind"`
	SymlinkTarget       string `json:"symlinkTarget,omitempty"`
	Displaced           bool   `json:"displaced,omitempty"`
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	data, err = migrateManifest(data)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	if manifest.SchemaVersion != ManifestSchemaVersion {
		return Manifest{}, fmt.Errorf("unsupported manifest schema version %d", manifest.SchemaVersion)
	}
	return manifest, nil
}

func migrateManifest(data []byte) ([]byte, error) {
	var header struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	switch header.SchemaVersion {
	case ManifestSchemaVersion:
		return data, nil
	case 0:
		var object map[string]any
		if err := json.Unmarshal(data, &object); err != nil {
			return nil, fmt.Errorf("parse legacy manifest: %w", err)
		}
		if _, ok := object["northVersion"]; !ok {
			return nil, errors.New("legacy manifest is missing northVersion")
		}
		object["schemaVersion"] = ManifestSchemaVersion
		if instructions, ok := object["instructions"].(map[string]any); ok {
			source, _ := instructions["sourcePath"].(string)
			private, _ := instructions["privateSnapshotPath"].(string)
			stable, _ := instructions["stableBackupPath"].(string)
			kind, _ := instructions["sourceKind"].(string)
			if source != "" && private == "" && stable == "" && kind != "symlink" {
				return nil, errors.New("legacy manifest has no instruction snapshot for safe restoration")
			}
		}
		migrated, err := json.Marshal(object)
		if err != nil {
			return nil, fmt.Errorf("migrate manifest: %w", err)
		}
		return migrated, nil
	default:
		return nil, fmt.Errorf("unsupported manifest schema version %d", header.SchemaVersion)
	}
}

func LoadManifestIfExists(path string) (*Manifest, error) {
	manifest, err := LoadManifest(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &manifest, nil
}

func WriteManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	data = append(data, '\n')
	return platform.WriteFileAtomic(path, data, 0o600)
}
