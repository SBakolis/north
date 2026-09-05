// Package scheduler executes dependency plans with bounded concurrency.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SBakolis/north/internal/dag"
	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/orchestration"
)

var (
	ErrNoChanges  = errors.New("stage produced no changes")
	ErrNotRunning = errors.New("run is not active in this scheduler")
)

type RetryConfig struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

type Config struct {
	Store        orchestration.StateStore
	Runtime      orchestration.AgentRuntime
	Isolation    orchestration.IsolationProvider
	Inspector    orchestration.ChangedPathInspector
	Scope        orchestration.WriteScopeVerifier
	Verifier     orchestration.VerificationProvider
	Committer    orchestration.ExactPathCommitter
	Integration  orchestration.IntegrationProvider
	Conflicts    orchestration.ConflictResolutionProvider
	Classifier   orchestration.FailureClassifier
	Policy       orchestration.SchedulerPolicy
	Retry        RetryConfig
	Jitter       func(time.Duration) time.Duration
	WorkerAlive  func(int) bool
	StageTimeout time.Duration
	PollInterval time.Duration
	Now          func() time.Time
	Sleep        func(context.Context, time.Duration) error
}

type Result struct {
	Run model.RunState
}

type Scheduler struct {
	config Config
	mu     sync.Mutex
	active map[string]*execution
}

type execution struct {
	mu      sync.Mutex
	persist sync.Mutex
	run     model.RunState
	cancel  context.CancelFunc
	wake    chan struct{}
	merge   sync.Mutex
}

type workerResult struct {
	stageID string
	failed  bool
	err     error
}

