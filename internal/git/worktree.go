package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SBakolis/north/internal/orchestration"
)

type Adapter struct {
	Repo *Repository
}

var _ orchestration.IsolationProvider = (*Adapter)(nil)
var _ orchestration.IntegrationProvider = (*Adapter)(nil)
var _ orchestration.ChangedPathInspector = (*Adapter)(nil)
var _ orchestration.WriteScopeVerifier = (*Adapter)(nil)
var _ orchestration.ExactPathCommitter = (*Adapter)(nil)

func NewAdapter(repo *Repository) *Adapter { return &Adapter{Repo: repo} }

func (a *Adapter) Prepare(ctx context.Context, req orchestration.IsolationRequest) (orchestration.Workspace, error) {
	branch := SanitizeBranch("north", req.RunID, "stage", req.StageID)
	path := filepath.Join(a.Repo.WorktreeRoot, SanitizeBranch(req.RunID), "stages", SanitizeBranch(req.StageID))
	if err := a.addWorktree(ctx, path, branch, req.BaseCommit); err != nil {
		return orchestration.Workspace{}, err
	}
	return orchestration.Workspace{Path: path, Branch: branch}, nil
}

func (a *Adapter) PrepareIntegration(ctx context.Context, runID, baseCommit string) (orchestration.Workspace, error) {
	branch := SanitizeBranch("north", runID, "integration")
	path := filepath.Join(a.Repo.WorktreeRoot, SanitizeBranch(runID), "integration")
	if err := a.addWorktree(ctx, path, branch, baseCommit); err != nil {
		return orchestration.Workspace{}, err
	}
	return orchestration.Workspace{Path: path, Branch: branch}, nil
}

func (a *Adapter) addWorktree(ctx context.Context, path, branch, base string) error {
	if !within(a.Repo.WorktreeRoot, path) || within(a.Repo.Root, path) {
		return ErrUnsafeCleanup
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	canonical, err := canonicalPath(path)
	if err != nil || !within(a.Repo.WorktreeRoot, canonical) || within(a.Repo.Root, canonical) {
		return fmt.Errorf("%w: canonical worktree path %s escapes managed root", ErrSymlinkEscape, canonical)
	}
	_, err = a.Repo.Runner.Run(ctx, a.Repo.Root, "worktree", "add", "-b", branch, canonical, base)
	return err
}

func (a *Adapter) Cleanup(ctx context.Context, workspace orchestration.Workspace) error {
	path, err := filepath.Abs(workspace.Path)
	if err != nil {
		return fmt.Errorf("%w: resolve path: %v", ErrUnsafeCleanup, err)
	}
	if !within(a.Repo.WorktreeRoot, path) || path == a.Repo.WorktreeRoot || within(a.Repo.Root, path) {
		return fmt.Errorf("%w: path %s is outside managed worktree root %s", ErrUnsafeCleanup, path, a.Repo.WorktreeRoot)
	}
	comparisonPath := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		comparisonPath = resolved
	}
	listed, err := a.Repo.Runner.Run(ctx, a.Repo.Root, "worktree", "list", "--porcelain")
	if err != nil {
		return err
	}
	found := false
	for _, line := range strings.Split(listed.Stdout, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			listedPath, _ := filepath.Abs(strings.TrimPrefix(line, "worktree "))
			if resolved, err := filepath.EvalSymlinks(listedPath); err == nil {
				listedPath = resolved
			}
			found = listedPath == comparisonPath
			if found {
				break
			}
		}
	}
	if !found {
		return fmt.Errorf("%w: %s is not a registered worktree (registered: %q)", ErrUnsafeCleanup, path, listed.Stdout)
	}
	if dirty, err := a.Repo.Runner.Run(ctx, path, "status", "--porcelain=v1", "--untracked-files=all"); err != nil || dirty.Stdout != "" {
		return fmt.Errorf("%w: worktree is dirty", ErrUnsafeCleanup)
	}
	_, err = a.Repo.Runner.Run(ctx, a.Repo.Root, "worktree", "remove", path)
	return err
}

func (a *Adapter) DeleteManagedBranch(ctx context.Context, branch string) error {
	if !strings.HasPrefix(branch, "north/") || branch == "north/" {
		return fmt.Errorf("%w: refuse to delete non-North branch %q", ErrUnsafeCleanup, branch)
	}
	if result, err := a.Repo.Runner.Run(ctx, a.Repo.Root, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		if result.ExitCode == 1 {
			return nil
		}
		return err
	}
	_, err := a.Repo.Runner.Run(ctx, a.Repo.Root, "branch", "--delete", "--force", branch)
	return err
}

func (a *Adapter) ChangedPaths(ctx context.Context, workspace, base string) ([]string, error) {
	return a.Repo.ChangedPaths(ctx, workspace, base)
}

func (a *Adapter) VerifyWriteScope(_ context.Context, workspace string, changed, scope []string) error {
	return ValidateScope(workspace, changed, scope)
}

func (a *Adapter) CommitPaths(ctx context.Context, req orchestration.ExactPathCommitRequest) (string, error) {
	return a.CommitExactPaths(ctx, req.Workspace, req.Paths, req.Message)
}

func (a *Adapter) CommitExactPaths(ctx context.Context, workspace string, paths []string, message string) (string, error) {
	for _, path := range paths {
		if _, err := SafePath(workspace, path); err != nil {
			return "", err
		}
	}
	before, err := a.Repo.Runner.Run(ctx, workspace, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return "", err
	}
	if before.Stdout != "" {
		return "", fmt.Errorf("index must be empty before exact-path commit")
	}
	args := append([]string{"add", "--"}, paths...)
	if _, err := a.Repo.Runner.Run(ctx, workspace, args...); err != nil {
		return "", err
	}
	staged, err := a.Repo.Runner.Run(ctx, workspace, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return "", err
	}
	actual := splitNUL(staged.Stdout)
	if !samePaths(actual, paths) {
		return "", fmt.Errorf("staged paths differ from requested exact paths")
	}
	commitArgs := append([]string{"commit", "-m", message, "--only", "--"}, paths...)
	if _, err := a.Repo.Runner.Run(ctx, workspace, commitArgs...); err != nil {
		return "", err
	}
	return a.Repo.ResolveAt(ctx, workspace, "HEAD")
}

func (r *Repository) ResolveAt(ctx context.Context, dir, ref string) (string, error) {
	result, err := r.Runner.Run(ctx, dir, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func splitNUL(value string) []string {
	var result []string
	for _, value := range strings.Split(value, "\x00") {
		if value != "" {
			result = append(result, filepath.ToSlash(value))
		}
	}
	return result
}

func samePaths(a, b []string) bool {
	set := make(map[string]int)
	for _, path := range a {
		set[filepath.ToSlash(filepath.Clean(path))]++
	}
	for _, path := range b {
		set[filepath.ToSlash(filepath.Clean(path))]--
	}
	if len(a) != len(b) {
		return false
	}
	for _, count := range set {
		if count != 0 {
			return false
		}
	}
	return true
}
