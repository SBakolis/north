// Package state persists North run state on the local filesystem.
package state

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/platform"
)

var (
	ErrCorrupt = errors.New("state is corrupt")
	ErrLocked  = errors.New("state is locked")
)

// CorruptionError identifies both the bad file and the path to which it was
// quarantined. A quarantine is never replaced with fresh state automatically.
type CorruptionError struct {
	Path           string
	QuarantinePath string
	Cause          error
}

func (e *CorruptionError) Error() string {
	if e.QuarantinePath == "" {
		return fmt.Sprintf("%v: %s: %v", ErrCorrupt, e.Path, e.Cause)
	}
	return fmt.Sprintf("%v: %s quarantined as %s: %v", ErrCorrupt, e.Path, e.QuarantinePath, e.Cause)
}

func (e *CorruptionError) Unwrap() error { return ErrCorrupt }

// Store uses root as the run directory for one project.
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) *Store {
	return &Store{root: filepath.Clean(root), now: time.Now}
}

func NewStore(root string) *Store { return New(root) }

func (s *Store) Root() string { return s.root }

func (s *Store) CreateRun(ctx context.Context, run model.RunState) error {
	if err := validateRun(run); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create run root: %w", err)
	}
	runDir, err := s.runDir(run.ID)
	if err != nil {
		return err
	}
	if err := os.Mkdir(runDir, 0o700); err != nil {
		return fmt.Errorf("create run %q: %w", run.ID, err)
	}
	created := true
	defer func() {
		if created {
			_ = os.RemoveAll(runDir)
		}
	}()

	unlock, err := s.acquireMutationLock(ctx, runDir)
	if err != nil {
		return err
	}
	defer unlock()
	if err := s.writeRunFiles(runDir, run); err != nil {
		return err
	}
	created = false
	return nil
}

func (s *Store) UpdateRun(ctx context.Context, run model.RunState) error {
	if err := validateRun(run); err != nil {
		return err
	}
	return s.withRunMutation(ctx, run.ID, func(runDir string) error {
		current, err := s.loadRun(runDir)
		if err != nil {
			return err
		}
		if current.ProjectID != run.ProjectID {
			return fmt.Errorf("update run %q: project ID cannot change", run.ID)
		}
		return s.writeRunFiles(runDir, run)
	})
}

func (s *Store) MutateRun(ctx context.Context, runID string, mutate func(*model.RunState) error) (model.RunState, error) {
	var updated model.RunState
	err := s.withRunMutation(ctx, runID, func(runDir string) error {
		run, err := s.loadRun(runDir)
		if err != nil {
			return err
		}
		if err := mutate(&run); err != nil {
			return err
		}
		if err := validateRun(run); err != nil {
			return err
		}
		if err := s.writeRunFiles(runDir, run); err != nil {
			return err
		}
		updated = run
		return nil
	})
	return updated, err
}

func (s *Store) UpdateStage(ctx context.Context, runID string, stage model.StageState) error {
	if err := validateID("run", runID); err != nil {
		return err
	}
	if err := validateID("stage", stage.ID); err != nil {
		return err
	}
	return s.withRunMutation(ctx, runID, func(runDir string) error {
		run, err := s.loadRun(runDir)
		if err != nil {
			return err
		}
		index := -1
		for i := range run.Stages {
			if run.Stages[i].ID == stage.ID {
				index = i
				break
			}
		}
		if index < 0 {
			return fmt.Errorf("update stage %q: stage does not belong to run %q", stage.ID, runID)
		}
		run.Stages[index] = stage
		if err := writeJSON(filepath.Join(runDir, "run.json"), run); err != nil {
			return fmt.Errorf("write run aggregate %q: %w", runID, err)
		}
		if err := writeJSON(filepath.Join(runDir, "stages", stage.ID+".json"), stage); err != nil {
			return fmt.Errorf("write stage %q: %w", stage.ID, err)
		}
		return nil
	})
}

func (s *Store) AppendEvent(ctx context.Context, event model.Event) error {
	if err := validateID("run", event.RunID); err != nil {
		return err
	}
	return s.withRunMutation(ctx, event.RunID, func(runDir string) error {
		if _, err := s.loadRun(runDir); err != nil {
			return err
		}
		path := filepath.Join(runDir, "events.ndjson")
		if event.ID != "" {
			exists, err := eventIDExists(path, event.ID)
			if err != nil {
				return err
			}
			if exists {
				return nil
			}
		}
		sequence, err := s.nextEventSequence(path, event.RunID)
		if err != nil {
			return err
		}
		event.Sequence = sequence
		if event.SchemaVersion == 0 {
			event.SchemaVersion = 1
		}
		if event.SchemaVersion != 1 {
			return fmt.Errorf("unsupported event schema version %d", event.SchemaVersion)
		}
		if event.Time.IsZero() {
			event.Time = s.now().UTC()
		}
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("encode event: %w", err)
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			return fmt.Errorf("open event log: %w", err)
		}
		if err := file.Chmod(0o600); err != nil {
			file.Close()
			return fmt.Errorf("secure event log: %w", err)
		}
		data = append(data, '\n')
		if _, err := file.Write(data); err != nil {
			file.Close()
			return fmt.Errorf("append event: %w", err)
		}
		if err := file.Sync(); err != nil {
			file.Close()
			return fmt.Errorf("sync event log: %w", err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close event log: %w", err)
		}
		return nil
	})
}

