package state

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/orchestration"
)

var _ orchestration.StateStore = (*Store)(nil)
var _ orchestration.AtomicRunStateStore = (*Store)(nil)

func TestStorePersistsRunStagesAndSummaries(t *testing.T) {
	store := New(t.TempDir())
	run := testRun()
	if err := store.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}

	stage := run.Stages[0]
	stage.Status = model.StageRunning
	stage.Attempt = 2
	if err := store.UpdateStage(context.Background(), run.ID, stage); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Stages[0].Status != model.StageRunning || loaded.Stages[0].Attempt != 2 {
		t.Fatalf("loaded stage = %#v", loaded.Stages[0])
	}
	var separate model.StageState
	readJSONFile(t, filepath.Join(store.Root(), run.ID, "stages", stage.ID+".json"), &separate)
	if separate != loaded.Stages[0] {
		t.Fatalf("separate stage = %#v, aggregate = %#v", separate, loaded.Stages[0])
	}
	summaries, err := store.ListRuns(context.Background(), run.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || summaries[0].ID != run.ID {
		t.Fatalf("summaries = %#v", summaries)
	}
	for _, path := range []string{
		filepath.Join(store.Root(), run.ID, "run.json"),
		filepath.Join(store.Root(), run.ID, "stages", stage.ID+".json"),
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode of %s = %o", path, info.Mode().Perm())
		}
	}
}

