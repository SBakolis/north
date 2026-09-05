// Package approval persists operator approval of immutable plan hashes.
package approval

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SBakolis/north/internal/platform"
)

type Record struct {
	Hash       string    `json:"hash"`
	ApprovedAt time.Time `json:"approvedAt"`
}

type fileFormat struct {
	SchemaVersion int               `json:"schemaVersion"`
	Approvals     map[string]Record `json:"approvals"`
}

type Store struct{ Path string }

func (s Store) IsApproved(hash string) (bool, error) {
	file, err := s.load()
	if err != nil {
		return false, err
	}
	_, approved := file.Approvals[hash]
	return approved, nil
}

func (s Store) Approve(hash string) error {
	if hash == "" {
		return errors.New("approval hash is empty")
	}
	file, err := s.load()
	if err != nil {
		return err
	}
	file.Approvals[hash] = Record{Hash: hash, ApprovedAt: time.Now().UTC()}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return platform.WriteFileAtomic(s.Path, append(data, '\n'), 0o600)
}

func (s Store) load() (fileFormat, error) {
	file := fileFormat{SchemaVersion: 1, Approvals: make(map[string]Record)}
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return file, nil
	}
	if err != nil {
		return file, err
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return file, fmt.Errorf("parse approvals %s: %w", filepath.Clean(s.Path), err)
	}
	if file.SchemaVersion != 1 {
		return file, fmt.Errorf("unsupported approval schema %d", file.SchemaVersion)
	}
	if file.Approvals == nil {
		file.Approvals = make(map[string]Record)
	}
	return file, nil
}
