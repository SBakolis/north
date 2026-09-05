// Package application composes North core contracts with production adapters.
package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SBakolis/north/internal/conflict"
	gitadapter "github.com/SBakolis/north/internal/git"
	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/opencode"
	"github.com/SBakolis/north/internal/orchestration"
	"github.com/SBakolis/north/internal/orchestration/scheduler"
	"github.com/SBakolis/north/internal/plan"
	"github.com/SBakolis/north/internal/platform"
	"github.com/SBakolis/north/internal/state"
	"github.com/SBakolis/north/internal/verification"
)

type Manager struct {
	Paths          platform.Paths
	OpenCodeBinary string
	OpenCodeEnv    map[string]string
	StageTimeout   time.Duration
	AllowShell     bool
	AgentResolver  plan.AgentResolver
}

type RunOptions struct {
	MaxParallel int
	FailFast    bool
	SetFailFast bool
}

func (m Manager) Start(ctx context.Context, start string, executionPlan model.ExecutionPlan, options RunOptions) (model.RunState, error) {
	if options.MaxParallel > 0 {
		executionPlan.Spec.Policy.MaxParallel = options.MaxParallel
	}
	if options.SetFailFast {
		executionPlan.Spec.Policy.FailFast = options.FailFast
	}
	repo, projectID, store, err := m.openProject(ctx, start)
	if err != nil {
		return model.RunState{}, err
	}
	agentResolver := m.AgentResolver
	if agentResolver == nil {
		agentResolver = installedAgentResolver{root: m.Paths.OpenCodeDir}
	}
	if _, err := plan.ValidateWithOptions(ctx, executionPlan, plan.ValidationOptions{BaseRefResolver: repo, AgentResolver: agentResolver}); err != nil {
		return model.RunState{}, err
	}
	if err := repo.Clean(ctx); err != nil {
		return model.RunState{}, err
	}
	base, err := repo.ResolveBase(ctx, executionPlan.Spec.BaseRef)
	if err != nil {
		return model.RunState{}, fmt.Errorf("resolve immutable base: %w", err)
	}
	target, err := currentBranch(ctx, repo)
	if err != nil {
		return model.RunState{}, err
	}
	runID, err := newRunID()
	if err != nil {
		return model.RunState{}, err
	}
	repositoryLock, err := acquireRepositoryLock(ctx, store, projectID)
	if err != nil {
		return model.RunState{}, err
	}
	defer repositoryLock.Release()

	runtime := m.agentRuntime(runID)
	if err := runtime.Validate(ctx); err != nil {
		return model.RunState{}, err
	}
	adapter := gitadapter.NewAdapter(repo)
	integration, err := adapter.PrepareIntegration(ctx, runID, base)
	if err != nil {
		return model.RunState{}, fmt.Errorf("prepare integration worktree: %w", err)
	}
	handedOff := false
	defer func() {
		if !handedOff {
			_ = adapter.Cleanup(context.WithoutCancel(ctx), integration)
			_ = adapter.DeleteManagedBranch(context.WithoutCancel(ctx), integration.Branch)
		}
	}()
	hash, err := plan.ApprovalHash(executionPlan)
	if err != nil {
		return model.RunState{}, err
	}
	run := model.RunState{
		SchemaVersion: 1, ID: runID, ProjectID: projectID, PlanHash: hash,
		BaseCommit: base, RepositoryRoot: repo.Root, IntegrationBranch: integration.Branch,
		IntegrationWorkspace: integration.Path, IntegrationHead: base, TargetBranch: target,
	}
	schedulerInstance, err := m.scheduler(repo, store, runtime)
	if err != nil {
		return run, err
	}
	result, err := schedulerInstance.Start(ctx, run, executionPlan)
	handedOff = result.Run.ID != ""
	return result.Run, err
}

type installedAgentResolver struct{ root string }

