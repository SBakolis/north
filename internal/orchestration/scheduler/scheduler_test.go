package scheduler_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/orchestration"
	"github.com/SBakolis/north/internal/orchestration/scheduler"
)

type memoryStore struct {
	mu         sync.Mutex
	run        model.RunState
	events     []model.Event
	eventError func(model.Event) error
}

func (s *memoryStore) CreateRun(_ context.Context, run model.RunState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.run = cloneMemoryRun(run)
	return nil
}
func (s *memoryStore) UpdateRun(_ context.Context, run model.RunState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.run = cloneMemoryRun(run)
	return nil
}
func (s *memoryStore) UpdateStage(_ context.Context, _ string, stage model.StageState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.run.Stages {
		if s.run.Stages[i].ID == stage.ID {
			s.run.Stages[i] = stage
			return nil
		}
	}
	return errors.New("stage not found")
}
func (s *memoryStore) AppendEvent(_ context.Context, event model.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.eventError != nil {
		if err := s.eventError(event); err != nil {
			return err
		}
	}
	if event.ID != "" {
		for _, existing := range s.events {
			if existing.ID == event.ID {
				return nil
			}
		}
	}
	s.events = append(s.events, event)
	return nil
}
func (s *memoryStore) LoadRun(_ context.Context, _ string) (model.RunState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneMemoryRun(s.run), nil
}
func (s *memoryStore) ListRuns(context.Context, string) ([]model.RunSummary, error) { return nil, nil }

func cloneMemoryRun(run model.RunState) model.RunState {
	run.Stages = append([]model.StageState(nil), run.Stages...)
	run.PendingEvents = append([]model.Event(nil), run.PendingEvents...)
	return run
}

type fakeIsolation struct {
	mu         sync.Mutex
	bases      map[string]string
	cleanupErr error
}

func (f *fakeIsolation) Prepare(ctx context.Context, req orchestration.IsolationRequest) (orchestration.Workspace, error) {
	if err := ctx.Err(); err != nil {
		return orchestration.Workspace{}, err
	}
	f.mu.Lock()
	f.bases[req.StageID] = req.BaseCommit
	f.mu.Unlock()
	return orchestration.Workspace{Path: req.StageID, Branch: "stage-" + req.StageID}, nil
}
func (f *fakeIsolation) Cleanup(context.Context, orchestration.Workspace) error { return f.cleanupErr }

type fakeRuntime struct {
	mu              sync.Mutex
	active, maximum int
	attempts        map[string]int
	started         chan string
	release         <-chan struct{}
	failures        map[string]int
	merged          func(string) bool
}

func (*fakeRuntime) Validate(context.Context) error         { return nil }
func (f *fakeRuntime) Cancel(context.Context, string) error { return nil }
func (f *fakeRuntime) Execute(ctx context.Context, req orchestration.AgentRequest, _ orchestration.EventSink) (orchestration.AgentResult, error) {
	if req.Started != nil {
		if err := req.Started(orchestration.AgentExecution{ExecutionID: "exec-" + req.StageID, PID: 1234}); err != nil {
			return orchestration.AgentResult{}, err
		}
	}
	f.mu.Lock()
	f.active++
	if f.active > f.maximum {
		f.maximum = f.active
	}
	f.attempts[req.StageID]++
	attempt := f.attempts[req.StageID]
	f.mu.Unlock()
	defer func() { f.mu.Lock(); f.active--; f.mu.Unlock() }()
	if f.started != nil {
		f.started <- req.StageID
	}
	if f.release != nil && (req.StageID == "A" || req.StageID == "B") {
		select {
		case <-f.release:
		case <-ctx.Done():
			return orchestration.AgentResult{}, ctx.Err()
		}
	}
	if req.StageID == "C" && (!f.merged("A") || !f.merged("B")) {
		return orchestration.AgentResult{}, errors.New("C ran before dependencies merged")
	}
	if attempt <= f.failures[req.StageID] {
		return orchestration.AgentResult{}, errors.New("transient")
	}
	return orchestration.AgentResult{ExecutionID: "exec-" + req.StageID, ExitCode: 0}, nil
}

type fakeRepo struct {
	mu        sync.Mutex
	committed map[string][]string
}

