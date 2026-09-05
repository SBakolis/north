package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SBakolis/north/internal/orchestration"
)

func TestRepositoryChangedPathsScopeAndSymlinkEscape(t *testing.T) {
	repo, adapter := testRepository(t)
	write(t, filepath.Join(repo.Root, "tracked.txt"), "changed")
	write(t, filepath.Join(repo.Root, "nested", "new.txt"), "new")
	paths, err := repo.ChangedPaths(context.Background(), repo.Root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "nested/new.txt" || paths[1] != "tracked.txt" {
		t.Fatalf("paths = %#v", paths)
	}
	if err := ValidateScope(repo.Root, paths, []string{"nested/**", "*.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateScope(repo.Root, []string{"nested/new.txt"}, []string{"nested"}); err != nil {
		t.Fatalf("directory scope should include descendants: %v", err)
	}
	if err := ValidateScope(repo.Root, paths, []string{"nested/**"}); !errors.Is(err, ErrOutsideScope) {
		t.Fatalf("expected scope error, got %v", err)
	}

	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(repo.Root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := ValidateScope(repo.Root, []string{"escape/file"}, []string{"escape/**"}); !errors.Is(err, ErrSymlinkEscape) {
		t.Fatalf("expected symlink error, got %v", err)
	}
	_ = adapter
}

func TestOpenRejectsSymlinkedWorktreeRootInsideRepository(t *testing.T) {
	repo, _ := testRepository(t)
	cacheLink := filepath.Join(t.TempDir(), "cache")
	if err := os.Symlink(repo.Root, cacheLink); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), repo.Root, filepath.Join(cacheLink, "worktrees", "project"), nil); err == nil || !strings.Contains(err.Error(), "outside repository") {
		t.Fatalf("error = %v", err)
	}
}

func TestSetWorktreeRootRevalidatesProjectSpecificPath(t *testing.T) {
	repo, _ := testRepository(t)
	cacheLink := filepath.Join(t.TempDir(), "cache")
	if err := os.Symlink(repo.Root, cacheLink); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetWorktreeRoot(filepath.Join(cacheLink, "worktrees", "project")); err == nil {
		t.Fatal("accepted project worktree root through repository symlink")
	}
}

func TestPrepareRejectsSymlinkAddedBelowCanonicalWorktreeRoot(t *testing.T) {
	repo, adapter := testRepository(t)
	runPath := filepath.Join(repo.WorktreeRoot, "run")
	if err := os.MkdirAll(repo.WorktreeRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(repo.Root, runPath); err != nil {
		t.Fatal(err)
	}
	_, err := adapter.Prepare(context.Background(), orchestration.IsolationRequest{RunID: "run", StageID: "stage", BaseCommit: "HEAD"})
	if !errors.Is(err, ErrSymlinkEscape) {
		t.Fatalf("error = %v", err)
	}
}

func TestWorktreesExactCommitAndProgressiveConflictAbort(t *testing.T) {
	repo, adapter := testRepository(t)
	ctx := context.Background()
	base, _ := repo.ResolveBase(ctx, "HEAD")
	integration, err := adapter.PrepareIntegration(ctx, "run 1", base)
	if err != nil {
		t.Fatal(err)
	}
	one, err := adapter.Prepare(ctx, orchestration.IsolationRequest{RunID: "run 1", StageID: "one", BaseCommit: base})
	if err != nil {
		t.Fatal(err)
	}
	two, err := adapter.Prepare(ctx, orchestration.IsolationRequest{RunID: "run 1", StageID: "two", BaseCommit: base})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(one.Path, "tracked.txt"), "one")
	write(t, filepath.Join(one.Path, "other.txt"), "not committed")
	shaOne, err := adapter.CommitExactPaths(ctx, one.Path, []string{"tracked.txt"}, "one")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(one.Path, "other.txt")); err != nil {
		t.Fatal(err)
	}
	if result := adapter.IntegrateStage(ctx, orchestration.StageIntegrationRequest{RunID: "run 1", CommitSHA: shaOne}); result.Err != nil {
		t.Fatal(result.Err)
	}

	write(t, filepath.Join(two.Path, "tracked.txt"), "two")
	shaTwo, err := adapter.CommitExactPaths(ctx, two.Path, []string{"tracked.txt"}, "two")
	if err != nil {
		t.Fatal(err)
	}
	result := adapter.IntegrateStage(ctx, orchestration.StageIntegrationRequest{RunID: "run 1", CommitSHA: shaTwo})
	if !errors.Is(result.Err, ErrIntegrationConflict) || len(result.ConflictingPaths) != 1 {
		t.Fatalf("result = %#v", result)
	}
	gitPath := exec.Command("git", "rev-parse", "--git-path", "CHERRY_PICK_HEAD")
	gitPath.Dir = integration.Path
	output, err := gitPath.Output()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(strings.TrimSpace(string(output))); !os.IsNotExist(err) {
		t.Fatalf("cherry-pick was not aborted: %v", err)
	}
	integrationHead, err := repo.ResolveAt(ctx, integration.Path, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	resolution, reproduced, err := adapter.PrepareConflictResolution(ctx, ConflictWorktreeRequest{RunID: "run 1", StageID: "two", IntegrationHead: integrationHead, CommitSHA: shaTwo})
	if err != nil || len(reproduced) != 1 {
		t.Fatalf("reproduce conflict: paths=%v err=%v", reproduced, err)
	}
	if _, err := adapter.ContinueConflictResolution(ctx, resolution.Path, []string{"tracked.txt"}); !errors.Is(err, ErrUnresolvedPaths) {
		t.Fatalf("expected unresolved path rejection, got %v", err)
	}
	if err := adapter.AbortConflictResolution(ctx, resolution.Path); err != nil {
		t.Fatal(err)
	}
	if err := adapter.CleanupConflictResolution(ctx, resolution); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Cleanup(ctx, orchestration.Workspace{Path: repo.Root}); !errors.Is(err, ErrUnsafeCleanup) {
		t.Fatalf("unsafe cleanup = %v", err)
	}
}

func TestFinalIntegrationRejectsDivergedTarget(t *testing.T) {
	repo, adapter := testRepository(t)
	ctx := context.Background()
	base, _ := repo.ResolveBase(ctx, "HEAD")
	if _, err := adapter.PrepareIntegration(ctx, "run", base); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(repo.Root, "host.txt"), "host")
	gitRun(t, repo.Root, "add", "host.txt")
	gitRun(t, repo.Root, "commit", "-m", "host")
	result := adapter.IntegrateRun(ctx, orchestration.RunIntegrationRequest{RunID: "run", TargetBranch: "main"})
	if !errors.Is(result.Err, ErrTargetDiverged) {
		t.Fatalf("expected divergence, got %v", result.Err)
	}
}

func testRepository(t *testing.T) (*Repository, *Adapter) {
	t.Helper()
	root := t.TempDir()
	gitRun(t, root, "init", "-b", "main")
	gitRun(t, root, "config", "user.name", "North Test")
	gitRun(t, root, "config", "user.email", "north@example.invalid")
	write(t, filepath.Join(root, "tracked.txt"), "base")
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-m", "base")
	repo, err := Open(context.Background(), root, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return repo, NewAdapter(repo)
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