func (r installedAgentResolver) ResolveAgent(_ context.Context, agent string) error {
	if err := plan.ValidateAgentReference(agent); err != nil {
		return err
	}
	path := filepath.Join(r.root, "agents", agent+".md")
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("agent %q is unavailable at %s: %w", agent, path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("agent %q is unavailable at non-file path %s", agent, path)
	}
	return nil
}

func (m Manager) Resume(ctx context.Context, start, runID string) (model.RunState, error) {
	repo, projectID, store, err := m.openProject(ctx, start)
	if err != nil {
		return model.RunState{}, err
	}
	lock, err := acquireRepositoryLock(ctx, store, projectID)
	if err != nil {
		return model.RunState{}, err
	}
	defer lock.Release()
	if metadata, inspectErr := store.InspectRunLock(runID); inspectErr == nil && !state.OwnerAlive(metadata) {
		if err := store.ReleaseRunLock(runID, metadata.Token); err != nil {
			return model.RunState{}, fmt.Errorf("release orphaned run lock: %w", err)
		}
	} else if inspectErr != nil && !errors.Is(inspectErr, os.ErrNotExist) {
		return model.RunState{}, inspectErr
	}
	runtime := m.agentRuntime(runID)
	schedulerInstance, err := m.scheduler(repo, store, runtime)
	if err != nil {
		return model.RunState{}, err
	}
	result, err := schedulerInstance.Resume(ctx, runID)
	return result.Run, err
}

func (m Manager) Integrate(ctx context.Context, start, runID, target string) (model.RunState, error) {
	repo, projectID, store, err := m.openProject(ctx, start)
	if err != nil {
		return model.RunState{}, err
	}
	lock, err := acquireRepositoryLock(ctx, store, projectID)
	if err != nil {
		return model.RunState{}, err
	}
	defer lock.Release()
	run, err := store.LoadRun(ctx, runID)
	if err != nil {
		return model.RunState{}, err
	}
	if run.Status != model.RunReadyToIntegrate {
		return run, fmt.Errorf("run %s is %s, not ReadyToIntegrate", runID, run.Status)
	}
	if target == "" {
		target = run.TargetBranch
	}
	result := gitadapter.NewAdapter(repo).IntegrateRun(ctx, orchestration.RunIntegrationRequest{RunID: runID, TargetBranch: target})
	if result.Err != nil {
		return run, result.Err
	}
	if err := orchestration.TransitionRun(&run, model.RunCompleted, time.Now()); err != nil {
		return run, err
	}
	run.IntegrationHead = result.CommitSHA
	if err := store.UpdateRun(ctx, run); err != nil {
		return run, err
	}
	if err := store.AppendEvent(ctx, model.Event{SchemaVersion: 1, Time: time.Now().UTC(), RunID: runID, Type: "run.integrated", Message: "integrated into " + target, Data: map[string]any{"commit": result.CommitSHA}}); err != nil {
		return run, err
	}
	return run, nil
}

