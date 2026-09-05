package git

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/SBakolis/north/internal/orchestration"
)

func (a *Adapter) IntegrateStage(ctx context.Context, req orchestration.StageIntegrationRequest) orchestration.IntegrationResult {
	path := filepath.Join(a.Repo.WorktreeRoot, SanitizeBranch(req.RunID), "integration")
	if _, err := a.Repo.Runner.Run(ctx, path, "cherry-pick", req.CommitSHA); err != nil {
		conflicts := a.conflicts(ctx, path)
		_, abortErr := a.Repo.Runner.Run(context.WithoutCancel(ctx), path, "cherry-pick", "--abort")
		if abortErr != nil {
			err = errors.Join(err, abortErr)
		}
		return orchestration.IntegrationResult{ConflictingPaths: conflicts, Err: errors.Join(ErrIntegrationConflict, err)}
	}
	sha, err := a.Repo.ResolveAt(ctx, path, "HEAD")
	return orchestration.IntegrationResult{CommitSHA: sha, Err: err}
}

func (a *Adapter) conflicts(ctx context.Context, path string) []string {
	result, err := a.Repo.Runner.Run(ctx, path, "diff", "--name-only", "--diff-filter=U", "-z")
	if err != nil {
		return nil
	}
	return splitNUL(result.Stdout)
}

func (a *Adapter) IntegrateRun(ctx context.Context, req orchestration.RunIntegrationRequest) orchestration.IntegrationResult {
	integrationPath := filepath.Join(a.Repo.WorktreeRoot, SanitizeBranch(req.RunID), "integration")
	integrationSHA, err := a.Repo.ResolveAt(ctx, integrationPath, "HEAD")
	if err != nil {
		return orchestration.IntegrationResult{Err: err}
	}
	targetSHA, err := a.Repo.ResolveBase(ctx, req.TargetBranch)
	if err != nil {
		return orchestration.IntegrationResult{Err: err}
	}
	base, err := a.Repo.Runner.Run(ctx, a.Repo.Root, "merge-base", targetSHA, integrationSHA)
	if err != nil {
		return orchestration.IntegrationResult{Err: err}
	}
	if strings.TrimSpace(base.Stdout) != targetSHA {
		return orchestration.IntegrationResult{Err: ErrTargetDiverged}
	}
	current, err := a.Repo.Runner.Run(ctx, a.Repo.Root, "branch", "--show-current")
	if err != nil {
		return orchestration.IntegrationResult{Err: err}
	}
	if strings.TrimSpace(current.Stdout) != req.TargetBranch {
		return orchestration.IntegrationResult{Err: ErrTargetDiverged}
	}
	if err := a.Repo.Clean(ctx); err != nil {
		return orchestration.IntegrationResult{Err: err}
	}
	if _, err := a.Repo.Runner.Run(ctx, a.Repo.Root, "merge", "--ff-only", integrationSHA); err != nil {
		return orchestration.IntegrationResult{Err: err}
	}
	sha, err := a.Repo.ResolveBase(ctx, "HEAD")
	return orchestration.IntegrationResult{CommitSHA: sha, Err: err}
}