func TestMutateRunAppliesAgainstCurrentState(t *testing.T) {
	store := New(t.TempDir())
	run := testRun()
	if err := store.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	cancelled := run
	cancelled.Cancellation = &model.Cancellation{RequestedAt: time.Now().UTC(), Reason: "stop"}
	if err := store.UpdateRun(context.Background(), cancelled); err != nil {
		t.Fatal(err)
	}
	if _, err := store.MutateRun(context.Background(), run.ID, func(current *model.RunState) error {
		current.Stages[0].Status = model.StageRunning
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadRun(context.Background(), run.ID)
	if err != nil || loaded.Cancellation == nil || loaded.Cancellation.Reason != "stop" || loaded.Stages[0].Status != model.StageRunning {
		t.Fatalf("loaded=%+v error=%v", loaded, err)
	}
}

func TestLoadQuarantinesDivergentStageSnapshot(t *testing.T) {
	store := New(t.TempDir())
	run := testRun()
	if err := store.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	stale := run.Stages[0]
	stale.Status = model.StageRunning
	data, err := json.Marshal(stale)
	if err != nil {
		t.Fatal(err)
	}
	stagePath := filepath.Join(store.Root(), run.ID, "stages", stale.ID+".json")
	if err := os.WriteFile(stagePath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = store.LoadRun(context.Background(), run.ID)
	var corruption *CorruptionError
	if !errors.As(err, &corruption) || corruption.Path != stagePath || corruption.QuarantinePath == "" {
		t.Fatalf("error = %v", err)
	}
	var preserved model.StageState
	readJSONFile(t, corruption.QuarantinePath, &preserved)
	if preserved.Status != model.StageRunning {
		t.Fatalf("quarantined stage = %+v", preserved)
	}
	if _, err := store.LoadRun(context.Background(), run.ID); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("second load error = %v", err)
	}
}

func TestLoadReportsMissingStageSnapshotWithoutRepair(t *testing.T) {
	store := New(t.TempDir())
	run := testRun()
	if err := store.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	stagePath := filepath.Join(store.Root(), run.ID, "stages", run.Stages[0].ID+".json")
	if err := os.Remove(stagePath); err != nil {
		t.Fatal(err)
	}
	_, err := store.LoadRun(context.Background(), run.ID)
	var corruption *CorruptionError
	if !errors.As(err, &corruption) || corruption.Path != stagePath || corruption.QuarantinePath != "" {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(stagePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("missing snapshot was recreated: %v", statErr)
	}
}

func TestAppendEventAssignsSequenceConcurrently(t *testing.T) {
	root := t.TempDir()
	first := New(root)
	second := New(root)
	run := testRun()
	if err := first.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}

	const count = 40
	var wait sync.WaitGroup
	errorsFound := make(chan error, count)
	for i := 0; i < count; i++ {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			store := first
			if i%2 == 1 {
				store = second
			}
			errorsFound <- store.AppendEvent(context.Background(), model.Event{RunID: run.ID, Type: "test"})
		}(i)
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}

	file, err := os.Open(filepath.Join(root, run.ID, "events.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var sequence uint64
	for scanner.Scan() {
		var event model.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		sequence++
		if event.Sequence != sequence {
			t.Fatalf("sequence = %d, want %d", event.Sequence, sequence)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if sequence != count {
		t.Fatalf("event count = %d, want %d", sequence, count)
	}
}

func TestAppendEventDeduplicatesDurableEventID(t *testing.T) {
	store := New(t.TempDir())
	run := testRun()
	if err := store.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	event := model.Event{ID: "merge:A:abc", RunID: run.ID, Type: "stage.transition"}
	if err := store.AppendEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(store.Root(), run.ID, "events.ndjson"))
	if err != nil || len(strings.Split(strings.TrimSpace(string(data)), "\n")) != 1 {
		t.Fatalf("event log = %q error=%v", data, err)
	}
}

func TestQuarantineDoesNotOverwriteSameTimestampEvidence(t *testing.T) {
	store := New(t.TempDir())
	store.now = func() time.Time { return time.Unix(1, 2) }
	path := filepath.Join(store.Root(), "corrupt.json")
	if err := os.WriteFile(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	var first *CorruptionError
	if !errors.As(store.quarantine(path, errors.New("bad")), &first) {
		t.Fatal("first quarantine did not return corruption error")
	}
	if err := os.WriteFile(path, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	var second *CorruptionError
	if !errors.As(store.quarantine(path, errors.New("bad")), &second) || first.QuarantinePath == second.QuarantinePath {
		t.Fatalf("quarantine paths = %q, %q", first.QuarantinePath, second.QuarantinePath)
	}
	firstData, _ := os.ReadFile(first.QuarantinePath)
	secondData, _ := os.ReadFile(second.QuarantinePath)
	if string(firstData) != "first" || string(secondData) != "second" {
		t.Fatalf("quarantined evidence = %q, %q", firstData, secondData)
	}
}

func TestCorruptStateIsQuarantinedAndNotReset(t *testing.T) {
	store := New(t.TempDir())
	run := testRun()
	if err := store.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	runPath := filepath.Join(store.Root(), run.ID, "run.json")
	if err := os.WriteFile(runPath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := store.LoadRun(context.Background(), run.ID)
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("error = %v, want corruption", err)
	}
	corrupt := new(CorruptionError)
	if !errors.As(err, &corrupt) || corrupt.QuarantinePath == "" {
		t.Fatalf("error = %#v", err)
	}
	if _, err := os.Stat(corrupt.QuarantinePath); err != nil {
		t.Fatal(err)
	}
	_, err = store.LoadRun(context.Background(), run.ID)
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("second load error = %v, want corruption", err)
	}
}

func TestCorruptEventLogIsQuarantinedAndNotReset(t *testing.T) {
	store := New(t.TempDir())
	run := testRun()
	if err := store.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Root(), run.ID, "events.ndjson")
	if err := os.WriteFile(path, []byte(`{"sequence":1`), 0o600); err != nil {
		t.Fatal(err)
	}
	event := model.Event{RunID: run.ID, Type: "test"}
	if err := store.AppendEvent(context.Background(), event); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("error = %v, want corruption", err)
	}
	if err := store.AppendEvent(context.Background(), event); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("second append error = %v, want corruption", err)
	}
}

func testRun() model.RunState {
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	return model.RunState{
		SchemaVersion: 1,
		ID:            "run-1",
		ProjectID:     "project-1",
		Status:        model.RunRunning,
		CreatedAt:     now,
		UpdatedAt:     now,
		Stages: []model.StageState{{
			SchemaVersion: 1,
			ID:            "stage-1",
			Status:        model.StageReady,
			LastActivity:  now,
		}},
	}
}

func readJSONFile(t *testing.T, path string, destination any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, destination); err != nil {
		t.Fatal(err)
	}
}