func (m Manager) Stop(ctx context.Context, start, runID, reason string) error {
	_, _, store, err := m.openProject(ctx, start)
	if err != nil {
		return err
	}
	run, err := store.LoadRun(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status != model.RunRunning && run.Status != model.RunPreparing {
		return fmt.Errorf("run %s is not active", runID)
	}
	if reason == "" {
		reason = "operator requested stop"
	}
	run.Cancellation = &model.Cancellation{RequestedAt: time.Now().UTC(), Reason: reason}
	run.UpdatedAt = time.Now().UTC()
	if err := store.UpdateRun(ctx, run); err != nil {
		return err
	}
	return store.AppendEvent(ctx, model.Event{SchemaVersion: 1, Time: time.Now().UTC(), RunID: runID, Type: "run.cancellation.requested", Message: reason})
}

func (m Manager) Load(ctx context.Context, start, runID string) (model.RunState, error) {
	_, _, store, err := m.openProject(ctx, start)
	if err != nil {
		return model.RunState{}, err
	}
	return store.LoadRun(ctx, runID)
}

func (m Manager) List(ctx context.Context, start string) ([]model.RunSummary, error) {
	_, projectID, store, err := m.openProject(ctx, start)
	if err != nil {
		return nil, err
	}
	return store.ListRuns(ctx, projectID)
}

func (m Manager) SetHold(ctx context.Context, start, runID, stageID, reason string, held bool) error {
	_, _, store, err := m.openProject(ctx, start)
	if err != nil {
		return err
	}
	run, err := store.LoadRun(ctx, runID)
	if err != nil {
		return err
	}
	index := stageIndex(run.Stages, stageID)
	if index < 0 {
		return fmt.Errorf("unknown stage %q", stageID)
	}
	if err := orchestration.SetStageHold(&run.Stages[index], held, reason); err != nil {
		return err
	}
	return store.UpdateStage(ctx, runID, run.Stages[index])
}

func (m Manager) Retry(ctx context.Context, start, runID, stageID string) error {
	_, _, store, err := m.openProject(ctx, start)
	if err != nil {
		return err
	}
	run, err := store.LoadRun(ctx, runID)
	if err != nil {
		return err
	}
	index := stageIndex(run.Stages, stageID)
	if index < 0 {
		return fmt.Errorf("unknown stage %q", stageID)
	}
	from := run.Stages[index].Status
	if err := orchestration.TransitionStage(&run.Stages[index], model.StageReady, time.Now()); err != nil {
		return err
	}
	run.Stages[index].Failure = nil
	run.Stages[index].ConflictingPaths = ""
	if err := store.UpdateStage(ctx, runID, run.Stages[index]); err != nil {
		return err
	}
	for i := range run.Stages {
		if run.Stages[i].Status == model.StageBlocked {
			if err := orchestration.TransitionStage(&run.Stages[i], model.StageWaitingForDependencies, time.Now()); err != nil {
				return err
			}
			if err := store.UpdateStage(ctx, runID, run.Stages[i]); err != nil {
				return err
			}
		}
	}
	if run.Status == model.RunFailed || run.Status == model.RunCancelled {
		run.Cancellation, run.Failure = nil, nil
		if err := orchestration.TransitionRun(&run, model.RunRunning, time.Now()); err != nil {
			return err
		}
		if err := store.UpdateRun(ctx, run); err != nil {
			return err
		}
	}
	return store.AppendEvent(ctx, model.Event{SchemaVersion: 1, Time: time.Now().UTC(), RunID: runID, StageID: stageID, Type: "stage.manual-retry", Message: string(from) + " -> Ready"})
}

func (m Manager) Cleanup(ctx context.Context, start, runID string) error {
	repo, projectID, store, err := m.openProject(ctx, start)
	if err != nil {
		return err
	}
	lock, err := acquireRepositoryLock(ctx, store, projectID)
	if err != nil {
		return err
	}
	defer lock.Release()
	run, err := store.LoadRun(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status == model.RunRunning || run.Status == model.RunPreparing || run.Status == model.RunReadyToIntegrate {
		return fmt.Errorf("refuse cleanup of run %s in status %s", runID, run.Status)
	}
	adapter := gitadapter.NewAdapter(repo)
	for _, stage := range run.Stages {
		if stage.Workspace == "" {
			continue
		}
		if _, err := os.Lstat(stage.Workspace); errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err := adapter.Cleanup(ctx, orchestration.Workspace{Path: stage.Workspace, Branch: stage.Branch}); err != nil {
			return err
		}
	}
	if run.IntegrationWorkspace != "" {
		if _, err := os.Lstat(run.IntegrationWorkspace); err == nil {
			if err := adapter.Cleanup(ctx, orchestration.Workspace{Path: run.IntegrationWorkspace, Branch: run.IntegrationBranch}); err != nil {
				return err
			}
		}
	}
	for _, stage := range run.Stages {
		expected := gitadapter.SanitizeBranch("north", run.ID, "stage", stage.ID)
		if stage.Branch != "" && stage.Branch != expected {
			return fmt.Errorf("refuse cleanup of unexpected stage branch %q", stage.Branch)
		}
		if stage.Branch != "" {
			if err := adapter.DeleteManagedBranch(ctx, stage.Branch); err != nil {
				return err
			}
		}
	}
	expectedIntegration := gitadapter.SanitizeBranch("north", run.ID, "integration")
	if run.IntegrationBranch != "" && run.IntegrationBranch != expectedIntegration {
		return fmt.Errorf("refuse cleanup of unexpected integration branch %q", run.IntegrationBranch)
	}
	if run.IntegrationBranch != "" {
		if err := adapter.DeleteManagedBranch(ctx, run.IntegrationBranch); err != nil {
			return err
		}
	}
	return nil
}

func (m Manager) scheduler(repo *gitadapter.Repository, store *state.Store, runtime *opencode.Runtime) (*scheduler.Scheduler, error) {
	adapter := gitadapter.NewAdapter(repo)
	verifier := verification.New(verification.Config{Git: repo, AllowShell: m.AllowShell, LogDir: filepath.Join(m.Paths.StateDir, "logs", "verification")})
	conflicts, err := conflict.New(conflict.Config{Git: adapter, Runtime: runtime, Verifier: verifier, Integration: adapter, Timeout: m.StageTimeout})
	if err != nil {
		return nil, err
	}
	return scheduler.New(scheduler.Config{
		Store: store, Runtime: runtime, Isolation: adapter, Inspector: adapter, Scope: adapter,
		Verifier: verifier, Committer: adapter, Integration: adapter, Conflicts: conflicts, StageTimeout: m.StageTimeout,
		WorkerAlive: state.ProcessTreeAlive,
	})
}

func (m Manager) agentRuntime(runID string) *opencode.Runtime {
	return opencode.New(opencode.Config{
		Binary: m.OpenCodeBinary, LogDir: filepath.Join(m.Paths.StateDir, "logs", runID),
		StateDir: m.Paths.StateDir, Env: m.OpenCodeEnv,
	})
}

func (m Manager) openProject(ctx context.Context, start string) (*gitadapter.Repository, string, *state.Store, error) {
	if start == "" {
		start = "."
	}
	// Resolve the repository first using a temporary external cache root.
	temporaryRoot := filepath.Join(m.Paths.CacheDir, "worktrees", "unresolved")
	repo, err := gitadapter.Open(ctx, start, temporaryRoot, nil)
	if err != nil {
		return nil, "", nil, err
	}
	projectID := ProjectID(repo.Root)
	repo.WorktreeRoot = filepath.Join(m.Paths.CacheDir, "worktrees", projectID)
	store := state.New(filepath.Join(m.Paths.StateDir, "projects", projectID, "runs"))
	return repo, projectID, store, nil
}

func ProjectID(root string) string {
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		canonical = filepath.Clean(root)
	}
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:12])
}

func currentBranch(ctx context.Context, repo *gitadapter.Repository) (string, error) {
	result, err := repo.Runner.Run(ctx, repo.Root, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(result.Stdout)
	if branch == "" {
		return "", errors.New("repository is in detached HEAD state")
	}
	return branch, nil
}

func newRunID() (string, error) {
	var random [6]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return time.Now().UTC().Format("20060102t150405") + "-" + hex.EncodeToString(random[:]), nil
}

func stageIndex(stages []model.StageState, id string) int {
	for index := range stages {
		if stages[index].ID == id {
			return index
		}
	}
	return -1
}

func acquireRepositoryLock(ctx context.Context, store *state.Store, projectID string) (*state.FileLock, error) {
	lock, err := store.AcquireRepositoryLock(ctx, projectID)
	if !errors.Is(err, state.ErrLocked) {
		return lock, err
	}
	metadata, inspectErr := store.InspectRepositoryLock()
	if inspectErr != nil || state.OwnerAlive(metadata) {
		return nil, err
	}
	if releaseErr := store.ReleaseRepositoryLock(metadata.Token); releaseErr != nil {
		return nil, errors.Join(err, releaseErr)
	}
	return store.AcquireRepositoryLock(ctx, projectID)
}