func (*fakeRepo) ChangedPaths(_ context.Context, workspace, _ string) ([]string, error) {
	return []string{workspace + ".txt"}, nil
}
func (*fakeRepo) VerifyWriteScope(context.Context, string, []string, []string) error { return nil }
func (f *fakeRepo) CommitPaths(_ context.Context, req orchestration.ExactPathCommitRequest) (string, error) {
	f.mu.Lock()
	f.committed[req.StageID] = append([]string(nil), req.Paths...)
	f.mu.Unlock()
	return "commit-" + req.StageID, nil
}

type fakeVerifier struct{}

func (fakeVerifier) Verify(context.Context, orchestration.VerificationRequest, orchestration.EventSink) orchestration.VerificationResult {
	return orchestration.VerificationResult{Passed: true, Evidence: []string{"accepted"}}
}

type finalFlakyVerifier struct{ calls int }

func (v *finalFlakyVerifier) Verify(context.Context, orchestration.VerificationRequest, orchestration.EventSink) orchestration.VerificationResult {
	v.calls++
	if v.calls == 3 {
		return orchestration.VerificationResult{Failure: &model.StageFailure{Class: "verification", Message: "transient final failure", Retryable: true}}
	}
	return orchestration.VerificationResult{Passed: true}
}

type integrationFailVerifier struct {
	mu       sync.Mutex
	requests []orchestration.VerificationRequest
}

func (v *integrationFailVerifier) Verify(_ context.Context, request orchestration.VerificationRequest, _ orchestration.EventSink) orchestration.VerificationResult {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.requests = append(v.requests, request)
	if request.StageID == "A" && request.Workspace == "integration" {
		return orchestration.VerificationResult{Failure: &model.StageFailure{Message: "integration regression", Retryable: true}}
	}
	return orchestration.VerificationResult{Passed: true}
}

type fakeConflictResolver struct {
	result orchestration.ConflictResolutionResult
	req    orchestration.ConflictResolutionRequest
}

type denyingPolicy struct {
	ordered bool
	retried bool
}

func (p *denyingPolicy) OrderReadyStages(_ context.Context, stages []model.StageState) []model.StageState {
	p.ordered = true
	return stages
}

func (p *denyingPolicy) RetryDecision(_ context.Context, _ model.StageFailure) orchestration.RetryDecision {
	p.retried = true
	return orchestration.RetryDecision{Retry: false}
}

func (f *fakeConflictResolver) ResolveConflict(_ context.Context, req orchestration.ConflictResolutionRequest, _ orchestration.EventSink) orchestration.ConflictResolutionResult {
	f.req = req
	return f.result
}

type fakeIntegration struct {
	mu              sync.Mutex
	active, maximum int
	mergedStages    map[string]bool
	order           []string
	head            string
	conflicts       map[string][]string
}

func (f *fakeIntegration) IntegrateStage(_ context.Context, req orchestration.StageIntegrationRequest) orchestration.IntegrationResult {
	f.mu.Lock()
	f.active++
	if f.active > f.maximum {
		f.maximum = f.active
	}
	f.mu.Unlock()
	time.Sleep(time.Millisecond)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active--
	if paths := f.conflicts[req.StageID]; len(paths) > 0 {
		return orchestration.IntegrationResult{ConflictingPaths: append([]string(nil), paths...), Err: errors.New("conflict")}
	}
	f.mergedStages[req.StageID] = true
	f.order = append(f.order, req.StageID)
	f.head = "head-" + req.StageID
	return orchestration.IntegrationResult{CommitSHA: f.head}
}
func (*fakeIntegration) IntegrateRun(context.Context, orchestration.RunIntegrationRequest) orchestration.IntegrationResult {
	return orchestration.IntegrationResult{}
}
func (f *fakeIntegration) merged(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.mergedStages[id]
}

func plan(max int, failFast bool, stages ...model.Stage) model.ExecutionPlan {
	return model.ExecutionPlan{APIVersion: model.APIVersionV1Alpha1, Kind: model.ExecutionPlanKind, Metadata: model.PlanMetadata{Name: "test"}, Spec: model.PlanSpec{Goal: "test", BaseRef: "HEAD", Policy: model.PlanPolicy{MaxParallel: max, FailFast: failFast, MaxAttemptsPerStage: 3}, Stages: stages}}
}

