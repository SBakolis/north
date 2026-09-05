package conflict_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/SBakolis/north/internal/conflict"
	gitadapter "github.com/SBakolis/north/internal/git"
	"github.com/SBakolis/north/internal/orchestration"
)

type resolvingRuntime struct{}

func (resolvingRuntime) Validate(context.Context) error       { return nil }
func (resolvingRuntime) Cancel(context.Context, string) error { return nil }
func (resolvingRuntime) Execute(_ context.Context, req orchestration.AgentRequest, _ orchestration.EventSink) (orchestration.AgentResult, error) {
	if req.Agent != conflict.Agent {
		return orchestration.AgentResult{ExitCode: 1}, nil
	}
	return orchestration.AgentResult{ExitCode: 0}, os.WriteFile(filepath.Join(req.Workspace, "tracked.txt"), []byte("resolved\n"), 0o600)
}

type acceptingVerifier struct{}

func (acceptingVerifier) Verify(context.Context, orchestration.VerificationRequest, orchestration.EventSink) orchestration.VerificationResult {
	return orchestration.VerificationResult{Passed: true, Evidence: []string{"accepted"}}
}

func TestProviderResolvesRealCherryPickConflict(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	gitRun(t, root, "init", "-b", "main")
	gitRun(t, root, "config", "user.name", "North Test")
	gitRun(t, root, "config", "user.email", "north@example.invalid")
	write(t, filepath.Join(root, "tracked.txt"), "base\n")
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-m", "base")
	repo, err := gitadapter.Open(ctx, root, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	adapter := gitadapter.NewAdapter(repo)
	base, err := repo.ResolveBase(ctx, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	integrationWorkspace, err := adapter.PrepareIntegration(ctx, "run", base)
	if err != nil {
		t.Fatal(err)
	}
	first, err := adapter.Prepare(ctx, orchestration.IsolationRequest{RunID: "run", StageID: "first", BaseCommit: base})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(first.Path, "tracked.txt"), "integration\n")
	firstCommit, err := adapter.CommitExactPaths(ctx, first.Path, []string{"tracked.txt"}, "first")
	if err != nil {
		t.Fatal(err)
	}
	if integrated := adapter.IntegrateStage(ctx, orchestration.StageIntegrationRequest{RunID: "run", StageID: "first", CommitSHA: firstCommit}); integrated.Err != nil {
		t.Fatal(integrated.Err)
	}

	second, err := adapter.Prepare(ctx, orchestration.IsolationRequest{RunID: "run", StageID: "second", BaseCommit: base})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(second.Path, "tracked.txt"), "stage\n")
	secondCommit, err := adapter.CommitExactPaths(ctx, second.Path, []string{"tracked.txt"}, "second")
	if err != nil {
		t.Fatal(err)
	}
	failed := adapter.IntegrateStage(ctx, orchestration.StageIntegrationRequest{RunID: "run", StageID: "second", CommitSHA: secondCommit})
	if failed.Err == nil || len(failed.ConflictingPaths) != 1 {
		t.Fatalf("expected conflict, got %+v", failed)
	}
	integrationHead, err := repo.ResolveAt(ctx, integrationWorkspace.Path, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := conflict.New(conflict.Config{Git: adapter, Runtime: resolvingRuntime{}, Verifier: acceptingVerifier{}, Integration: adapter})
	if err != nil {
		t.Fatal(err)
	}
	result := provider.ResolveConflict(ctx, orchestration.ConflictResolutionRequest{
		RunID: "run", StageID: "second", IntegrationHead: integrationHead, CommitSHA: secondCommit,
		ConflictingPaths: failed.ConflictingPaths, WriteScope: []string{"tracked.txt"},
	}, nil)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	content, err := os.ReadFile(filepath.Join(integrationWorkspace.Path, "tracked.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "resolved\n" || result.CommitSHA == "" {
		t.Fatalf("content = %q, commit = %q", content, result.CommitSHA)
	}
	resolutionPath := filepath.Join(repo.WorktreeRoot, "run", "conflicts", "second")
	if _, err := os.Stat(resolutionPath); !os.IsNotExist(err) {
		t.Fatalf("resolution worktree was not removed: %v", err)
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

func write(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}
