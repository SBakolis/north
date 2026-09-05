package install

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	managedBegin = "<!-- NORTH:BEGIN managed; schema=1 -->"
	managedEnd   = "<!-- NORTH:END managed -->"
)

type InstructionSource struct {
	Path        string
	BackupPath  string
	BackupPaths []string
	Content     []byte
	Originals   []InstructionOriginalSource
}

type InstructionOriginalSource struct {
	Path, BackupPath, Kind, SymlinkTarget string
	Content                               []byte
}

func InstructionSourcesDiffer(openCodeDir string) (bool, error) {
	canonical, canonicalExists, err := readInstructionIfExists(filepath.Join(openCodeDir, "AGENTS.md"))
	if err != nil {
		return false, err
	}
	legacy, legacyExists, err := readInstructionIfExists(filepath.Join(openCodeDir, "AGENT.md"))
	if err != nil {
		return false, err
	}
	return canonicalExists && legacyExists && !bytes.Equal(canonical, legacy), nil
}

func ResolveInstructionSource(openCodeDir, explicit string, nonInteractive bool) (InstructionSource, error) {
	canonical := filepath.Join(openCodeDir, "AGENTS.md")
	legacy := filepath.Join(openCodeDir, "AGENT.md")
	canonicalData, canonicalExists, err := readInstructionIfExists(canonical)
	if err != nil {
		return InstructionSource{}, err
	}
	legacyData, legacyExists, err := readInstructionIfExists(legacy)
	if err != nil {
		return InstructionSource{}, err
	}
	if explicit != "" && explicit != "AGENTS.md" && explicit != "AGENT.md" {
		return InstructionSource{}, fmt.Errorf("agent source must be AGENTS.md or AGENT.md")
	}
	if explicit == "AGENTS.md" && !canonicalExists {
		return InstructionSource{}, fmt.Errorf("selected agent source %s does not exist", canonical)
	}
	if explicit == "AGENT.md" && !legacyExists {
		return InstructionSource{}, fmt.Errorf("selected agent source %s does not exist", legacy)
	}
	originals := make([]InstructionOriginalSource, 0, 2)
	if canonicalExists {
		original, err := instructionOriginalSource(canonical, filepath.Join(openCodeDir, "AGENTS-backup.md"), canonicalData)
		if err != nil {
			return InstructionSource{}, err
		}
		originals = append(originals, original)
	}
	if legacyExists {
		original, err := instructionOriginalSource(legacy, filepath.Join(openCodeDir, "AGENT-backup.md"), legacyData)
		if err != nil {
			return InstructionSource{}, err
		}
		originals = append(originals, original)
	}
	resolved := func(path, backup string, content []byte) InstructionSource {
		backups := make([]string, 0, len(originals))
		for _, original := range originals {
			backups = append(backups, original.BackupPath)
		}
		return InstructionSource{Path: path, BackupPath: backup, BackupPaths: backups, Content: content, Originals: originals}
	}
	if explicit == "AGENTS.md" || canonicalExists && !legacyExists {
		backup := filepath.Join(openCodeDir, "AGENTS-backup.md")
		return resolved(canonical, backup, canonicalData), nil
	}
	if canonicalExists && legacyExists && bytes.Equal(canonicalData, legacyData) {
		canonicalBackup := filepath.Join(openCodeDir, "AGENTS-backup.md")
		return resolved(canonical, canonicalBackup, canonicalData), nil
	}
	if explicit == "AGENT.md" || legacyExists && !canonicalExists {
		backup := filepath.Join(openCodeDir, "AGENT-backup.md")
		return resolved(legacy, backup, legacyData), nil
	}
	if canonicalExists && legacyExists {
		message := "AGENTS.md and AGENT.md differ; select one with --agent-source"
		if !nonInteractive {
			message += " (interactive selection is not available in this build)"
		}
		return InstructionSource{}, fmt.Errorf("%s", message)
	}
	return InstructionSource{}, nil
}

func instructionOriginalSource(path, backupPath string, content []byte) (InstructionOriginalSource, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return InstructionOriginalSource{}, err
	}
	original := InstructionOriginalSource{Path: path, BackupPath: backupPath, Kind: "file", Content: content}
	if info.Mode()&os.ModeSymlink != 0 {
		original.Kind = "symlink"
		original.SymlinkTarget, err = os.Readlink(path)
	}
	return original, err
}

