package application

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/platform"
	"github.com/SBakolis/north/internal/testutil"
)

func TestParallelRunLeavesTargetUntouchedUntilExplicitIntegration(t *testing.T) {
	repository := testutil.GitRepository(t)
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", "README.md")
	runGit(t, repository, "commit", "-m", "base")
	base := runGit(t, repository, "rev-parse", "HEAD")

	binDir := t.TempDir()
	fake := testutil.WriteExecutable(t, binDir, "opencode", `
if [ "$1" = "--version" ]; then printf 'opencode 1.0\n'; exit 0; fi
if [ "$1" = "run" ] && [ "$2" = "--help" ]; then printf '%s\n' '--dir --agent --format --session'; exit 0; fi
sleep 0.05
printf 'stage %s\n' "$NORTH_STAGE_ID" > "$NORTH_WORKTREE/$NORTH_STAGE_ID.txt"
printf '{"type":"session.updated","sessionID":"session-%s"}\n' "$NORTH_STAGE_ID"
`)
	root := t.TempDir()
	paths := platform.Paths{
		ConfigDir: filepath.Join(root, "config"), DataDir: filepath.Join(root, "data"),
		StateDir: filepath.Join(root, "state"), CacheDir: filepath.Join(root, "cache"),
		OpenCodeDir: filepath.Join(root, "opencode"),
	}
	manager := Manager{Paths: paths, OpenCodeBinary: fake, StageTimeout: 5 * time.Second, AgentResolver: allowAgents{}}
	plan := model.ExecutionPlan{
		APIVersion: model.APIVersionV1Alpha1, Kind: model.ExecutionPlanKind,
		Metadata: model.PlanMetadata{Name: "parallel-e2e"},
		Spec: model.PlanSpec{Goal: "create three staged files", BaseRef: "main", Policy: model.PlanPolicy{MaxParallel: 2, MaxAttemptsPerStage: 1, FinalVerificationRequired: true},
			Stages: []model.Stage{
				{ID: "A", Title: "stage A", Description: "create A", Agent: "north-worker", WriteScope: []string{"A.txt"}, Acceptance: []model.AcceptanceCriterion{{ID: "a", Type: "file-exists", Path: "A.txt", Timeout: time.Second}}},
				{ID: "B", Title: "stage B", Description: "create B", Agent: "north-worker", WriteScope: []string{"B.txt"}, Acceptance: []model.AcceptanceCriterion{{ID: "b", Type: "file-exists", Path: "B.txt", Timeout: time.Second}}},
				{ID: "C", Title: "stage C", Description: "create C", DependsOn: []string{"A", "B"}, Agent: "north-worker", WriteScope: []string{"C.txt"}, Acceptance: []model.AcceptanceCriterion{{ID: "c", Type: "file-exists", Path: "C.txt", Timeout: time.Second}}},
			},
		},
	}
	run, err := manager.Start(context.Background(), repository, plan, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != model.RunReadyToIntegrate {
		for _, stage := range run.Stages {
			if stage.Failure != nil {
				t.Logf("stage %s failure: %+v", stage.ID, *stage.Failure)
			}
		}
		t.Fatalf("run status = %s, failure=%+v, stages=%+v", run.Status, run.Failure, run.Stages)
	}
	if got := runGit(t, repository, "rev-parse", "HEAD"); got != base {
		t.Fatalf("target moved before integration: %s != %s", got, base)
	}
	for _, name := range []string{"A.txt", "B.txt", "C.txt"} {
		if _, err := os.Stat(filepath.Join(repository, name)); !os.IsNotExist(err) {
			t.Fatalf("%s appeared in target before integration", name)
		}
	}
	run, err = manager.Integrate(context.Background(), repository, run.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != model.RunCompleted {
		t.Fatalf("integrated status = %s", run.Status)
	}
	for _, name := range []string{"A.txt", "B.txt", "C.txt"} {
		if _, err := os.Stat(filepath.Join(repository, name)); err != nil {
			t.Fatalf("integrated file %s: %v", name, err)
		}
	}
	if err := manager.Cleanup(context.Background(), repository, run.ID); err != nil {
		t.Fatal(err)
	}
	for _, branch := range append([]string{run.IntegrationBranch}, run.Stages[0].Branch, run.Stages[1].Branch, run.Stages[2].Branch) {
		command := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
		command.Dir = repository
		if command.Run() == nil {
			t.Fatalf("cleanup retained branch %s", branch)
		}
	}
}

func TestVerificationFailureRepairsInSameWorkspace(t *testing.T) {
	repository := initializedRepository(t)
	fake := testutil.WriteExecutable(t, t.TempDir(), "opencode", `
if [ "$1" = "--version" ]; then printf 'opencode 1.0\n'; exit 0; fi
if [ "$1" = "run" ] && [ "$2" = "--help" ]; then printf '%s\n' '--dir --agent --format --session'; exit 0; fi
file="$NORTH_WORKTREE/result.txt"
if [ -f "$file" ]; then printf 'repaired\n' > "$file"; else printf 'first attempt\n' > "$file"; fi
printf '{"type":"session.updated","sessionID":"repair-session"}\n'
`)
	manager := Manager{Paths: isolatedPaths(t), OpenCodeBinary: fake, StageTimeout: 5 * time.Second, AgentResolver: allowAgents{}}
	executionPlan := simplePlan("repair", model.AcceptanceCriterion{ID: "content", Type: "contains", Path: "result.txt", Value: "repaired", Timeout: time.Second})
	executionPlan.Spec.Policy.MaxAttemptsPerStage = 2
	run, err := manager.Start(context.Background(), repository, executionPlan, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != model.RunReadyToIntegrate || run.Stages[0].Attempt != 2 || run.Stages[0].SessionID != "repair-session" {
		t.Fatalf("run = %+v", run)
	}
}

func TestPersistedStopCancelsRunningWorker(t *testing.T) {
	repository := initializedRepository(t)
	fake := testutil.WriteExecutable(t, t.TempDir(), "opencode", `
if [ "$1" = "--version" ]; then printf 'opencode 1.0\n'; exit 0; fi
if [ "$1" = "run" ] && [ "$2" = "--help" ]; then printf '%s\n' '--dir --agent --format --session'; exit 0; fi
sleep 30
printf 'late\n' > "$NORTH_WORKTREE/result.txt"
`)
	manager := Manager{Paths: isolatedPaths(t), OpenCodeBinary: fake, StageTimeout: time.Minute, AgentResolver: allowAgents{}}
	done := make(chan model.RunState, 1)
	errs := make(chan error, 1)
	go func() {
		run, err := manager.Start(context.Background(), repository, simplePlan("cancel", model.AcceptanceCriterion{ID: "file", Type: "file-exists", Path: "result.txt", Timeout: time.Second}), RunOptions{})
		if err != nil {
			errs <- err
			return
		}
		done <- run
	}()
	var runID string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runs, err := manager.List(context.Background(), repository)
		if err == nil && len(runs) > 0 {
			runID = runs[0].ID
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if runID == "" {
		t.Fatal("running state was not persisted")
	}
	if err := manager.Stop(context.Background(), repository, runID, "test stop"); err != nil {
		t.Fatal(err)
	}
	select {
	case run := <-done:
		if run.Status != model.RunCancelled || run.Cancellation == nil {
			t.Fatalf("run = %+v", run)
		}
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("worker was not cancelled from persisted intent")
	}
}

func initializedRepository(t *testing.T) string {
	t.Helper()
	repository := testutil.GitRepository(t)
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", "README.md")
	runGit(t, repository, "commit", "-m", "base")
	return repository
}

func isolatedPaths(t *testing.T) platform.Paths {
	t.Helper()
	root := t.TempDir()
	return platform.Paths{ConfigDir: filepath.Join(root, "config"), DataDir: filepath.Join(root, "data"), StateDir: filepath.Join(root, "state"), CacheDir: filepath.Join(root, "cache"), OpenCodeDir: filepath.Join(root, "opencode")}
}

func simplePlan(name string, criterion model.AcceptanceCriterion) model.ExecutionPlan {
	return model.ExecutionPlan{
		APIVersion: model.APIVersionV1Alpha1, Kind: model.ExecutionPlanKind, Metadata: model.PlanMetadata{Name: name},
		Spec: model.PlanSpec{Goal: name, BaseRef: "main", Policy: model.PlanPolicy{MaxParallel: 1, MaxAttemptsPerStage: 1, FinalVerificationRequired: true},
			Stages: []model.Stage{{ID: "stage", Title: name, Description: name, Agent: "north-worker", WriteScope: []string{"result.txt"}, Acceptance: []model.AcceptanceCriterion{criterion}}}},
	}
}

type allowAgents struct{}

func (allowAgents) ResolveAgent(context.Context, string) error { return nil }

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(bytes.TrimSpace(output))
}