func newScheduler(t *testing.T, runtime *fakeRuntime, store *memoryStore, isolation *fakeIsolation, integration *fakeIntegration, sleep func(context.Context, time.Duration) error, conflicts ...orchestration.ConflictResolutionProvider) *scheduler.Scheduler {
	t.Helper()
	repo := &fakeRepo{committed: map[string][]string{}}
	var conflictProvider orchestration.ConflictResolutionProvider
	if len(conflicts) > 0 {
		conflictProvider = conflicts[0]
	}
	s, err := scheduler.New(scheduler.Config{Store: store, Runtime: runtime, Isolation: isolation, Inspector: repo, Scope: repo, Verifier: fakeVerifier{}, Committer: repo, Integration: integration, Conflicts: conflictProvider, Retry: scheduler.RetryConfig{BaseDelay: 2 * time.Millisecond, MaxDelay: 8 * time.Millisecond}, Jitter: func(delay time.Duration) time.Duration { return delay }, Sleep: sleep})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestBoundedParallelismAndMergedDependencies(t *testing.T) {
	release := make(chan struct{})
	started := make(chan string, 3)
	store := &memoryStore{}
	isolation := &fakeIsolation{bases: map[string]string{}}
	integration := &fakeIntegration{mergedStages: map[string]bool{}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{}, started: started, release: release, merged: integration.merged}
	s := newScheduler(t, runtime, store, isolation, integration, nil)
	p := plan(2, false,
		model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}},
		model.Stage{ID: "B", Title: "B", Description: "B", WriteScope: []string{"B.txt"}},
		model.Stage{ID: "C", Title: "C", Description: "C", DependsOn: []string{"A", "B"}, WriteScope: []string{"C.txt"}},
	)
	done := make(chan scheduler.Result, 1)
	errs := make(chan error, 1)
	go func() {
		result, err := s.Start(context.Background(), model.RunState{ID: "run", ProjectID: "project", BaseCommit: "base", IntegrationHead: "base"}, p)
		if err != nil {
			errs <- err
			return
		}
		done <- result
	}()
	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case id := <-started:
			seen[id] = true
		case err := <-errs:
			t.Fatal(err)
		case <-time.After(time.Second):
			t.Fatal("parallel roots did not start")
		}
	}
	if !seen["A"] || !seen["B"] {
		t.Fatalf("first launches = %v", seen)
	}
	persisted, err := store.LoadRun(context.Background(), "run")
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range persisted.Stages {
		if seen[stage.ID] && (stage.WorkerPID != 1234 || stage.ExecutionID == "") {
			t.Fatalf("worker identity was not persisted while active: %+v", stage)
		}
	}
	select {
	case id := <-started:
		t.Fatalf("%s started before roots were released", id)
	default:
	}
	close(release)
	var result scheduler.Result
	select {
	case result = <-done:
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not finish")
	}
	if result.Run.Status != model.RunReadyToIntegrate {
		t.Fatalf("run status = %s", result.Run.Status)
	}
	runtime.mu.Lock()
	maximum := runtime.maximum
	runtime.mu.Unlock()
	if maximum != 2 {
		t.Fatalf("runtime maximum = %d", maximum)
	}
	integration.mu.Lock()
	integrationMaximum := integration.maximum
	rootHead := ""
	if len(integration.order) >= 2 {
		rootHead = "head-" + integration.order[1]
	}
	integration.mu.Unlock()
	if integrationMaximum != 1 {
		t.Fatalf("integration maximum = %d", integrationMaximum)
	}
	isolation.mu.Lock()
	cBase := isolation.bases["C"]
	isolation.mu.Unlock()
	if cBase != rootHead {
		t.Fatalf("C base = %q, root integration head = %q", cBase, rootHead)
	}
}