func eventIDExists(path, id string) (bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		var event model.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return false, err
		}
		if event.ID == id {
			return true, nil
		}
	}
	return false, scanner.Err()
}

func (s *Store) LoadRun(ctx context.Context, runID string) (model.RunState, error) {
	if err := validateID("run", runID); err != nil {
		return model.RunState{}, err
	}
	if err := ctx.Err(); err != nil {
		return model.RunState{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	runDir, _ := s.runDir(runID)
	if _, err := os.Stat(runDir); err != nil {
		return model.RunState{}, fmt.Errorf("open run %q: %w", runID, err)
	}
	unlock, err := s.acquireMutationLock(ctx, runDir)
	if err != nil {
		return model.RunState{}, err
	}
	defer unlock()
	return s.loadRun(runDir)
}

func (s *Store) ListRuns(ctx context.Context, projectID string) ([]model.RunSummary, error) {
	if projectID == "" {
		return nil, errors.New("list runs: project ID is empty")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if errors.Is(err, os.ErrNotExist) {
		return []model.RunSummary{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list run root: %w", err)
	}
	var summaries []model.RunSummary
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if err := validateID("run", entry.Name()); err != nil {
			continue
		}
		runDir := filepath.Join(s.root, entry.Name())
		unlock, err := s.acquireMutationLock(ctx, runDir)
		if err != nil {
			return nil, err
		}
		run, err := s.loadRun(runDir)
		unlock()
		if err != nil {
			return nil, err
		}
		if run.ProjectID == projectID {
			summaries = append(summaries, model.RunSummary{ID: run.ID, Status: run.Status, UpdatedAt: run.UpdatedAt})
		}
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].UpdatedAt.Equal(summaries[j].UpdatedAt) {
			return summaries[i].ID < summaries[j].ID
		}
		return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt)
	})
	return summaries, nil
}

func (s *Store) withRunMutation(ctx context.Context, runID string, operation func(string) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	runDir, err := s.runDir(runID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(runDir); err != nil {
		return fmt.Errorf("open run %q: %w", runID, err)
	}
	unlock, err := s.acquireMutationLock(ctx, runDir)
	if err != nil {
		return err
	}
	defer unlock()
	return operation(runDir)
}

func (s *Store) writeRunFiles(runDir string, run model.RunState) error {
	stagesDir := filepath.Join(runDir, "stages")
	if err := os.MkdirAll(stagesDir, 0o700); err != nil {
		return fmt.Errorf("create stages directory: %w", err)
	}
	if err := writeJSON(filepath.Join(runDir, "run.json"), run); err != nil {
		return fmt.Errorf("write run aggregate: %w", err)
	}
	for _, stage := range run.Stages {
		if err := writeJSON(filepath.Join(stagesDir, stage.ID+".json"), stage); err != nil {
			return fmt.Errorf("write stage %q: %w", stage.ID, err)
		}
	}
	wanted := make(map[string]struct{}, len(run.Stages))
	for _, stage := range run.Stages {
		wanted[stage.ID+".json"] = struct{}{}
	}
	entries, err := os.ReadDir(stagesDir)
	if err != nil {
		return fmt.Errorf("list stage snapshots: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if _, exists := wanted[entry.Name()]; !exists {
			if err := os.Remove(filepath.Join(stagesDir, entry.Name())); err != nil {
				return fmt.Errorf("remove obsolete stage snapshot %q: %w", entry.Name(), err)
			}
		}
	}
	return nil
}

func (s *Store) loadRun(runDir string) (model.RunState, error) {
	path := filepath.Join(runDir, "run.json")
	var run model.RunState
	if err := s.readJSON(path, &run); err != nil {
		return model.RunState{}, err
	}
	if filepath.Base(runDir) != run.ID {
		return model.RunState{}, s.quarantine(path, fmt.Errorf("run ID %q does not match directory", run.ID))
	}
	if err := validateRun(run); err != nil {
		return model.RunState{}, s.quarantine(path, err)
	}
	expected := make(map[string]struct{}, len(run.Stages))
	for _, aggregate := range run.Stages {
		expected[aggregate.ID+".json"] = struct{}{}
		stagePath := filepath.Join(runDir, "stages", aggregate.ID+".json")
		var stage model.StageState
		if err := s.readJSON(stagePath, &stage); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return model.RunState{}, &CorruptionError{Path: stagePath, Cause: errors.New("stage snapshot is missing from run state")}
			}
			return model.RunState{}, err
		}
		if !reflect.DeepEqual(stage, aggregate) {
			return model.RunState{}, s.quarantine(stagePath, errors.New("stage snapshot differs from run aggregate"))
		}
	}
	entries, err := os.ReadDir(filepath.Join(runDir, "stages"))
	if err != nil {
		return model.RunState{}, s.quarantine(path, fmt.Errorf("list stage snapshots: %w", err))
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if _, exists := expected[entry.Name()]; !exists {
			extra := filepath.Join(runDir, "stages", entry.Name())
			return model.RunState{}, s.quarantine(extra, errors.New("stage snapshot is absent from run aggregate"))
		}
	}
	return run, nil
}