func New(config Config) (*Scheduler, error) {
	if config.Store == nil || config.Runtime == nil || config.Isolation == nil || config.Inspector == nil ||
		config.Scope == nil || config.Verifier == nil || config.Committer == nil || config.Integration == nil {
		return nil, errors.New("scheduler requires store, runtime, isolation, inspector, scope, verifier, committer, and integration")
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.Sleep == nil {
		config.Sleep = sleepContext
	}
	if config.Retry.BaseDelay <= 0 {
		config.Retry.BaseDelay = time.Second
	}
	if config.Retry.MaxDelay <= 0 {
		config.Retry.MaxDelay = time.Minute
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 250 * time.Millisecond
	}
	if config.Jitter == nil {
		config.Jitter = func(delay time.Duration) time.Duration {
			spread := delay / 5
			if spread <= 0 {
				return delay
			}
			return delay - spread + time.Duration(rand.Int64N(int64(2*spread)+1))
		}
	}
	return &Scheduler{config: config, active: make(map[string]*execution)}, nil
}

// Start creates durable state and executes plan. The supplied run carries the
// repository and integration workspace identity resolved by the application.
func (s *Scheduler) Start(ctx context.Context, run model.RunState, plan model.ExecutionPlan) (Result, error) {
	if _, err := dag.New(plan); err != nil {
		return Result{}, err
	}
	if len(plan.Spec.Stages) == 0 {
		return Result{}, errors.New("plan has no stages")
	}
	if plan.Spec.Policy.MaxParallel < 1 || plan.Spec.Policy.MaxAttemptsPerStage < 1 {
		return Result{}, errors.New("plan maxParallel and maxAttemptsPerStage must be at least 1")
	}
	if err := s.config.Runtime.Validate(ctx); err != nil {
		return Result{}, fmt.Errorf("validate runtime: %w", err)
	}
	now := s.config.Now().UTC()
	run.Plan = plan
	run.Status = model.RunPreparing
	if run.SchemaVersion == 0 {
		run.SchemaVersion = 1
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	run.UpdatedAt = now
	run.Stages = initialStages(plan, run.Stages)
	if err := s.config.Store.CreateRun(ctx, run); err != nil {
		return Result{}, err
	}
	if locker, ok := s.config.Store.(orchestration.RunLockProvider); ok {
		lock, err := locker.AcquireSchedulerRunLock(ctx, run.ProjectID, run.ID)
		if err != nil {
			return Result{Run: run}, err
		}
		defer lock.Release()
	}
	if err := s.persistRunTransition(ctx, &run, model.RunRunning); err != nil {
		return Result{Run: run}, err
	}
	return s.execute(ctx, run)
}

func (s *Scheduler) Resume(ctx context.Context, runID string) (Result, error) {
	run, err := s.config.Store.LoadRun(ctx, runID)
	if err != nil {
		return Result{}, err
	}
	if locker, ok := s.config.Store.(orchestration.RunLockProvider); ok {
		lock, err := locker.AcquireSchedulerRunLock(ctx, run.ProjectID, run.ID)
		if err != nil {
			return Result{Run: run}, err
		}
		defer lock.Release()
	}
	if err := s.replayPendingEvents(ctx, &run); err != nil {
		return Result{Run: run}, err
	}
	if _, err := dag.New(run.Plan); err != nil {
		return Result{Run: run}, err
	}
	if run.Status == model.RunFailed && run.Failure != nil && run.Failure.Class == "final-verification" && run.Failure.Retryable {
		run.Failure = nil
		if err := s.persistRunTransition(ctx, &run, model.RunRunning); err != nil {
			return Result{Run: run}, err
		}
	}
	if run.Status != model.RunRunning {
		return Result{Run: run}, fmt.Errorf("run %q cannot resume from %s", run.ID, run.Status)
	}
	for i := range run.Stages {
		if (run.Stages[i].Status == model.StagePreparing || run.Stages[i].Status == model.StageRunning || run.Stages[i].Status == model.StageVerifying || run.Stages[i].Status == model.StageMerging) && run.Stages[i].WorkerPID > 0 && s.config.WorkerAlive != nil && s.config.WorkerAlive(run.Stages[i].WorkerPID) {
			return Result{Run: run}, fmt.Errorf("stage %q still has live worker pid %d; stop that process before resume", run.Stages[i].ID, run.Stages[i].WorkerPID)
		}
		if run.Stages[i].Status == model.StageRetryScheduled {
			continue
		}
		if orchestration.IsActiveStage(run.Stages[i].Status) && run.Stages[i].Status != model.StageCommitReady && run.Stages[i].Attempt >= run.Plan.Spec.Policy.MaxAttemptsPerStage {
			from := run.Stages[i].Status
			if err := orchestration.TransitionStage(&run.Stages[i], model.StageFailed, s.config.Now()); err != nil {
				return Result{Run: run}, err
			}
			run.Stages[i].Failure = &model.StageFailure{Class: "interrupted", Message: "attempt budget exhausted while recovering from " + string(from), Retryable: false}
			if err := s.persistStoredStageTransition(ctx, &run, i, from); err != nil {
				return Result{Run: run}, err
			}
			continue
		}
		to, normalize := orchestration.ResumeStatus(run.Stages[i].Status)
		if !normalize {
			continue
		}
		from := run.Stages[i].Status
		if err := orchestration.TransitionStage(&run.Stages[i], to, s.config.Now()); err != nil {
			return Result{Run: run}, err
		}
		run.Stages[i].Failure = &model.StageFailure{Class: "interrupted", Message: "orphaned while " + string(from), Retryable: true}
		if err := s.persistStoredStageTransition(ctx, &run, i, from); err != nil {
			return Result{Run: run}, err
		}
		if to == model.StageRetryScheduled {
			retryFrom := run.Stages[i].Status
			if err := orchestration.TransitionStage(&run.Stages[i], model.StageReady, s.config.Now()); err != nil {
				return Result{Run: run}, err
			}
			if err := s.persistStoredStageTransition(ctx, &run, i, retryFrom); err != nil {
				return Result{Run: run}, err
			}
		}
	}
	return s.execute(ctx, run)
}

func initialStages(plan model.ExecutionPlan, supplied []model.StageState) []model.StageState {
	byID := make(map[string]model.StageState, len(supplied))
	for _, state := range supplied {
		byID[state.ID] = state
	}
	states := make([]model.StageState, 0, len(plan.Spec.Stages))
	for _, stage := range plan.Spec.Stages {
		state, ok := byID[stage.ID]
		if !ok {
			state = model.StageState{SchemaVersion: 1, ID: stage.ID, Status: model.StageWaitingForDependencies}
		}
		states = append(states, state)
	}
	return states
}

func (s *Scheduler) execute(parent context.Context, run model.RunState) (Result, error) {
	ctx, cancel := context.WithCancel(parent)
	exec := &execution{run: run, cancel: cancel, wake: make(chan struct{}, 1)}
	if err := s.register(exec); err != nil {
		cancel()
		return Result{Run: run}, err
	}
	defer func() { cancel(); s.unregister(run.ID, exec) }()

	results := make(chan workerResult, len(run.Stages))
	running := make(map[string]bool)
	failFast := false
	poll := time.NewTicker(s.config.PollInterval)
	defer poll.Stop()
	finalVerified := false
	for {
		if parent.Err() != nil && !s.cancelRequested(exec) {
			if err := s.requestCancellation(exec, parent.Err().Error()); err != nil {
				return s.result(exec), err
			}
		}
		if s.cancelRequested(exec) {
			cancel()
			if err := s.cancelUnstarted(exec, cancellationReason(exec, parent)); err != nil {
				return s.result(exec), err
			}
		}
		if err := s.refreshDependencies(exec); err != nil {
			return s.result(exec), err
		}
		if err := s.refreshRetries(exec); err != nil {
			return s.result(exec), err
		}

		maxParallel := exec.run.Plan.Spec.Policy.MaxParallel
		if maxParallel < 1 {
			maxParallel = 1
		}
		for _, id := range s.readyIDs(ctx, exec) {
			if len(running) >= maxParallel || ctx.Err() != nil || failFast {
				break
			}
			if running[id] {
				continue
			}
			_, candidate, _ := stageAndState(exec, id)
			if candidate.CommitSHA == "" {
				if err := s.transition(exec, id, model.StagePreparing, func(stage *model.StageState) {
					stage.Attempt++
					stage.Failure = nil
					stage.RetryEligibleAt = time.Time{}
					stage.ConflictingPaths = ""
					stage.Evidence = ""
					stage.ChangedPaths = ""
				}); err != nil {
					return s.result(exec), err
				}
			}
			running[id] = true
			go func(stageID string) { results <- s.runAttempt(ctx, exec, stageID) }(id)
		}

		if len(running) == 0 {
			status, done := finalStatus(exec)
			if done {
				if status == model.RunReadyToIntegrate && !finalVerified {
					if err := s.finalVerification(parent, exec); err != nil {
						exec.mu.Lock()
						exec.run.Failure = &model.StageFailure{Class: "final-verification", Message: err.Error(), Retryable: true}
						exec.mu.Unlock()
						if transitionErr := s.persistExecutionRunTransition(context.WithoutCancel(parent), exec, model.RunFailed); transitionErr != nil {
							return s.result(exec), errors.Join(err, transitionErr)
						}
						return s.result(exec), nil
					}
					finalVerified = true
				}
				if err := s.persistExecutionRunTransition(context.WithoutCancel(parent), exec, status); err != nil {
					return s.result(exec), err
				}
				return s.result(exec), nil
			}
		}

		select {
		case result := <-results:
			delete(running, result.stageID)
			if result.err != nil {
				return s.result(exec), result.err
			}
			if result.failed && exec.run.Plan.Spec.Policy.FailFast && !failFast {
				failFast = true
				cancel()
				if err := s.refreshDependencies(exec); err != nil {
					return s.result(exec), err
				}
				if err := s.cancelUnstarted(exec, "fail-fast"); err != nil {
					return s.result(exec), err
				}
			}
		case <-exec.wake:
		case <-poll.C:
			persisted, err := s.config.Store.LoadRun(parent, run.ID)
			if err != nil {
				return s.result(exec), err
			}
			syncStoredHolds(exec, persisted)
			if persisted.Cancellation != nil {
				exec.mu.Lock()
				exec.run.Cancellation = persisted.Cancellation
				exec.mu.Unlock()
				cancel()
			}
		case <-parent.Done():
			cancel()
			if err := s.requestCancellation(exec, parent.Err().Error()); err != nil {
				return s.result(exec), err
			}
		}
	}
}

func syncStoredHolds(exec *execution, persisted model.RunState) {
	exec.mu.Lock()
	defer exec.mu.Unlock()
	for _, stored := range persisted.Stages {
		index := stageIndex(exec.run.Stages, stored.ID)
		if index >= 0 && exec.run.Stages[index].Status == stored.Status && !orchestration.IsActiveStage(stored.Status) {
			exec.run.Stages[index].Held = stored.Held
			exec.run.Stages[index].HoldReason = stored.HoldReason
		}
	}
}

func (s *Scheduler) runAttempt(ctx context.Context, exec *execution, stageID string) workerResult {
	stage, stageState, ok := stageAndState(exec, stageID)
	if !ok {
		return workerResult{stageID: stageID, failed: true}
	}
	if err := ctx.Err(); err != nil {
		return s.cancelStage(exec, stageID, err.Error())
	}
	if stageState.Status == model.StagePostMergeVerifying {
		return s.verifyIntegratedStage(ctx, exec, stage, orchestration.Workspace{Path: stageState.Workspace, Branch: stageState.Branch}, stageState.CommitSHA, nil)
	}
	if stageState.CommitSHA != "" {
		return s.integrateCommit(ctx, exec, stage, orchestration.Workspace{Path: stageState.Workspace, Branch: stageState.Branch}, stageState.CommitSHA)
	}
	base := s.integrationHead(exec)
	workspace := orchestration.Workspace{Path: stageState.Workspace, Branch: stageState.Branch}
	if workspace.Path == "" {
		var err error
		workspace, err = s.config.Isolation.Prepare(ctx, orchestration.IsolationRequest{RunID: exec.run.ID, StageID: stageID, BaseCommit: base})
		if err != nil {
			return s.failAttempt(ctx, exec, stageID, "prepare", err)
		}
	}
	if err := s.transition(exec, stageID, model.StageRunning, func(st *model.StageState) {
		st.Workspace, st.Branch, st.StartedAt = workspace.Path, workspace.Branch, s.config.Now().UTC()
	}); err != nil {
		return workerResult{stageID: stageID, failed: true, err: err}
	}
	timeoutCtx, cancel := withOptionalTimeout(ctx, s.config.StageTimeout)
	result, err := s.config.Runtime.Execute(timeoutCtx, orchestration.AgentRequest{
		RunID: exec.run.ID, StageID: stageID, Workspace: workspace.Path, Prompt: StagePrompt(exec.run.Plan, stage),
		Agent: stage.Agent, SessionID: stageState.SessionID, Role: "worker", Timeout: s.config.StageTimeout,
		Started: func(started orchestration.AgentExecution) error {
			return s.persistWorkerStart(exec, stageID, started)
		},
	}, activitySink{s: s, exec: exec, stageID: stageID})
	cancel()
	if err != nil {
		if persistErr := s.persistWorkerStopped(exec, stageID, result); persistErr != nil {
			return workerResult{stageID: stageID, failed: true, err: errors.Join(err, persistErr)}
		}
		return s.failAttempt(ctx, exec, stageID, "runtime", err)
	}
	if result.ExitCode != 0 {
		return s.failAttempt(ctx, exec, stageID, "runtime", fmt.Errorf("agent exited with code %d", result.ExitCode))
	}
	if err := s.transition(exec, stageID, model.StageVerifying, func(st *model.StageState) {
		st.ExecutionID, st.SessionID, st.WorkerPID = result.ExecutionID, result.SessionID, 0
	}); err != nil {
		return workerResult{stageID: stageID, failed: true, err: err}
	}
	paths, err := s.config.Inspector.ChangedPaths(ctx, workspace.Path, "HEAD")
	if err != nil {
		return s.failAttempt(ctx, exec, stageID, "inspect", err)
	}
	sort.Strings(paths)
	if err := s.config.Scope.VerifyWriteScope(ctx, workspace.Path, paths, stage.WriteScope); err != nil {
		return s.failAttempt(ctx, exec, stageID, "scope", err)
	}
	if len(paths) == 0 && !stage.AllowNoChanges {
		return s.failAttempt(ctx, exec, stageID, "changes", ErrNoChanges)
	}
	verification := s.config.Verifier.Verify(ctx, orchestration.VerificationRequest{RunID: exec.run.ID, StageID: stageID, Workspace: workspace.Path, Criteria: stage.Acceptance, WriteScope: stage.WriteScope}, storeSink{s.config.Store})
	if !verification.Passed {
		exec.mu.Lock()
		if index := stageIndex(exec.run.Stages, stageID); index >= 0 {
			exec.run.Stages[index].Evidence = model.NewStringList(verification.Evidence)
		}
		exec.mu.Unlock()
		failure := verification.Failure
		if failure == nil {
			failure = &model.StageFailure{Class: "verification", Message: "acceptance verification failed", Retryable: true}
		}
		return s.failClassified(ctx, exec, stageID, *failure)
	}
	sha := base
	if len(paths) > 0 {
		sha, err = s.config.Committer.CommitPaths(ctx, orchestration.ExactPathCommitRequest{RunID: exec.run.ID, StageID: stageID, Workspace: workspace.Path, Message: "north(" + stage.ID + "): " + stage.Title, Paths: paths})
		if err != nil {
			return s.failAttempt(ctx, exec, stageID, "commit", err)
		}
	}
	if err := s.transition(exec, stageID, model.StageCommitReady, func(st *model.StageState) {
		st.CommitSHA, st.ChangedPaths, st.Evidence = sha, model.NewStringList(paths), model.NewStringList(verification.Evidence)
	}); err != nil {
		return workerResult{stageID: stageID, failed: true, err: err}
	}
	return s.integrateCommit(ctx, exec, stage, workspace, sha)
}

func (s *Scheduler) integrateCommit(ctx context.Context, exec *execution, stage model.Stage, workspace orchestration.Workspace, sha string) workerResult {
	stageID := stage.ID
	_, current, _ := stageAndState(exec, stageID)
	exec.merge.Lock()
	defer exec.merge.Unlock()
	if err := ctx.Err(); err != nil {
		return s.cancelStage(exec, stageID, err.Error())
	}
	if err := s.transition(exec, stageID, model.StageMerging, nil); err != nil {
		return workerResult{stageID: stageID, failed: true, err: err}
	}
	if len(current.ChangedPaths.Values()) > 0 {
		integration := s.config.Integration.IntegrateStage(ctx, orchestration.StageIntegrationRequest{RunID: exec.run.ID, StageID: stageID, CommitSHA: sha})
		if integration.Err != nil {
			if len(integration.ConflictingPaths) > 0 {
				sort.Strings(integration.ConflictingPaths)
				if exec.run.Plan.Spec.Policy.AutoResolveConflicts {
					if s.config.Conflicts == nil {
						if err := s.transition(exec, stageID, model.StageNeedsHumanReview, func(st *model.StageState) {
							st.ConflictingPaths = model.NewStringList(integration.ConflictingPaths)
							st.Failure = &model.StageFailure{Class: "conflict-resolution", Message: "automatic conflict resolution is enabled but no provider is configured", Retryable: false}
						}); err != nil {
							return workerResult{stageID: stageID, failed: true, err: err}
						}
						return workerResult{stageID: stageID, failed: true}
					}
					resolution := s.config.Conflicts.ResolveConflict(ctx, orchestration.ConflictResolutionRequest{
						RunID: exec.run.ID, StageID: stageID, IntegrationHead: s.integrationHead(exec), CommitSHA: sha,
						ConflictingPaths: integration.ConflictingPaths, WriteScope: stage.WriteScope, Criteria: stage.Acceptance,
						Started:  func(started orchestration.AgentExecution) error { return s.persistWorkerStart(exec, stageID, started) },
						Finished: func(result orchestration.AgentResult) error { return s.persistWorkerStopped(exec, stageID, result) },
					}, storeSink{s.config.Store})
					if resolution.Err != nil || resolution.CommitSHA == "" {
						message := "conflict resolver returned no commit"
						if resolution.Err != nil {
							message = resolution.Err.Error()
						}
						if err := s.transition(exec, stageID, model.StageNeedsHumanReview, func(st *model.StageState) {
							st.ConflictingPaths = model.NewStringList(integration.ConflictingPaths)
							st.Evidence = model.NewStringList(resolution.Evidence)
							st.Failure = &model.StageFailure{Class: "conflict-resolution", Message: message, Retryable: false}
						}); err != nil {
							return workerResult{stageID: stageID, failed: true, err: err}
						}
						return workerResult{stageID: stageID, failed: true}
					}
					sha = resolution.CommitSHA
					return s.verifyIntegratedStage(ctx, exec, stage, workspace, sha, func(st *model.StageState) {
						st.CommitSHA = sha
						st.ConflictingPaths = model.NewStringList(integration.ConflictingPaths)
						st.Evidence = model.NewStringList(append(st.Evidence.Values(), resolution.Evidence...))
					})
				}
				if err := s.transition(exec, stageID, model.StageMergeConflict, func(st *model.StageState) {
					st.ConflictingPaths = model.NewStringList(integration.ConflictingPaths)
					st.Failure = &model.StageFailure{Class: "merge-conflict", Message: integration.Err.Error(), Retryable: false}
				}); err != nil {
					return workerResult{stageID: stageID, failed: true, err: err}
				}
				return workerResult{stageID: stageID, failed: true}
			}
			return s.failAttempt(ctx, exec, stageID, "integration", integration.Err)
		}
		sha = integration.CommitSHA
	} else {
		sha = s.integrationHead(exec)
	}
	return s.verifyIntegratedStage(ctx, exec, stage, workspace, sha, func(st *model.StageState) { st.CommitSHA = sha })
}

func (s *Scheduler) verifyIntegratedStage(ctx context.Context, exec *execution, stage model.Stage, workspace orchestration.Workspace, sha string, mutate func(*model.StageState)) workerResult {
	stageID := stage.ID
	if persisted := s.persistIntegrationHead(ctx, exec, stageID, sha); persisted.err != nil {
		return persisted
	}
	_, current, _ := stageAndState(exec, stageID)
	if current.Status == model.StageMerging {
		if err := s.transition(exec, stageID, model.StagePostMergeVerifying, mutate); err != nil {
			return workerResult{stageID: stageID, failed: true, err: err}
		}
		mutate = nil
	}
	criteria := make([]model.AcceptanceCriterion, 0, len(stage.Acceptance))
	for _, criterion := range stage.Acceptance {
		if criterion.Type != "git-diff-not-empty" {
			criteria = append(criteria, criterion)
		}
	}
	exec.mu.Lock()
	integrationWorkspace := exec.run.IntegrationWorkspace
	exec.mu.Unlock()
	verification := s.config.Verifier.Verify(ctx, orchestration.VerificationRequest{
		RunID: exec.run.ID, StageID: stageID, Workspace: integrationWorkspace,
		Criteria: criteria, WriteScope: stage.WriteScope,
	}, storeSink{s.config.Store})
	if !verification.Passed {
		message := "post-merge acceptance verification failed"
		if verification.Failure != nil && verification.Failure.Message != "" {
			message = verification.Failure.Message
		}
		err := s.transition(exec, stageID, model.StageFailed, func(state *model.StageState) {
			if mutate != nil {
				mutate(state)
			}
			state.Evidence = model.NewStringList(append(state.Evidence.Values(), verification.Evidence...))
			state.Failure = &model.StageFailure{Class: "post-merge-verification", Message: message, Retryable: false}
		})
		return workerResult{stageID: stageID, failed: true, err: err}
	}
	if err := s.completeStageMerge(ctx, exec, stageID, sha, func(state *model.StageState) {
		if mutate != nil {
			mutate(state)
		}
		state.Evidence = model.NewStringList(append(state.Evidence.Values(), verification.Evidence...))
	}); err != nil {
		return workerResult{stageID: stageID, failed: true, err: err}
	}
	s.cleanupMergedWorkspace(ctx, exec.run.ID, stageID, workspace)
	return workerResult{stageID: stageID}
}

func (s *Scheduler) completeStageMerge(ctx context.Context, exec *execution, stageID, sha string, mutate func(*model.StageState)) error {
	exec.persist.Lock()
	defer exec.persist.Unlock()
	exec.mu.Lock()
	index := stageIndex(exec.run.Stages, stageID)
	if index < 0 {
		exec.mu.Unlock()
		return fmt.Errorf("unknown stage %q", stageID)
	}
	from := exec.run.Stages[index].Status
	if err := orchestration.TransitionStage(&exec.run.Stages[index], model.StageMerged, s.config.Now()); err != nil {
		exec.mu.Unlock()
		return err
	}
	if mutate != nil {
		mutate(&exec.run.Stages[index])
	}
	exec.run.IntegrationHead = sha
	event := s.queueStageTransition(&exec.run, exec.run.Stages[index], from)
	run := cloneRun(exec.run)
	exec.mu.Unlock()
	err := s.persistPendingEvent(ctx, &run, event)
	exec.mu.Lock()
	syncPersistedEventState(&exec.run, run)
	exec.mu.Unlock()
	if err != nil {
		return fmt.Errorf("persist merged stage transition: %w", err)
	}
	select {
	case exec.wake <- struct{}{}:
	default:
	}
	return nil
}

func (s *Scheduler) cleanupMergedWorkspace(ctx context.Context, runID, stageID string, workspace orchestration.Workspace) {
	if err := s.config.Isolation.Cleanup(context.WithoutCancel(ctx), workspace); err != nil {
		_ = s.config.Store.AppendEvent(context.WithoutCancel(ctx), model.Event{
			RunID: runID, StageID: stageID, Type: "cleanup.deferred", Message: err.Error(),
		})
	}
}

func (s *Scheduler) persistWorkerStart(exec *execution, stageID string, started orchestration.AgentExecution) error {
	ctx := context.Background()
	exec.persist.Lock()
	defer exec.persist.Unlock()
	exec.mu.Lock()
	index := stageIndex(exec.run.Stages, stageID)
	if index < 0 {
		exec.mu.Unlock()
		return fmt.Errorf("unknown stage %q", stageID)
	}
	exec.run.Stages[index].ExecutionID = started.ExecutionID
	exec.run.Stages[index].WorkerPID = started.PID
	exec.run.Stages[index].LastActivity = s.config.Now().UTC()
	stage := exec.run.Stages[index]
	exec.mu.Unlock()
	return s.config.Store.UpdateStage(ctx, exec.run.ID, stage)
}

func (s *Scheduler) persistWorkerStopped(exec *execution, stageID string, result orchestration.AgentResult) error {
	ctx := context.Background()
	exec.persist.Lock()
	defer exec.persist.Unlock()
	exec.mu.Lock()
	index := stageIndex(exec.run.Stages, stageID)
	if index < 0 {
		exec.mu.Unlock()
		return fmt.Errorf("unknown stage %q", stageID)
	}
	exec.run.Stages[index].ExecutionID = result.ExecutionID
	if result.SessionID != "" {
		exec.run.Stages[index].SessionID = result.SessionID
	}
	exec.run.Stages[index].WorkerPID = 0
	stage := exec.run.Stages[index]
	exec.mu.Unlock()
	return s.config.Store.UpdateStage(ctx, exec.run.ID, stage)
}

func (s *Scheduler) persistIntegrationHead(ctx context.Context, exec *execution, stageID, sha string) workerResult {
	exec.persist.Lock()
	defer exec.persist.Unlock()
	if store, ok := s.config.Store.(orchestration.AtomicRunStateStore); ok {
		_, err := store.MutateRun(context.WithoutCancel(ctx), exec.run.ID, func(run *model.RunState) error {
			run.IntegrationHead = sha
			return nil
		})
		if err == nil {
			exec.mu.Lock()
			exec.run.IntegrationHead = sha
			exec.mu.Unlock()
		}
		return workerResult{stageID: stageID, failed: err != nil, err: err}
	}
	exec.mu.Lock()
	exec.run.IntegrationHead = sha
	runSnapshot := cloneRun(exec.run)
	exec.mu.Unlock()
	err := s.config.Store.UpdateRun(context.WithoutCancel(ctx), runSnapshot)
	return workerResult{stageID: stageID, failed: err != nil, err: err}
}

func stageAndState(exec *execution, id string) (model.Stage, model.StageState, bool) {
	exec.mu.Lock()
	defer exec.mu.Unlock()
	var spec model.Stage
	foundSpec := false
	for _, item := range exec.run.Plan.Spec.Stages {
		if item.ID == id {
			spec, foundSpec = item, true
			break
		}
	}
	for _, item := range exec.run.Stages {
		if item.ID == id {
			return spec, item, foundSpec
		}
	}
	return model.Stage{}, model.StageState{}, false
}

func (s *Scheduler) failAttempt(ctx context.Context, exec *execution, id, phase string, err error) workerResult {
	if ctx.Err() != nil {
		return s.cancelStage(exec, id, ctx.Err().Error())
	}
	failure := model.StageFailure{Class: phase, Message: err.Error(), Retryable: phase == "runtime" || phase == "inspect" || phase == "integration"}
	if s.config.Classifier != nil {
		failure = s.config.Classifier.Classify(ctx, orchestration.FailureContext{Phase: phase, Err: err})
	}
	return s.failClassified(ctx, exec, id, failure)
}

func (s *Scheduler) failClassified(ctx context.Context, exec *execution, id string, failure model.StageFailure) workerResult {
	_, state, _ := stageAndState(exec, id)
	max := exec.run.Plan.Spec.Policy.MaxAttemptsPerStage
	retry := failure.Retryable && state.Attempt < max
	delay := s.config.Jitter(s.retryDelay(state.Attempt))
	eligible := s.config.Now().Add(delay).UTC()
	if s.config.Policy != nil {
		decision := s.config.Policy.RetryDecision(ctx, failure)
		retry = decision.Retry && state.Attempt < max
		if !decision.EligibleAt.IsZero() {
			eligible = decision.EligibleAt.UTC()
			delay = eligible.Sub(s.config.Now())
			if delay < 0 {
				delay = 0
			}
		}
	}
	if retry {
		if err := s.transition(exec, id, model.StageRetryScheduled, func(st *model.StageState) { st.Failure = &failure; st.RetryEligibleAt = eligible }); err != nil {
			return workerResult{stageID: id, failed: true, err: err}
		}
		return workerResult{stageID: id}
	}
	if err := s.transition(exec, id, model.StageFailed, func(st *model.StageState) { st.Failure = &failure }); err != nil {
		return workerResult{stageID: id, failed: true, err: err}
	}
	return workerResult{stageID: id, failed: true}
}

func (s *Scheduler) refreshRetries(exec *execution) error {
	exec.mu.Lock()
	var eligible []string
	now := s.config.Now()
	for _, stage := range exec.run.Stages {
		if stage.Status == model.StageRetryScheduled && !stage.RetryEligibleAt.After(now) {
			eligible = append(eligible, stage.ID)
		}
	}
	exec.mu.Unlock()
	for _, id := range eligible {
		if err := s.transition(exec, id, model.StageReady, nil); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) retryDelay(attempt int) time.Duration {
	delay := s.config.Retry.BaseDelay
	for i := 1; i < attempt && delay < s.config.Retry.MaxDelay; i++ {
		if delay > s.config.Retry.MaxDelay/2 {
			return s.config.Retry.MaxDelay
		}
		delay *= 2
	}
	if delay > s.config.Retry.MaxDelay {
		return s.config.Retry.MaxDelay
	}
	return delay
}

func (s *Scheduler) cancelStage(exec *execution, id, reason string) workerResult {
	if err := s.transition(exec, id, model.StageCancelled, func(st *model.StageState) {
		st.Failure = &model.StageFailure{Class: "cancelled", Message: reason, Retryable: true}
	}); err != nil {
		return workerResult{stageID: id, failed: true, err: err}
	}
	return workerResult{stageID: id, failed: true}
}

func withOptionalTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return context.WithCancel(ctx)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type storeSink struct{ store orchestration.StateStore }

func (s storeSink) Emit(ctx context.Context, event model.Event) error {
	return s.store.AppendEvent(ctx, event)
}

type activitySink struct {
	s       *Scheduler
	exec    *execution
	stageID string
}

func (sink activitySink) Emit(ctx context.Context, event model.Event) error {
	ctx = context.WithoutCancel(ctx)
	if err := sink.s.config.Store.AppendEvent(ctx, event); err != nil {
		return err
	}
	sink.exec.persist.Lock()
	defer sink.exec.persist.Unlock()
	sink.exec.mu.Lock()
	index := stageIndex(sink.exec.run.Stages, sink.stageID)
	if index < 0 {
		sink.exec.mu.Unlock()
		return fmt.Errorf("unknown stage %q", sink.stageID)
	}
	sink.exec.run.Stages[index].LastActivity = sink.s.config.Now().UTC()
	if sessionID, _ := event.Data["sessionId"].(string); sessionID != "" {
		sink.exec.run.Stages[index].SessionID = sessionID
	}
	stage := sink.exec.run.Stages[index]
	sink.exec.mu.Unlock()
	return sink.s.config.Store.UpdateStage(ctx, sink.exec.run.ID, stage)
}

func (s *Scheduler) register(exec *execution) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.active[exec.run.ID]; exists {
		return fmt.Errorf("run %q is already active", exec.run.ID)
	}
	s.active[exec.run.ID] = exec
	return nil
}
func (s *Scheduler) unregister(id string, exec *execution) {
	s.mu.Lock()
	if s.active[id] == exec {
		delete(s.active, id)
	}
	s.mu.Unlock()
}

func (s *Scheduler) integrationHead(exec *execution) string {
	exec.mu.Lock()
	defer exec.mu.Unlock()
	if exec.run.IntegrationHead != "" {
		return exec.run.IntegrationHead
	}
	return exec.run.BaseCommit
}

func (s *Scheduler) result(exec *execution) Result {
	exec.mu.Lock()
	defer exec.mu.Unlock()
	return Result{Run: cloneRun(exec.run)}
}

func cloneRun(run model.RunState) model.RunState {
	run.Stages = append([]model.StageState(nil), run.Stages...)
	run.PendingEvents = append([]model.Event(nil), run.PendingEvents...)
	return run
}

func syncPersistedEventState(target *model.RunState, persisted model.RunState) {
	target.PendingEvents = append([]model.Event(nil), persisted.PendingEvents...)
	if persisted.NextEventID > target.NextEventID {
		target.NextEventID = persisted.NextEventID
	}
	if persisted.Cancellation != nil {
		copy := *persisted.Cancellation
		target.Cancellation = &copy
	}
}

func removePendingEvent(events []model.Event, id string) []model.Event {
	for index := range events {
		if events[index].ID == id {
			return append(events[:index:index], events[index+1:]...)
		}
	}
	return events
}

func (s *Scheduler) queueEvent(run *model.RunState, event model.Event) model.Event {
	run.NextEventID++
	event.Time = s.config.Now().UTC()
	event.ID = fmt.Sprintf("run-event:%d:%d", run.NextEventID, event.Time.UnixNano())
	event.RunID = run.ID
	run.PendingEvents = append(run.PendingEvents, event)
	return event
}

func (s *Scheduler) queueStageTransition(run *model.RunState, stage model.StageState, from model.StageStatus) model.Event {
	return s.queueEvent(run, model.Event{
		StageID: stage.ID,
		Type:    "stage.transition",
		Message: string(from) + " -> " + string(stage.Status),
		Data:    map[string]any{"from": from, "to": stage.Status, "attempt": stage.Attempt},
	})
}

func (s *Scheduler) persistPendingEvent(ctx context.Context, run *model.RunState, event model.Event) error {
	ctx = context.WithoutCancel(ctx)
	if store, ok := s.config.Store.(orchestration.AtomicRunStateStore); ok {
		desired := cloneRun(*run)
		updated, err := store.MutateRun(ctx, run.ID, func(current *model.RunState) error {
			if err := applyEventMutation(current, desired, event); err != nil {
				return err
			}
			current.NextEventID++
			event.ID = fmt.Sprintf("run-event:%d:%d", current.NextEventID, event.Time.UnixNano())
			current.PendingEvents = append(current.PendingEvents, event)
			return nil
		})
		if err != nil {
			return err
		}
		*run = updated
	} else if err := s.config.Store.UpdateRun(ctx, *run); err != nil {
		return err
	}
	if err := s.config.Store.AppendEvent(ctx, event); err != nil {
		return fmt.Errorf("append pending event %q: %w", event.ID, err)
	}
	if store, ok := s.config.Store.(orchestration.AtomicRunStateStore); ok {
		current, err := store.MutateRun(ctx, run.ID, func(current *model.RunState) error {
			current.PendingEvents = removePendingEvent(current.PendingEvents, event.ID)
			return nil
		})
		if err != nil {
			return fmt.Errorf("clear pending event %q: %w", event.ID, err)
		}
		*run = current
		return nil
	}
	current, err := s.config.Store.LoadRun(ctx, run.ID)
	if err != nil {
		return fmt.Errorf("reload run before clearing pending event %q: %w", event.ID, err)
	}
	current.PendingEvents = removePendingEvent(current.PendingEvents, event.ID)
	if err := s.config.Store.UpdateRun(ctx, current); err != nil {
		return fmt.Errorf("clear pending event %q: %w", event.ID, err)
	}
	*run = current
	return nil
}

func applyEventMutation(current *model.RunState, desired model.RunState, event model.Event) error {
	switch event.Type {
	case "stage.transition":
		currentIndex := stageIndex(current.Stages, event.StageID)
		desiredIndex := stageIndex(desired.Stages, event.StageID)
		if currentIndex < 0 || desiredIndex < 0 || string(current.Stages[currentIndex].Status) != fmt.Sprint(event.Data["from"]) {
			return fmt.Errorf("stage %s changed concurrently while persisting %s", event.StageID, event.Message)
		}
		if current.Stages[currentIndex].Held != desired.Stages[desiredIndex].Held || current.Stages[currentIndex].HoldReason != desired.Stages[desiredIndex].HoldReason {
			return fmt.Errorf("stage %s hold changed concurrently", event.StageID)
		}
		current.Stages[currentIndex] = desired.Stages[desiredIndex]
		current.UpdatedAt = desired.UpdatedAt
	case "run.transition":
		if string(current.Status) != fmt.Sprint(event.Data["from"]) {
			return fmt.Errorf("run changed concurrently while persisting %s", event.Message)
		}
		current.Status = desired.Status
		current.UpdatedAt = desired.UpdatedAt
		current.Failure = desired.Failure
		from := fmt.Sprint(event.Data["from"])
		if desired.Cancellation != nil || from == string(model.RunFailed) || from == string(model.RunCancelled) {
			current.Cancellation = desired.Cancellation
		}
	case "stage.hold.changed":
		if desiredIndex := stageIndex(desired.Stages, event.StageID); desiredIndex >= 0 {
			if currentIndex := stageIndex(current.Stages, event.StageID); currentIndex >= 0 {
				current.Stages[currentIndex].Held = desired.Stages[desiredIndex].Held
				current.Stages[currentIndex].HoldReason = desired.Stages[desiredIndex].HoldReason
			}
		}
	case "run.cancellation.requested":
		current.Cancellation = desired.Cancellation
	}
	return nil
}

// persistLockedExecutionEvent persists an already-applied in-memory mutation.
// The caller holds exec.persist, preventing another snapshot from overtaking it.
func (s *Scheduler) persistLockedExecutionEvent(ctx context.Context, exec *execution, event model.Event) error {
	exec.mu.Lock()
	event = s.queueEvent(&exec.run, event)
	run := cloneRun(exec.run)
	exec.mu.Unlock()
	err := s.persistPendingEvent(ctx, &run, event)
	exec.mu.Lock()
	syncPersistedEventState(&exec.run, run)
	exec.mu.Unlock()
	return err
}

func (s *Scheduler) replayPendingEvents(ctx context.Context, run *model.RunState) error {
	if len(run.PendingEvents) == 0 {
		return nil
	}
	pending := append([]model.Event(nil), run.PendingEvents...)
	for _, event := range pending {
		if err := s.config.Store.AppendEvent(context.WithoutCancel(ctx), event); err != nil {
			return fmt.Errorf("replay pending event %q: %w", event.ID, err)
		}
	}
	var current model.RunState
	var err error
	if store, ok := s.config.Store.(orchestration.AtomicRunStateStore); ok {
		current, err = store.MutateRun(context.WithoutCancel(ctx), run.ID, func(current *model.RunState) error {
			for _, event := range pending {
				current.PendingEvents = removePendingEvent(current.PendingEvents, event.ID)
			}
			return nil
		})
	} else {
		current, err = s.config.Store.LoadRun(context.WithoutCancel(ctx), run.ID)
		if err == nil {
			for _, event := range pending {
				current.PendingEvents = removePendingEvent(current.PendingEvents, event.ID)
			}
			err = s.config.Store.UpdateRun(context.WithoutCancel(ctx), current)
		}
	}
	if err != nil {
		return fmt.Errorf("clear replayed pending events: %w", err)
	}
	*run = current
	for _, event := range pending {
		if event.Type == "stage.transition" && fmt.Sprint(event.Data["to"]) == string(model.StageMerged) {
			if index := stageIndex(run.Stages, event.StageID); index >= 0 && run.Stages[index].Workspace != "" {
				s.cleanupMergedWorkspace(ctx, run.ID, event.StageID, orchestration.Workspace{Path: run.Stages[index].Workspace, Branch: run.Stages[index].Branch})
			}
		}
	}
	return nil
}

func cancellationReason(exec *execution, parent context.Context) string {
	exec.mu.Lock()
	defer exec.mu.Unlock()
	if exec.run.Cancellation != nil && exec.run.Cancellation.Reason != "" {
		return exec.run.Cancellation.Reason
	}
	if parent.Err() != nil {
		return parent.Err().Error()
	}
	return "cancelled"
}

func (s *Scheduler) cancelRequested(exec *execution) bool {
	exec.mu.Lock()
	defer exec.mu.Unlock()
	return exec.run.Cancellation != nil
}

func (s *Scheduler) readyIDs(ctx context.Context, exec *execution) []string {
	exec.mu.Lock()
	defer exec.mu.Unlock()
	var ready []model.StageState
	for _, stage := range exec.run.Stages {
		if (stage.Status == model.StageReady || stage.Status == model.StageCommitReady || stage.Status == model.StagePostMergeVerifying) && !stage.Held {
			ready = append(ready, stage)
		}
	}
	allowed := make(map[string]bool, len(ready))
	for _, stage := range ready {
		allowed[stage.ID] = true
	}
	if s.config.Policy != nil {
		ready = s.config.Policy.OrderReadyStages(ctx, append([]model.StageState(nil), ready...))
	}
	ids := make([]string, 0, len(ready))
	seen := make(map[string]bool, len(ready))
	for _, stage := range ready {
		if allowed[stage.ID] && !seen[stage.ID] {
			ids = append(ids, stage.ID)
			seen[stage.ID] = true
		}
	}
	if s.config.Policy == nil {
		sort.Strings(ids)
	} else if len(ids) < len(allowed) {
		var omitted []string
		for id := range allowed {
			if !seen[id] {
				omitted = append(omitted, id)
			}
		}
		sort.Strings(omitted)
		ids = append(ids, omitted...)
	}
	return ids
}

func finalStatus(exec *execution) (model.RunStatus, bool) {
	exec.mu.Lock()
	defer exec.mu.Unlock()
	allMerged, allTerminal := true, true
	for _, stage := range exec.run.Stages {
		allMerged = allMerged && stage.Status == model.StageMerged
		allTerminal = allTerminal && orchestration.IsTerminalStage(stage.Status)
	}
	if allMerged {
		return model.RunReadyToIntegrate, true
	}
	if exec.run.Cancellation != nil && allTerminal {
		return model.RunCancelled, true
	}
	if allTerminal {
		return model.RunFailed, true
	}
	return "", false
}

func (s *Scheduler) finalVerification(ctx context.Context, exec *execution) error {
	exec.mu.Lock()
	run := cloneRun(exec.run)
	exec.mu.Unlock()
	if !run.Plan.Spec.Policy.FinalVerificationRequired {
		return nil
	}
	for _, stage := range run.Plan.Spec.Stages {
		criteria := make([]model.AcceptanceCriterion, 0, len(stage.Acceptance))
		for _, criterion := range stage.Acceptance {
			if criterion.Type != "git-diff-not-empty" {
				criteria = append(criteria, criterion)
			}
		}
		result := s.config.Verifier.Verify(ctx, orchestration.VerificationRequest{
			RunID: run.ID, StageID: stage.ID, Workspace: run.IntegrationWorkspace,
			Criteria: criteria, WriteScope: stage.WriteScope,
		}, storeSink{s.config.Store})
		if !result.Passed {
			if result.Failure != nil {
				return fmt.Errorf("stage %s: %s", stage.ID, result.Failure.Message)
			}
			return fmt.Errorf("stage %s failed final verification", stage.ID)
		}
	}
	return nil
}

func (s *Scheduler) refreshDependencies(exec *execution) error {
	exec.mu.Lock()
	states := make(map[string]model.StageStatus, len(exec.run.Stages))
	for _, stage := range exec.run.Stages {
		states[stage.ID] = stage.Status
	}
	var changes []struct {
		id string
		to model.StageStatus
	}
	for _, stage := range exec.run.Plan.Spec.Stages {
		if states[stage.ID] != model.StageWaitingForDependencies {
			continue
		}
		ready, blocked := true, false
		for _, dependency := range stage.DependsOn {
			status := states[dependency]
			if status != model.StageMerged {
				ready = false
			}
			if orchestration.IsTerminalStage(status) && status != model.StageMerged {
				blocked = true
			}
		}
		if blocked {
			changes = append(changes, struct {
				id string
				to model.StageStatus
			}{stage.ID, model.StageBlocked})
		} else if ready {
			changes = append(changes, struct {
				id string
				to model.StageStatus
			}{stage.ID, model.StageReady})
		}
	}
	exec.mu.Unlock()
	for _, change := range changes {
		if err := s.transition(exec, change.id, change.to, nil); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) transition(exec *execution, id string, to model.StageStatus, mutate func(*model.StageState)) error {
	exec.persist.Lock()
	defer exec.persist.Unlock()
	exec.mu.Lock()
	index := -1
	for i := range exec.run.Stages {
		if exec.run.Stages[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		exec.mu.Unlock()
		return fmt.Errorf("unknown stage %q", id)
	}
	from := exec.run.Stages[index].Status
	if err := orchestration.TransitionStage(&exec.run.Stages[index], to, s.config.Now()); err != nil {
		exec.mu.Unlock()
		return err
	}
	if mutate != nil {
		mutate(&exec.run.Stages[index])
	}
	exec.run.UpdatedAt = exec.run.Stages[index].LastActivity
	event := s.queueStageTransition(&exec.run, exec.run.Stages[index], from)
	run := cloneRun(exec.run)
	exec.mu.Unlock()
	err := s.persistPendingEvent(context.Background(), &run, event)
	exec.mu.Lock()
	syncPersistedEventState(&exec.run, run)
	exec.mu.Unlock()
	return err
}

func (s *Scheduler) persistStoredStageTransition(ctx context.Context, run *model.RunState, index int, from model.StageStatus) error {
	event := s.queueStageTransition(run, run.Stages[index], from)
	return s.persistPendingEvent(ctx, run, event)
}

func (s *Scheduler) persistRunTransition(ctx context.Context, run *model.RunState, to model.RunStatus) error {
	from := run.Status
	if err := orchestration.TransitionRun(run, to, s.config.Now()); err != nil {
		return err
	}
	event := s.queueEvent(run, model.Event{Type: "run.transition", Message: string(from) + " -> " + string(to), Data: map[string]any{"from": from, "to": to}})
	return s.persistPendingEvent(ctx, run, event)
}

func (s *Scheduler) persistExecutionRunTransition(ctx context.Context, exec *execution, to model.RunStatus) error {
	exec.persist.Lock()
	defer exec.persist.Unlock()
	exec.mu.Lock()
	from := exec.run.Status
	if err := orchestration.TransitionRun(&exec.run, to, s.config.Now()); err != nil {
		exec.mu.Unlock()
		return err
	}
	run := cloneRun(exec.run)
	event := s.queueEvent(&run, model.Event{Type: "run.transition", Message: string(from) + " -> " + string(to), Data: map[string]any{"from": from, "to": to}})
	exec.run.NextEventID = run.NextEventID
	syncPersistedEventState(&exec.run, run)
	exec.mu.Unlock()
	err := s.persistPendingEvent(ctx, &run, event)
	exec.mu.Lock()
	syncPersistedEventState(&exec.run, run)
	exec.mu.Unlock()
	return err
}

func (s *Scheduler) cancelUnstarted(exec *execution, reason string) error {
	for _, status := range []model.StageStatus{model.StageWaitingForDependencies, model.StageReady, model.StageRetryScheduled} {
		exec.mu.Lock()
		var ids []string
		for _, stage := range exec.run.Stages {
			if stage.Status == status {
				ids = append(ids, stage.ID)
			}
		}
		exec.mu.Unlock()
		for _, id := range ids {
			if err := s.transition(exec, id, model.StageCancelled, func(st *model.StageState) {
				st.Failure = &model.StageFailure{Class: "cancelled", Message: reason, Retryable: true}
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Scheduler) requestCancellation(exec *execution, reason string) error {
	exec.persist.Lock()
	defer exec.persist.Unlock()
	exec.mu.Lock()
	if exec.run.Cancellation != nil {
		exec.mu.Unlock()
		return nil
	}
	exec.run.Cancellation = &model.Cancellation{RequestedAt: s.config.Now().UTC(), Reason: reason}
	exec.mu.Unlock()
	return s.persistLockedExecutionEvent(context.Background(), exec, model.Event{
		Type: "run.cancellation.requested", Message: reason,
	})
}

// StagePrompt is versioned and deterministic so retries and resumes receive the
// same behavioral contract.
const StagePromptVersion = "north.stage/v1"

func StagePrompt(plan model.ExecutionPlan, stage model.Stage) string {
	var acceptance []string
	for _, criterion := range stage.Acceptance {
		acceptance = append(acceptance, criterion.ID+" ("+criterion.Type+")")
	}
	return strings.Join([]string{
		StagePromptVersion,
		"Run plan: " + plan.Metadata.Name,
		"Goal: " + plan.Spec.Goal,
		"Stage: " + stage.ID + " - " + stage.Title,
		"Task: " + stage.Description,
		"Dependencies already integrated: " + strings.Join(stage.DependsOn, ", "),
		"Write scope: " + strings.Join(stage.WriteScope, ", "),
		"Acceptance criteria: " + strings.Join(acceptance, ", "),
		"Change only files in the write scope. Do not commit, merge, rebase, push, create worktrees, or modify repository history.",
		"Do not modify North state, invoke North or /loop recursively, or weaken tests or acceptance checks.",
		"Complete the task and leave all intended changes in the workspace for host verification.",
		"Final report: summarize changed paths, checks run, evidence, and blockers. Host verification decides completion.",
	}, "\n") + "\n"
}