func TestMergedTransitionPropagatesEventPersistenceFailure(t *testing.T) {
	store := &memoryStore{eventError: func(event model.Event) error {
		if event.Type == "stage.transition" && strings.HasSuffix(event.Message, " -> Merged") {
			return errors.New("event store unavailable")
		}
		return nil
	}}
	integration := &fakeIntegration{mergedStages: map[string]bool{}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{}, merged: integration.merged}
	s := newScheduler(t, runtime, store, &fakeIsolation{bases: map[string]string{}}, integration, nil)
	_, err := s.Start(context.Background(), model.RunState{ID: "event-failure", ProjectID: "project", BaseCommit: "base", IntegrationHead: "base"}, plan(1, false, model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}}))
	if err == nil || !strings.Contains(err.Error(), "persist merged stage transition") {
		t.Fatalf("error = %v", err)
	}
	persisted, _ := store.LoadRun(context.Background(), "event-failure")
	if len(persisted.PendingEvents) != 1 || persisted.Stages[0].Status != model.StageMerged {
		t.Fatalf("pending merge event was not persisted: %+v", persisted)
	}
	store.mu.Lock()
	store.eventError = nil
	store.mu.Unlock()
	result, err := s.Resume(context.Background(), "event-failure")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Run.PendingEvents) != 0 || result.Run.Status != model.RunReadyToIntegrate {
		t.Fatalf("resumed run = %+v", result.Run)
	}
	mergedEvents := 0
	for _, event := range store.events {
		if event.ID == persisted.PendingEvents[0].ID {
			mergedEvents++
		}
	}
	if mergedEvents != 1 {
		t.Fatalf("merged transition events = %d", mergedEvents)
	}
}

func TestAllStageTransitionsUsePendingEventOutbox(t *testing.T) {
	store := &memoryStore{eventError: func(event model.Event) error {
		if event.Type == "stage.transition" && event.Message == "Ready -> Preparing" {
			return errors.New("event store unavailable")
		}
		return nil
	}}
	integration := &fakeIntegration{mergedStages: map[string]bool{}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{}, merged: integration.merged}
	s := newScheduler(t, runtime, store, &fakeIsolation{bases: map[string]string{}}, integration, nil)
	_, err := s.Start(context.Background(), model.RunState{ID: "transition-outbox", ProjectID: "project", BaseCommit: "base"}, plan(1, false, model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}}))
	if err == nil || !strings.Contains(err.Error(), "append pending event") {
		t.Fatalf("error = %v", err)
	}
	persisted, _ := store.LoadRun(context.Background(), "transition-outbox")
	if persisted.Stages[0].Status != model.StagePreparing || len(persisted.PendingEvents) != 1 || persisted.PendingEvents[0].Message != "Ready -> Preparing" {
		t.Fatalf("persisted run = %+v", persisted)
	}
}

func TestManualHoldUsesPendingEventOutbox(t *testing.T) {
	p := plan(1, false, model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}})
	store := &memoryStore{run: model.RunState{ID: "hold-outbox", ProjectID: "project", Status: model.RunRunning, Plan: p, Stages: []model.StageState{{ID: "A", Status: model.StageReady}}}, eventError: func(event model.Event) error {
		if event.Type == "stage.hold.changed" {
			return errors.New("event store unavailable")
		}
		return nil
	}}
	integration := &fakeIntegration{mergedStages: map[string]bool{}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{}, merged: integration.merged}
	s := newScheduler(t, runtime, store, &fakeIsolation{bases: map[string]string{}}, integration, nil)
	if err := s.HoldStage(context.Background(), "hold-outbox", "A", "pause"); err == nil {
		t.Fatal("hold succeeded despite event failure")
	}
	persisted, _ := store.LoadRun(context.Background(), "hold-outbox")
	if !persisted.Stages[0].Held || len(persisted.PendingEvents) != 1 || persisted.PendingEvents[0].Type != "stage.hold.changed" {
		t.Fatalf("persisted run = %+v", persisted)
	}
}