func (s *Store) readJSON(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if quarantined, _ := filepath.Glob(path + ".corrupt.*"); len(quarantined) > 0 {
				return &CorruptionError{Path: path, QuarantinePath: quarantined[len(quarantined)-1], Cause: errors.New("original file was quarantined")}
			}
		}
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(destination); err != nil {
		return s.quarantine(path, err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return s.quarantine(path, err)
	}
	return nil
}

func (s *Store) nextEventSequence(path, runID string) (uint64, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		if quarantined, _ := filepath.Glob(path + ".corrupt.*"); len(quarantined) > 0 {
			return 0, &CorruptionError{Path: path, QuarantinePath: quarantined[len(quarantined)-1], Cause: errors.New("event log was quarantined")}
		}
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("open event log: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("inspect event log: %w", err)
	}
	if info.Size() > 0 {
		last := []byte{0}
		if _, err := file.ReadAt(last, info.Size()-1); err != nil {
			return 0, fmt.Errorf("inspect event log terminator: %w", err)
		}
		if last[0] != '\n' {
			return 0, s.quarantineAfterClose(file, path, errors.New("event log has a truncated final record"))
		}
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var previous uint64
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			return 0, s.quarantineAfterClose(file, path, errors.New("blank event record"))
		}
		var event model.Event
		if err := json.Unmarshal(line, &event); err != nil {
			return 0, s.quarantineAfterClose(file, path, err)
		}
		if event.RunID != runID || event.Sequence != previous+1 {
			return 0, s.quarantineAfterClose(file, path, fmt.Errorf("invalid event sequence or run ID at sequence %d", event.Sequence))
		}
		previous = event.Sequence
	}
	if err := scanner.Err(); err != nil {
		return 0, s.quarantineAfterClose(file, path, err)
	}
	if previous == ^uint64(0) {
		return 0, errors.New("event sequence exhausted")
	}
	return previous + 1, nil
}

func (s *Store) quarantineAfterClose(file *os.File, path string, cause error) error {
	_ = file.Close()
	return s.quarantine(path, cause)
}

func (s *Store) quarantine(path string, cause error) error {
	base := fmt.Sprintf("%s.corrupt.%d", path, s.now().UTC().UnixNano())
	for attempt := 0; ; attempt++ {
		quarantinePath := base
		if attempt > 0 {
			quarantinePath = fmt.Sprintf("%s.%d", base, attempt)
		}
		if err := os.Link(path, quarantinePath); errors.Is(err, os.ErrExist) {
			continue
		} else if err != nil {
			return &CorruptionError{Path: path, Cause: fmt.Errorf("%v; quarantine failed: %w", cause, err)}
		}
		if err := os.Remove(path); err != nil {
			_ = os.Remove(quarantinePath)
			return &CorruptionError{Path: path, Cause: fmt.Errorf("%v; quarantine failed: %w", cause, err)}
		}
		return &CorruptionError{Path: path, QuarantinePath: quarantinePath, Cause: cause}
	}
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return platform.WriteFileAtomic(path, data, 0o600)
}

func validateRun(run model.RunState) error {
	if run.SchemaVersion != 1 {
		return fmt.Errorf("unsupported run schema version %d", run.SchemaVersion)
	}
	if err := validateID("run", run.ID); err != nil {
		return err
	}
	if run.ProjectID == "" {
		return errors.New("run project ID is empty")
	}
	seen := make(map[string]struct{}, len(run.Stages))
	for _, stage := range run.Stages {
		if stage.SchemaVersion != 1 {
			return fmt.Errorf("stage %q has unsupported schema version %d", stage.ID, stage.SchemaVersion)
		}
		if err := validateID("stage", stage.ID); err != nil {
			return err
		}
		if _, exists := seen[stage.ID]; exists {
			return fmt.Errorf("duplicate stage ID %q", stage.ID)
		}
		seen[stage.ID] = struct{}{}
	}
	return nil
}

func validateID(kind, id string) error {
	if id == "" || id == "." || id == ".." || filepath.Base(id) != id || strings.ContainsAny(id, `/\\`) {
		return fmt.Errorf("invalid %s ID %q", kind, id)
	}
	return nil
}

func (s *Store) runDir(runID string) (string, error) {
	if err := validateID("run", runID); err != nil {
		return "", err
	}
	return filepath.Join(s.root, runID), nil
}
