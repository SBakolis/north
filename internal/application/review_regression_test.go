package application

import (
	"context"
	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/testutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const reviewFakeHeader = `
if [ "$1" = "--version" ]; then printf 'opencode 1.0\n'; exit 0; fi
if [ "$1" = "run" ] && [ "$2" = "--help" ]; then printf '%s\n' '--dir --agent --format --session'; exit 0; fi
`

func TestRegressionGeneratedAcceptanceArtifact(t *testing.T) {
	repo := initializedRepository(t)
	fake := testutil.WriteExecutable(t, t.TempDir(), "opencode", reviewFakeHeader+`printf 'worker\n' > "$NORTH_WORKTREE/result.txt"`)
	m := Manager{Paths: isolatedPaths(t), OpenCodeBinary: fake, StageTimeout: 5 * time.Second, AgentResolver: allowAgents{}, AllowShell: true}
	p := simplePlan("generated", model.AcceptanceCriterion{ID: "generate", Type: "shell", Command: []string{"printf 'required artifact\\n' > generated.txt"}, Timeout: time.Second})
	p.Spec.Stages[0].WriteScope = []string{"*.txt"}
	p.Spec.Stages[0].Acceptance = append(p.Spec.Stages[0].Acceptance, model.AcceptanceCriterion{ID: "artifact", Type: "file-exists", Path: "generated.txt", Timeout: time.Second})
	run, err := m.Start(context.Background(), repo, p, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != model.RunReadyToIntegrate {
		t.Fatalf("unexpected status: %+v", run)
	}
	_, err = m.Integrate(context.Background(), repo, run.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(repo, "generated.txt")); err != nil {
		t.Fatalf("completed run is missing an accepted artifact: %v", err)
	}
}

func TestRegressionPersistedHold(t *testing.T) {
	repo := initializedRepository(t)
	fake := testutil.WriteExecutable(t, t.TempDir(), "opencode", reviewFakeHeader+`
if [ "$NORTH_STAGE_ID" = stage ]; then sleep 1; fi
printf 'worker\n' > "$NORTH_WORKTREE/result.txt"
`)
	m := Manager{Paths: isolatedPaths(t), OpenCodeBinary: fake, StageTimeout: 5 * time.Second, AgentResolver: allowAgents{}}
	p := simplePlan("hold", model.AcceptanceCriterion{ID: "exists", Type: "file-exists", Path: "result.txt", Timeout: time.Second})
	second := p.Spec.Stages[0]
	second.ID = "second"
	second.DependsOn = []string{"stage"}
	second.AllowNoChanges = true
	p.Spec.Stages = append(p.Spec.Stages, second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan model.RunState, 1)
	errs := make(chan error, 1)
	go func() {
		r, e := m.Start(ctx, repo, p, RunOptions{})
		if e != nil {
			errs <- e
		} else {
			done <- r
		}
	}()
	var id string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runs, e := m.List(context.Background(), repo)
		if e == nil && len(runs) > 0 {
			r, e := m.Load(context.Background(), repo, runs[0].ID)
			if e == nil && r.Stages[0].Status == model.StageRunning {
				id = r.ID
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if id == "" {
		t.Fatal("no active run")
	}
	if err := m.SetHold(context.Background(), repo, id, "second", "review hold", true); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-done:
		t.Fatalf("run finished despite holding second stage: status=%s second=%s held=%v", r.Status, r.Stages[1].Status, r.Stages[1].Held)
	case e := <-errs:
		t.Fatal(e)
	case <-time.After(3 * time.Second):
		if err := m.SetHold(context.Background(), repo, id, "second", "", false); err != nil {
			t.Fatal(err)
		}
		select {
		case r := <-done:
			if r.Status != model.RunReadyToIntegrate {
				t.Fatalf("released run: %+v", r)
			}
		case e := <-errs:
			t.Fatal(e)
		case <-time.After(5 * time.Second):
			t.Fatal("release did not resume scheduling")
		}

	}
}

func TestRegressionCleanupPreservesOperatorCommit(t *testing.T) {
	repo := initializedRepository(t)
	fake := testutil.WriteExecutable(t, t.TempDir(), "opencode", reviewFakeHeader+`printf 'worker\n' > "$NORTH_WORKTREE/result.txt"`)
	m := Manager{Paths: isolatedPaths(t), OpenCodeBinary: fake, StageTimeout: 5 * time.Second, AgentResolver: allowAgents{}}
	p := simplePlan("cleanup", model.AcceptanceCriterion{ID: "fails", Type: "contains", Path: "result.txt", Value: "not-present", Timeout: time.Second})
	r, err := m.Start(context.Background(), repo, p, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != model.RunFailed {
		t.Fatalf("expected failed run, got %s", r.Status)
	}
	runGit(t, r.Stages[0].Workspace, "add", "result.txt")
	runGit(t, r.Stages[0].Workspace, "commit", "-m", "operator rescue")
	if err = m.Cleanup(context.Background(), repo, r.ID); err == nil {
		t.Fatal("cleanup deleted a clean worktree and branch containing an unrecorded operator commit")
	}
	if _, err := os.Stat(r.Stages[0].Workspace); err != nil {
		t.Fatal("cleanup removed worktree before refusing:", err)
	}
	runGit(t, repo, "show-ref", "--verify", "refs/heads/"+r.Stages[0].Branch)
}

func TestRegressionStoppedRunCanResume(t *testing.T) {
	repo := initializedRepository(t)
	fake := testutil.WriteExecutable(t, t.TempDir(), "opencode", reviewFakeHeader+`if [ ! -f "$NORTH_WORKTREE/result.txt" ]; then printf 'pending' > "$NORTH_WORKTREE/result.txt"; sleep 30; fi
printf 'worker' > "$NORTH_WORKTREE/result.txt"`)
	m := Manager{Paths: isolatedPaths(t), OpenCodeBinary: fake, StageTimeout: time.Minute, AgentResolver: allowAgents{}}
	done := make(chan model.RunState, 1)
	errs := make(chan error, 1)
	go func() {
		r, e := m.Start(context.Background(), repo, simplePlan("stop", model.AcceptanceCriterion{ID: "exists", Type: "file-exists", Path: "result.txt", Timeout: time.Second}), RunOptions{})
		if e != nil {
			errs <- e
		} else {
			done <- r
		}
	}()
	var id string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runs, e := m.List(context.Background(), repo)
		if e == nil && len(runs) > 0 {
			r, e := m.Load(context.Background(), repo, runs[0].ID)
			if e == nil && r.Stages[0].Status == model.StageRunning {
				if _, err := os.Stat(filepath.Join(r.Stages[0].Workspace, "result.txt")); err != nil {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				id = r.ID
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if id == "" {
		t.Fatal("no active run")
	}
	if err := m.Stop(context.Background(), repo, id, "review"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case e := <-errs:
		t.Fatal(e)
	case <-time.After(5 * time.Second):
		t.Fatal("did not stop")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if r, err := m.Resume(ctx, repo, id); err != nil {
		t.Fatalf("stopped run cannot resume: %v", err)
	} else if r.Status != model.RunReadyToIntegrate {
		t.Fatalf("resumed status: %s", r.Status)
	}
}

func TestRegressionRepairReceivesHostFeedback(t *testing.T) {
	repo := initializedRepository(t)
	fake := testutil.WriteExecutable(t, t.TempDir(), "opencode", reviewFakeHeader+`
case "$*" in
 *"Previous host failure:"*"host-check-failed"*) printf 'repaired' > "$NORTH_WORKTREE/result.txt" ;;
 *) printf 'wrong' > "$NORTH_WORKTREE/result.txt" ;;
esac
printf '{"type":"session.updated","sessionID":"repair-session"}\n'
`)
	m := Manager{Paths: isolatedPaths(t), OpenCodeBinary: fake, StageTimeout: 5 * time.Second, AgentResolver: allowAgents{}, AllowShell: true}
	p := simplePlan("feedback", model.AcceptanceCriterion{ID: "check", Type: "shell", Command: []string{"if [ \"$(cat result.txt)\" != repaired ]; then echo host-check-failed; exit 1; fi"}, Timeout: time.Second})
	p.Spec.Policy.MaxAttemptsPerStage = 2
	r, err := m.Start(context.Background(), repo, p, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != model.RunReadyToIntegrate || r.Stages[0].Attempt != 2 {
		t.Fatalf("feedback repair failed: %+v", r)
	}
}

func TestRegressionVerificationRejectsOutOfScopeOutput(t *testing.T) {
	repo := initializedRepository(t)
	fake := testutil.WriteExecutable(t, t.TempDir(), "opencode", reviewFakeHeader+`printf 'worker' > "$NORTH_WORKTREE/result.txt"`)
	m := Manager{Paths: isolatedPaths(t), OpenCodeBinary: fake, StageTimeout: 5 * time.Second, AgentResolver: allowAgents{}, AllowShell: true}
	p := simplePlan("scope", model.AcceptanceCriterion{ID: "generate", Type: "shell", Command: []string{"printf unexpected > outside.txt"}, Timeout: time.Second})
	r, err := m.Start(context.Background(), repo, p, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != model.RunFailed || r.Stages[0].Failure == nil || r.Stages[0].Failure.Class != "scope" || r.Stages[0].CommitSHA != "" {
		t.Fatalf("out-of-scope verification output accepted: %+v", r)
	}
}

func TestRegressionPostMergeVerificationCannotChangeDeliveredTree(t *testing.T) {
	repo := initializedRepository(t)
	fake := testutil.WriteExecutable(t, t.TempDir(), "opencode", reviewFakeHeader+`printf 'worker' > "$NORTH_WORKTREE/result.txt"`)
	m := Manager{Paths: isolatedPaths(t), OpenCodeBinary: fake, StageTimeout: 5 * time.Second, AgentResolver: allowAgents{}, AllowShell: true}
	p := simplePlan("committed", model.AcceptanceCriterion{ID: "mutate", Type: "shell", Command: []string{"printf changed >> result.txt"}, Timeout: time.Second})
	r, err := m.Start(context.Background(), repo, p, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != model.RunFailed || r.Stages[0].Failure == nil || r.Stages[0].Failure.Class != "post-merge-verification" {
		t.Fatalf("dirty integration accepted: %+v", r)
	}
}