func TestPostMergeVerificationBlocksDependents(t *testing.T) {
	store := &memoryStore{}
	repo := &fakeRepo{committed: map[string][]string{}}
	integration := &fakeIntegration{mergedStages: map[string]bool{}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{}, merged: integration.merged}
	verifier := &integrationFailVerifier{}
	s, err := scheduler.New(scheduler.Config{
		Store: store, Runtime: runtime, Isolation: &fakeIsolation{bases: map[string]string{}}, Inspector: repo, Scope: repo,
		Verifier: verifier, Committer: repo, Integration: integration,
	})
	if err != nil {
		t.Fatal(err)
	}
	p := plan(1, false,
		model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}, Acceptance: []model.AcceptanceCriterion{{ID: "check", Type: "file-exists", Path: "A.txt"}}},
		model.Stage{ID: "B", Title: "B", Description: "B", DependsOn: []string{"A"}, WriteScope: []string{"B.txt"}},
	)
	result, err := s.Start(context.Background(), model.RunState{ID: "post-merge", ProjectID: "project", BaseCommit: "base", IntegrationWorkspace: "integration"}, p)
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Stages[0].Status != model.StageFailed || result.Run.Stages[0].Failure.Class != "post-merge-verification" || result.Run.Stages[1].Status != model.StageBlocked {
		t.Fatalf("stages = %+v", result.Run.Stages)
	}
	if runtime.attempts["B"] != 0 || result.Run.Status != model.RunFailed {
		t.Fatalf("dependent attempts=%d run=%s", runtime.attempts["B"], result.Run.Status)
	}
	if err := s.RetryStage(context.Background(), "post-merge", "A"); err != nil {
		t.Fatal(err)
	}
	retried, _ := store.LoadRun(context.Background(), "post-merge")
	if retried.Stages[0].Status != model.StagePostMergeVerifying || retried.Stages[0].CommitSHA == "" {
		t.Fatalf("retried post-merge stage = %+v", retried.Stages[0])
	}
}

func TestRetryIsBoundedAndBackedOff(t *testing.T) {
	store := &memoryStore{}
	isolation := &fakeIsolation{bases: map[string]string{}}
	integration := &fakeIntegration{mergedStages: map[string]bool{}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{"A": 1}, merged: integration.merged}
	var mu sync.Mutex
	var delays []time.Duration
	s := newScheduler(t, runtime, store, isolation, integration, func(_ context.Context, delay time.Duration) error {
		mu.Lock()
		delays = append(delays, delay)
		mu.Unlock()
		return nil
	})
	result, err := s.Start(context.Background(), model.RunState{ID: "retry", ProjectID: "project", BaseCommit: "base"}, plan(1, false, model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Stages[0].Attempt != 2 || result.Run.Stages[0].Status != model.StageMerged {
		t.Fatalf("stage = %+v", result.Run.Stages[0])
	}
	mu.Lock()
	defer mu.Unlock()
	if len(delays) != 0 {
		t.Fatalf("delays = %v", delays)
	}
}

func TestResumeCommitReadyIntegratesWithoutRerunningWorker(t *testing.T) {
	p := plan(1, false, model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}})
	store := &memoryStore{run: model.RunState{
		ID: "resume", ProjectID: "project", Status: model.RunRunning, Plan: p, BaseCommit: "base", IntegrationHead: "base",
		Stages: []model.StageState{{ID: "A", Status: model.StageCommitReady, Attempt: 1, Workspace: "A", Branch: "stage-A", CommitSHA: "commit-A", ChangedPaths: model.NewStringList([]string{"A.txt"})}},
	}}
	isolation := &fakeIsolation{bases: map[string]string{}}
	integration := &fakeIntegration{mergedStages: map[string]bool{}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{}, merged: integration.merged}
	s := newScheduler(t, runtime, store, isolation, integration, nil)
	result, err := s.Resume(context.Background(), "resume")
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Status != model.RunReadyToIntegrate || result.Run.Stages[0].Status != model.StageMerged {
		t.Fatalf("run = %+v", result.Run)
	}
	if runtime.attempts["A"] != 0 {
		t.Fatalf("worker reran %d times", runtime.attempts["A"])
	}
	if len(integration.order) != 1 || integration.order[0] != "A" {
		t.Fatalf("integration order = %v", integration.order)
	}
}

