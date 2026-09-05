package git

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/SBakolis/north/internal/orchestration"
)

var (
	ErrConflictNotReproduced = errors.New("conflict was not reproduced")
	ErrUnresolvedPaths       = errors.New("conflict has unresolved paths")
	ErrNoCherryPick          = errors.New("no cherry-pick is in progress")
)

type ConflictWorktreeRequest struct {
	RunID, StageID, IntegrationHead, CommitSHA string
}

// PrepareConflictResolution creates a dedicated worktree at the current
// integration head and leaves the reproduced cherry-pick conflict in place.
func (a *Adapter) PrepareConflictResolution(ctx context.Context, req ConflictWorktreeRequest) (orchestration.Workspace, []string, error) {
	branch := SanitizeBranch("north", req.RunID, "conflict", req.StageID)
	path := filepath.Join(a.Repo.WorktreeRoot, SanitizeBranch(req.RunID), "conflicts", SanitizeBranch(req.StageID))
	workspace := orchestration.Workspace{Path: path, Branch: branch}
	if err := a.addWorktree(ctx, path, branch, req.IntegrationHead); err != nil {
		return orchestration.Workspace{}, nil, err
	}
	if _, err := a.Repo.Runner.Run(ctx, path, "cherry-pick", req.CommitSHA); err == nil {
		return workspace, nil, ErrConflictNotReproduced
	} else if conflicts := a.conflicts(ctx, path); len(conflicts) > 0 {
		return workspace, conflicts, nil
	} else {
		return workspace, nil, fmt.Errorf("reproduce cherry-pick conflict: %w", err)
	}
}

func (a *Adapter) UnresolvedPaths(ctx context.Context, workspace string) ([]string, error) {
	result, err := a.Repo.Runner.Run(ctx, workspace, "diff", "--name-only", "--diff-filter=U", "-z")
	if err != nil {
		return nil, err
	}
	return splitNUL(result.Stdout), nil
}

// ContinueConflictResolution stages only host-inspected paths and creates the
// cherry-pick commit. Agents are never trusted to create repository history.
func (a *Adapter) ContinueConflictResolution(ctx context.Context, workspace string, paths []string) (string, error) {
	if err := a.requireCherryPick(ctx, workspace); err != nil {
		return "", err
	}
	check, err := a.Repo.Runner.Run(ctx, workspace, "diff", "--check")
	if strings.Contains(check.Stdout, "leftover conflict marker") {
		return "", fmt.Errorf("%w: conflict markers remain", ErrUnresolvedPaths)
	}
	if err != nil && strings.TrimSpace(check.Stderr) != "" {
		return "", err
	}
	for _, path := range paths {
		if _, err := SafePath(workspace, path); err != nil {
			return "", err
		}
	}
	args := append([]string{"add", "-A", "--"}, paths...)
	if _, err := a.Repo.Runner.Run(ctx, workspace, args...); err != nil {
		return "", err
	}
	if unresolved, err := a.UnresolvedPaths(ctx, workspace); err != nil {
		return "", err
	} else if len(unresolved) > 0 {
		return "", fmt.Errorf("%w: %s", ErrUnresolvedPaths, strings.Join(unresolved, ", "))
	}
	if _, err := a.Repo.Runner.Run(ctx, workspace, "cherry-pick", "--continue"); err != nil {
		return "", err
	}
	return a.Repo.ResolveAt(ctx, workspace, "HEAD")
}

func (a *Adapter) AbortConflictResolution(ctx context.Context, workspace string) error {
	if err := a.requireCherryPick(ctx, workspace); err != nil {
		if errors.Is(err, ErrNoCherryPick) {
			return nil
		}
		return err
	}
	_, err := a.Repo.Runner.Run(ctx, workspace, "cherry-pick", "--abort")
	return err
}

// CleanupConflictResolution removes only a registered clean worktree, then its
// dedicated namespaced branch so a later retry can use the same identity.
func (a *Adapter) CleanupConflictResolution(ctx context.Context, workspace orchestration.Workspace) error {
	if !strings.HasPrefix(workspace.Branch, "north/") || !strings.Contains(workspace.Branch, "/conflict/") {
		return ErrUnsafeCleanup
	}
	if err := a.Cleanup(ctx, workspace); err != nil {
		return err
	}
	_, err := a.Repo.Runner.Run(ctx, a.Repo.Root, "branch", "-D", workspace.Branch)
	return err
}

func (a *Adapter) requireCherryPick(ctx context.Context, workspace string) error {
	if _, err := a.Repo.Runner.Run(ctx, workspace, "rev-parse", "--verify", "CHERRY_PICK_HEAD"); err != nil {
		return ErrNoCherryPick
	}
	return nil
}
