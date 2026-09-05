// Package conflict provides host-controlled automatic merge-conflict resolution.
package conflict

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	gitadapter "github.com/SBakolis/north/internal/git"
	"github.com/SBakolis/north/internal/orchestration"
)

const Agent = "north-conflict-resolver"

type GitOperations interface {
	PrepareConflictResolution(context.Context, gitadapter.ConflictWorktreeRequest) (orchestration.Workspace, []string, error)
	ChangedPaths(context.Context, string, string) ([]string, error)
	VerifyWriteScope(context.Context, string, []string, []string) error
	ContinueConflictResolution(context.Context, string, []string) (string, error)
	AbortConflictResolution(context.Context, string) error
	CleanupConflictResolution(context.Context, orchestration.Workspace) error
}

type Config struct {
	Git         GitOperations
	Runtime     orchestration.AgentRuntime
	Verifier    orchestration.VerificationProvider
	Integration orchestration.IntegrationProvider
	Timeout     time.Duration
}

type Provider struct{ config Config }

var _ orchestration.ConflictResolutionProvider = (*Provider)(nil)

func New(config Config) (*Provider, error) {
	if config.Git == nil || config.Runtime == nil || config.Verifier == nil || config.Integration == nil {
		return nil, errors.New("conflict provider requires git, runtime, verifier, and integration")
	}
	return &Provider{config: config}, nil
}

func (p *Provider) ResolveConflict(ctx context.Context, req orchestration.ConflictResolutionRequest, sink orchestration.EventSink) (result orchestration.ConflictResolutionResult) {
	if err := p.config.Runtime.Validate(ctx); err != nil {
		result.Err = fmt.Errorf("validate conflict resolver runtime: %w", err)
		return result
	}
	workspace, reproduced, err := p.config.Git.PrepareConflictResolution(ctx, gitadapter.ConflictWorktreeRequest{
		RunID: req.RunID, StageID: req.StageID, IntegrationHead: req.IntegrationHead, CommitSHA: req.CommitSHA,
	})
	if workspace.Path == "" {
		result.Err = err
		return result
	}
	continued := false
	defer func() {
		cleanupCtx := context.WithoutCancel(ctx)
		if !continued {
			result.Err = errors.Join(result.Err, p.config.Git.AbortConflictResolution(cleanupCtx, workspace.Path))
		}
		result.Err = errors.Join(result.Err, p.config.Git.CleanupConflictResolution(cleanupCtx, workspace))
	}()
	if err != nil {
		result.Err = err
		return result
	}
	if !samePaths(reproduced, req.ConflictingPaths) {
		result.Err = fmt.Errorf("reproduced conflicts differ: got %v, expected %v", reproduced, req.ConflictingPaths)
		return result
	}

	runtimeCtx := ctx
	cancel := func() {}
	if p.config.Timeout > 0 {
		runtimeCtx, cancel = context.WithTimeout(ctx, p.config.Timeout)
	}
	resultAgent, err := p.config.Runtime.Execute(runtimeCtx, orchestration.AgentRequest{
		RunID: req.RunID, StageID: req.StageID, Workspace: workspace.Path,
		Agent: Agent, Role: "conflict-resolver", Prompt: Prompt(req), Timeout: p.config.Timeout, Started: req.Started,
	}, sink)
	cancel()
	if req.Finished != nil {
		err = errors.Join(err, req.Finished(resultAgent))
	}
	if err != nil {
		result.Err = fmt.Errorf("execute conflict resolver: %w", err)
		return result
	}
	if resultAgent.ExitCode != 0 {
		result.Err = fmt.Errorf("conflict resolver exited with code %d", resultAgent.ExitCode)
		return result
	}
	paths, err := p.config.Git.ChangedPaths(ctx, workspace.Path, req.IntegrationHead)
	if err != nil {
		result.Err = fmt.Errorf("inspect resolution changes: %w", err)
		return result
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		result.Err = errors.New("conflict resolution produced no changes")
		return result
	}
	if err := p.config.Git.VerifyWriteScope(ctx, workspace.Path, paths, req.WriteScope); err != nil {
		result.Err = fmt.Errorf("verify resolution scope: %w", err)
		return result
	}
	verification := p.config.Verifier.Verify(ctx, orchestration.VerificationRequest{
		RunID: req.RunID, StageID: req.StageID, Workspace: workspace.Path,
		Criteria: req.Criteria, WriteScope: req.WriteScope,
	}, sink)
	result.Evidence = verification.Evidence
	if !verification.Passed {
		if verification.Failure != nil {
			result.Err = errors.New(verification.Failure.Message)
		} else {
			result.Err = errors.New("resolution acceptance verification failed")
		}
		return result
	}
	resolutionCommit, err := p.config.Git.ContinueConflictResolution(ctx, workspace.Path, paths)
	if err != nil {
		result.Err = fmt.Errorf("continue conflict resolution: %w", err)
		return result
	}
	continued = true
	integration := p.config.Integration.IntegrateStage(ctx, orchestration.StageIntegrationRequest{
		RunID: req.RunID, StageID: req.StageID, CommitSHA: resolutionCommit,
	})
	if integration.Err != nil {
		result.Err = fmt.Errorf("integrate resolution: %w", integration.Err)
		return result
	}
	if integration.CommitSHA == "" {
		result.Err = errors.New("resolution integration returned no commit")
		return result
	}
	result.CommitSHA = integration.CommitSHA
	return result
}

func Prompt(req orchestration.ConflictResolutionRequest) string {
	return fmt.Sprintf(`north.conflict/v1
Resolve the cherry-pick conflicts for stage %s.
Conflicting paths: %s
Allowed write scope: %s
Resolve all conflict markers and leave the working tree ready for host verification.
Do not commit, cherry-pick, merge, rebase, abort, create branches, or modify repository history.
The host inspects changed paths, verifies scope and acceptance criteria, and creates the commit.`,
		req.StageID, strings.Join(req.ConflictingPaths, ", "), strings.Join(req.WriteScope, ", "))
}

func samePaths(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	a = append([]string(nil), a...)
	b = append([]string(nil), b...)
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
