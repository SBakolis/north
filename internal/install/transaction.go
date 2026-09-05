package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SBakolis/north/internal/platform"
)

type pathState struct {
	path       string
	existed    bool
	mode       os.FileMode
	data       []byte
	symlink    string
	wasSymlink bool
}

// Transaction records original file and symlink states and can restore them in
// reverse mutation order. Directories created as parents are left in place.
type Transaction struct {
	states []pathState
	seen   map[string]bool
}

func NewTransaction() *Transaction { return &Transaction{seen: make(map[string]bool)} }

func (transaction *Transaction) capture(path string) error {
	if transaction.seen[path] {
		return nil
	}
	state := pathState{path: path}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		transaction.states = append(transaction.states, state)
		transaction.seen[path] = true
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	state.existed = true
	state.mode = info.Mode()
	if info.Mode()&os.ModeSymlink != 0 {
		state.wasSymlink = true
		state.symlink, err = os.Readlink(path)
	} else if info.Mode().IsRegular() {
		state.data, err = os.ReadFile(path)
	} else {
		return fmt.Errorf("refuse to mutate non-file path %s", path)
	}
	if err != nil {
		return fmt.Errorf("capture %s: %w", path, err)
	}
	transaction.states = append(transaction.states, state)
	transaction.seen[path] = true
	return nil
}

func (transaction *Transaction) WriteFile(path string, data []byte, mode os.FileMode) error {
	if err := transaction.capture(path); err != nil {
		return err
	}
	return platform.WriteFileAtomic(path, data, mode)
}

func (transaction *Transaction) Symlink(target, path string) error {
	if err := transaction.capture(path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Symlink(target, path)
}

func (transaction *Transaction) Remove(path string) error {
	if err := transaction.capture(path); err != nil {
		return err
	}
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (transaction *Transaction) Rollback() error {
	var rollbackErrors []error
	for index := len(transaction.states) - 1; index >= 0; index-- {
		state := transaction.states[index]
		if err := os.Remove(state.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErrors = append(rollbackErrors, err)
			continue
		}
		if !state.existed {
			continue
		}
		if state.wasSymlink {
			if err := os.Symlink(state.symlink, state.path); err != nil {
				rollbackErrors = append(rollbackErrors, err)
			}
			continue
		}
		if err := platform.WriteFileAtomic(state.path, state.data, state.mode.Perm()); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	return errors.Join(rollbackErrors...)
}