func TestResumeRefusesLivePersistedWorker(t *testing.T) {
	p := plan(1, false, model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}})
	store := &memoryStore{run: model.RunState{ID: "resume", ProjectID: "project", Status: model.RunRunning, Plan: p, Stages: []model.StageState{{ID: "A", Status: model.StageRunning, WorkerPID: 42}}}}
	repo := &fakeRepo{committed: map[string][]string{}}
	integration := &fakeIntegration{mergedStages: map[string]bool{}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{}, merged: integration.merged}
	s, err := scheduler.New(scheduler.Config{
		Store: store, Runtime: runtime, Isolation: &fakeIsolation{bases: map[string]string{}}, Inspector: repo, Scope: repo,
		Verifier: fakeVerifier{}, Committer: repo, Integration: integration, WorkerAlive: func(pid int) bool { return pid == 42 },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resume(context.Background(), "resume"); err == nil || !strings.Contains(err.Error(), "live worker") {
		t.Fatalf("error = %v", err)
	}
}

func TestConfiguredSchedulerPolicyControlsRetry(t *testing.T) {
	store := &memoryStore{}
	repo := &fakeRepo{committed: map[string][]string{}}
	integration := &fakeIntegration{mergedStages: map[string]bool{}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{"A": 3}, merged: integration.merged}
	policy := &denyingPolicy{}
	s, err := scheduler.New(scheduler.Config{
		Store: store, Runtime: runtime, Isolation: &fakeIsolation{bases: map[string]string{}}, Inspector: repo, Scope: repo,
		Verifier: fakeVerifier{}, Committer: repo, Integration: integration, Policy: policy,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.Start(context.Background(), model.RunState{ID: "policy", ProjectID: "project", BaseCommit: "base"}, plan(1, false, model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}}))
	if err != nil {
		t.Fatal(err)
	}
	if !policy.ordered || !policy.retried || result.Run.Stages[0].Attempt != 1 || result.Run.Stages[0].Status != model.StageFailed {
		t.Fatalf("policy=%+v stage=%+v", policy, result.Run.Stages[0])
	}
}

func TestCleanupFailureAfterIntegrationDoesNotReintegrateCommit(t *testing.T) {
	store := &memoryStore{}
	isolation := &fakeIsolation{bases: map[string]string{}, cleanupErr: errors.New("busy worktree")}
	integration := &fakeIntegration{mergedStages: map[string]bool{}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{}, merged: integration.merged}
	s := newScheduler(t, runtime, store, isolation, integration, nil)
	result, err := s.Start(context.Background(), model.RunState{ID: "cleanup", ProjectID: "project", BaseCommit: "base"}, plan(1, false, model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Stages[0].Status != model.StageMerged || len(integration.order) != 1 {
		t.Fatalf("stage=%+v integrations=%v", result.Run.Stages[0], integration.order)
	}
	found := false
	for _, event := range store.events {
		found = found || event.Type == "cleanup.deferred"
	}
	if !found {
		t.Fatal("deferred cleanup was not recorded")
	}
}

func TestResumeRetriesFinalVerificationWithoutRerunningMergedStages(t *testing.T) {
	store := &memoryStore{}
	repo := &fakeRepo{committed: map[string][]string{}}
	integration := &fakeIntegration{mergedStages: map[string]bool{}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{}, merged: integration.merged}
	verifier := &finalFlakyVerifier{}
	s, err := scheduler.New(scheduler.Config{
		Store: store, Runtime: runtime, Isolation: &fakeIsolation{bases: map[string]string{}}, Inspector: repo, Scope: repo,
		Verifier: verifier, Committer: repo, Integration: integration, Jitter: func(delay time.Duration) time.Duration { return delay },
	})
	if err != nil {
		t.Fatal(err)
	}
	p := plan(1, false, model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}, Acceptance: []model.AcceptanceCriterion{{ID: "check", Type: "file-exists", Path: "A.txt"}}})
	p.Spec.Policy.FinalVerificationRequired = true
	first, err := s.Start(context.Background(), model.RunState{ID: "final", ProjectID: "project", BaseCommit: "base", IntegrationWorkspace: "integration"}, p)
	if err != nil {
		t.Fatal(err)
	}
	if first.Run.Status != model.RunFailed || first.Run.Stages[0].Status != model.StageMerged {
		t.Fatalf("first run = %+v", first.Run)
	}
	second, err := s.Resume(context.Background(), "final")
	if err != nil {
		t.Fatal(err)
	}
	if second.Run.Status != model.RunReadyToIntegrate || runtime.attempts["A"] != 1 {
		t.Fatalf("second run = %+v attempts=%v", second.Run, runtime.attempts)
	}
}

func TestFailFastCancelsNewWorkAndBlocksDependents(t *testing.T) {
	store := &memoryStore{}
	isolation := &fakeIsolation{bases: map[string]string{}}
	integration := &fakeIntegration{mergedStages: map[string]bool{}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{"A": 10}, merged: integration.merged}
	s := newScheduler(t, runtime, store, isolation, integration, func(context.Context, time.Duration) error { return nil })
	p := plan(1, true, model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}}, model.Stage{ID: "B", Title: "B", Description: "B", WriteScope: []string{"B.txt"}}, model.Stage{ID: "C", Title: "C", Description: "C", DependsOn: []string{"A"}, WriteScope: []string{"C.txt"}})
	p.Spec.Policy.MaxAttemptsPerStage = 1
	result, err := s.Start(context.Background(), model.RunState{ID: "fail-fast", ProjectID: "project", BaseCommit: "base"}, p)
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[string]model.StageStatus{}
	for _, stage := range result.Run.Stages {
		statuses[stage.ID] = stage.Status
	}
	if statuses["A"] != model.StageFailed || statuses["B"] != model.StageCancelled || statuses["C"] != model.StageBlocked {
		t.Fatalf("statuses = %v", statuses)
	}
	if result.Run.Status != model.RunFailed {
		t.Fatalf("run status = %s", result.Run.Status)
	}
}