func MergeInstructions(base, current, managed []byte) ([]byte, []byte, bool, error) {
	proposed, err := ComposeInstructions(base, nil, managed)
	if err != nil {
		return nil, nil, false, fmt.Errorf("previous generated instructions are malformed: %w", err)
	}
	if bytes.Equal(base, current) {
		return proposed, nil, false, nil
	}
	baseStart, baseEnd, err := managedBlockRange(base)
	if err != nil {
		return nil, nil, false, err
	}
	currentStart, currentEnd, currentErr := managedBlockRange(current)
	if currentErr != nil || !bytes.Equal(base[baseStart:baseEnd], current[currentStart:currentEnd]) {
		if currentErr == nil {
			proposed, _ = ComposeInstructions(current, nil, managed)
		} else if bytes.Count(current, []byte(managedBegin)) == 0 && bytes.Count(current, []byte(managedEnd)) == 0 {
			proposed, _ = ComposeInstructions(nil, current, managed)
		}
		return nil, proposed, true, nil
	}
	merged, err := ComposeInstructions(current, nil, managed)
	return merged, nil, false, err
}

func managedBlockRange(data []byte) (int, int, error) {
	begin := bytes.Index(data, []byte(managedBegin))
	end := bytes.Index(data, []byte(managedEnd))
	if bytes.Count(data, []byte(managedBegin)) != 1 || bytes.Count(data, []byte(managedEnd)) != 1 || begin < 0 || end < begin {
		return 0, 0, fmt.Errorf("malformed North managed markers")
	}
	return begin, end + len(managedEnd), nil
}

func ManagedBlockUnchanged(base, current []byte) (bool, error) {
	baseStart, baseEnd, err := managedBlockRange(base)
	if err != nil {
		return false, err
	}
	currentStart, currentEnd, err := managedBlockRange(current)
	if err != nil {
		return false, err
	}
	return bytes.Equal(base[baseStart:baseEnd], current[currentStart:currentEnd]), nil
}

func ComposeInstructions(current, user, managed []byte) ([]byte, error) {
	begin := bytes.Index(current, []byte(managedBegin))
	end := bytes.Index(current, []byte(managedEnd))
	if bytes.Count(current, []byte(managedBegin)) > 1 || bytes.Count(current, []byte(managedEnd)) > 1 || (begin >= 0) != (end >= 0) || begin >= 0 && end < begin {
		return nil, fmt.Errorf("malformed North managed markers")
	}
	block := []byte(managedBegin + "\n" + strings.TrimSpace(string(managed)) + "\n" + managedEnd)
	if begin >= 0 {
		end += len(managedEnd)
		result := append([]byte(nil), current[:begin]...)
		result = append(result, block...)
		result = append(result, current[end:]...)
		return result, nil
	}
	if len(current) > 0 {
		user = current
	}
	var result []byte
	if len(user) > 0 {
		result = append(result, []byte("# User Instructions\n\n")...)
		result = append(result, user...)
		result = bytes.TrimRight(result, "\n")
		result = append(result, '\n', '\n')
	}
	result = append(result, block...)
	result = append(result, '\n')
	return result, nil
}

func RemoveManagedInstructions(current []byte) ([]byte, error) {
	begin := bytes.Index(current, []byte(managedBegin))
	end := bytes.Index(current, []byte(managedEnd))
	if bytes.Count(current, []byte(managedBegin)) != 1 || bytes.Count(current, []byte(managedEnd)) != 1 {
		return nil, fmt.Errorf("expected exactly one North managed block")
	}
	if begin < 0 && end < 0 {
		return nil, fmt.Errorf("North managed markers not found")
	}
	if begin < 0 || end < begin {
		return nil, fmt.Errorf("malformed North managed markers")
	}
	end += len(managedEnd)
	result := append([]byte(nil), current[:begin]...)
	result = append(result, current[end:]...)
	return result, nil
}

func FileSHA256(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func readIfExists(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	return data, true, nil
}

func readInstructionIfExists(path string) ([]byte, bool, error) {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, fmt.Errorf("inspect %s: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	return data, true, nil
}