func TestMergeConflictPersistsExactPathsAndBlocksDependents(t *testing.T) {
	store := &memoryStore{}
	isolation := &fakeIsolation{bases: map[string]string{}}
	integration := &fakeIntegration{mergedStages: map[string]bool{}, conflicts: map[string][]string{"A": {"one.txt", "two.txt"}}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{}, merged: integration.merged}
	s := newScheduler(t, runtime, store, isolation, integration, nil)
	p := plan(1, false,
		model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}},
		model.Stage{ID: "B", Title: "B", Description: "B", DependsOn: []string{"A"}, WriteScope: []string{"B.txt"}},
	)
	result, err := s.Start(context.Background(), model.RunState{ID: "conflict", ProjectID: "project", BaseCommit: "base"}, p)
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Stages[0].Status != model.StageMergeConflict || result.Run.Stages[1].Status != model.StageBlocked {
		t.Fatalf("stages = %+v", result.Run.Stages)
	}
	paths := result.Run.Stages[0].ConflictingPaths.Values()
	if len(paths) != 2 || paths[0] != "one.txt" || paths[1] != "two.txt" {
		t.Fatalf("conflicting paths = %v", paths)
	}
	integration.mu.Lock()
	integration.conflicts = nil
	integration.mu.Unlock()
	if err := s.RetryStage(context.Background(), "conflict", "A"); err != nil {
		t.Fatal(err)
	}
	resumed, err := s.Resume(context.Background(), "conflict")
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Run.Status != model.RunReadyToIntegrate {
		t.Fatalf("resumed run status = %s", resumed.Run.Status)
	}
}

func TestAutomaticConflictResolutionReachesMerged(t *testing.T) {
	store := &memoryStore{}
	isolation := &fakeIsolation{bases: map[string]string{}}
	integration := &fakeIntegration{mergedStages: map[string]bool{}, conflicts: map[string][]string{"A": {"A.txt"}}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{}, merged: integration.merged}
	resolver := &fakeConflictResolver{result: orchestration.ConflictResolutionResult{CommitSHA: "resolved-A", Evidence: []string{"resolution accepted"}}}
	s := newScheduler(t, runtime, store, isolation, integration, nil, resolver)
	p := plan(1, false, model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}})
	p.Spec.Policy.AutoResolveConflicts = true
	result, err := s.Start(context.Background(), model.RunState{ID: "auto-conflict", ProjectID: "project", BaseCommit: "base", IntegrationHead: "base"}, p)
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Status != model.RunReadyToIntegrate || result.Run.Stages[0].Status != model.StageMerged {
		t.Fatalf("run = %+v", result.Run)
	}
	if result.Run.IntegrationHead != "resolved-A" || resolver.req.IntegrationHead != "base" {
		t.Fatalf("integration head = %q, request = %+v", result.Run.IntegrationHead, resolver.req)
	}
}

func TestAutomaticConflictResolutionFailureNeedsHumanReview(t *testing.T) {
	store := &memoryStore{}
	isolation := &fakeIsolation{bases: map[string]string{}}
	integration := &fakeIntegration{mergedStages: map[string]bool{}, conflicts: map[string][]string{"A": {"A.txt"}}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{}, merged: integration.merged}
	resolver := &fakeConflictResolver{result: orchestration.ConflictResolutionResult{Err: errors.New("unresolved")}}
	s := newScheduler(t, runtime, store, isolation, integration, nil, resolver)
	p := plan(1, false, model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}})
	p.Spec.Policy.AutoResolveConflicts = true
	result, err := s.Start(context.Background(), model.RunState{ID: "failed-conflict", ProjectID: "project", BaseCommit: "base", IntegrationHead: "base"}, p)
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Stages[0].Status != model.StageNeedsHumanReview || result.Run.Stages[0].Failure.Class != "conflict-resolution" {
		t.Fatalf("stage = %+v", result.Run.Stages[0])
	}
}

func TestContextCancellationIsPersistedAndPreventsLaunch(t *testing.T) {
	release := make(chan struct{})
	started := make(chan string, 2)
	store := &memoryStore{}
	isolation := &fakeIsolation{bases: map[string]string{}}
	integration := &fakeIntegration{mergedStages: map[string]bool{}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{}, started: started, release: release, merged: integration.merged}
	s := newScheduler(t, runtime, store, isolation, integration, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan scheduler.Result, 1)
	errs := make(chan error, 1)
	go func() {
		result, err := s.Start(ctx, model.RunState{ID: "cancel", ProjectID: "project", BaseCommit: "base"}, plan(1, false, model.Stage{ID: "A", Title: "A", Description: "A", WriteScope: []string{"A.txt"}}, model.Stage{ID: "B", Title: "B", Description: "B", WriteScope: []string{"B.txt"}}))
		if err != nil {
			errs <- err
			return
		}
		done <- result
	}()
	select {
	case id := <-started:
		if id != "A" {
			t.Fatalf("first stage = %s", id)
		}
	case <-time.After(time.Second):
		t.Fatal("A did not start")
	}
	cancel()
	select {
	case result := <-done:
		if result.Run.Status != model.RunCancelled || result.Run.Cancellation == nil {
			t.Fatalf("run = %+v", result.Run)
		}
		if result.Run.Stages[1].Attempt != 0 {
			t.Fatalf("B launched: %+v", result.Run.Stages[1])
		}
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("cancellation did not finish")
	}
}

func TestPromptIsStable(t *testing.T) {
	p := plan(1, false)
	stage := model.Stage{ID: "A", Title: "title", Description: "task", WriteScope: []string{"z", "a"}}
	got := scheduler.StagePrompt(p, stage)
	for _, required := range []string{"north.stage/v1", "Goal: test", "Stage: A - title", "Write scope: z, a", "Acceptance criteria:", "Do not modify North state", "Host verification decides completion"} {
		if !strings.Contains(got, required) {
			t.Fatalf("prompt missing %q:\n%s", required, got)
		}
	}
}

func TestResumeCancellationPreservesMergedStagesAndPostMergeVerification(t *testing.T) {
	integration := &fakeIntegration{mergedStages: map[string]bool{"A": true, "B": true}}
	runtime := &fakeRuntime{attempts: map[string]int{}, failures: map[string]int{}, merged: integration.merged}
	p := plan(1, false, model.Stage{ID: "A", WriteScope: []string{"A.txt"}}, model.Stage{ID: "B", DependsOn: []string{"A"}, WriteScope: []string{"B.txt"}})
	p.Spec.Policy.MaxAttemptsPerStage = 1
	store := &memoryStore{run: model.RunState{ID: "cancelled", ProjectID: "project", Status: model.RunCancelled, Plan: p, BaseCommit: "base", IntegrationHead: "merged-b", Cancellation: &model.Cancellation{}, Stages: []model.StageState{
		{ID: "A", Status: model.StageMerged, Attempt: 1, CommitSHA: "merged-a"},
		{ID: "B", Status: model.StageCancelled, CancelledFrom: model.StagePostMergeVerifying, Attempt: 1, CommitSHA: "merged-b"},
	}}}
	s := newScheduler(t, runtime, store, &fakeIsolation{bases: map[string]string{}}, integration, func(context.Context, time.Duration) error { return nil })
	result, err := s.Resume(context.Background(), "cancelled")
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Status != model.RunReadyToIntegrate || result.Run.Cancellation != nil || len(runtime.attempts) != 0 {
		t.Fatalf("resume reran completed work: %+v attempts=%v", result.Run, runtime.attempts)
	}
}
