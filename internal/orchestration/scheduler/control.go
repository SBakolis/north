package scheduler

import (
	"context"
	"errors"
	"fmt"

	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/orchestration"
)

func (s *Scheduler) execution(runID string) (*execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	exec := s.active[runID]
	if exec == nil {
		return nil, ErrNotRunning
	}
	return exec, nil
}

func (s *Scheduler) HoldStage(ctx context.Context, runID, stageID, reason string) error {
	exec, err := s.execution(runID)
	if err != nil {
		if !errors.Is(err, ErrNotRunning) {
			return err
		}
		run, loadErr := s.config.Store.LoadRun(ctx, runID)
		if loadErr != nil {
			return loadErr
		}
		index := stageIndex(run.Stages, stageID)
		if index < 0 {
			return fmt.Errorf("unknown stage %q", stageID)
		}
		if err := orchestration.SetStageHold(&run.Stages[index], true, reason); err != nil {
			return err
		}
		return s.config.Store.UpdateStage(context.WithoutCancel(ctx), runID, run.Stages[index])
	}
	exec.persist.Lock()
	defer exec.persist.Unlock()
	exec.mu.Lock()
	index := stageIndex(exec.run.Stages, stageID)
	if index < 0 {
		exec.mu.Unlock()
		return fmt.Errorf("unknown stage %q", stageID)
	}
	if err := orchestration.SetStageHold(&exec.run.Stages[index], true, reason); err != nil {
		exec.mu.Unlock()
		return err
	}
	stage := exec.run.Stages[index]
	exec.mu.Unlock()
	if err := s.config.Store.UpdateStage(context.WithoutCancel(ctx), runID, stage); err != nil {
		return err
	}
	notify(exec)
	return nil
}

func (s *Scheduler) ReleaseStage(ctx context.Context, runID, stageID string) error {
	exec, err := s.execution(runID)
	if err != nil {
		if !errors.Is(err, ErrNotRunning) {
			return err
		}
		run, loadErr := s.config.Store.LoadRun(ctx, runID)
		if loadErr != nil {
			return loadErr
		}
		index := stageIndex(run.Stages, stageID)
		if index < 0 {
			return fmt.Errorf("unknown stage %q", stageID)
		}
		if err := orchestration.SetStageHold(&run.Stages[index], false, ""); err != nil {
			return err
		}
		return s.config.Store.UpdateStage(context.WithoutCancel(ctx), runID, run.Stages[index])
	}
	exec.persist.Lock()
	defer exec.persist.Unlock()
	exec.mu.Lock()
	index := stageIndex(exec.run.Stages, stageID)
	if index < 0 {
		exec.mu.Unlock()
		return fmt.Errorf("unknown stage %q", stageID)
	}
	if err := orchestration.SetStageHold(&exec.run.Stages[index], false, ""); err != nil {
		exec.mu.Unlock()
		return err
	}
	stage := exec.run.Stages[index]
	exec.mu.Unlock()
	if err := s.config.Store.UpdateStage(context.WithoutCancel(ctx), runID, stage); err != nil {
		return err
	}
	notify(exec)
	return nil
}

func (s *Scheduler) RetryStage(ctx context.Context, runID, stageID string) error {
	exec, err := s.execution(runID)
	if err != nil {
		if !errors.Is(err, ErrNotRunning) {
			return err
		}
		return s.retryStoredStage(ctx, runID, stageID)
	}
	if err := s.transition(exec, stageID, model.StageReady, func(stage *model.StageState) {
		stage.Failure = nil
		stage.ConflictingPaths = ""
		stage.RetryEligibleAt = s.config.Now().UTC()
	}); err != nil {
		return err
	}
	if err := s.resetBlocked(exec); err != nil {
		return err
	}
	notify(exec)
	return nil
}

func (s *Scheduler) retryStoredStage(ctx context.Context, runID, stageID string) error {
	run, err := s.config.Store.LoadRun(ctx, runID)
	if err != nil {
		return err
	}
	index := stageIndex(run.Stages, stageID)
	if index < 0 {
		return fmt.Errorf("unknown stage %q", stageID)
	}
	from := run.Stages[index].Status
	if err := orchestration.TransitionStage(&run.Stages[index], model.StageReady, s.config.Now()); err != nil {
		return err
	}
	run.Stages[index].Failure = nil
	run.Stages[index].ConflictingPaths = ""
	run.Stages[index].RetryEligibleAt = s.config.Now().UTC()
	if err := s.persistStage(ctx, runID, run.Stages[index], from); err != nil {
		return err
	}
	for i := range run.Stages {
		if run.Stages[i].Status != model.StageBlocked {
			continue
		}
		blockedFrom := run.Stages[i].Status
		if err := orchestration.TransitionStage(&run.Stages[i], model.StageWaitingForDependencies, s.config.Now()); err != nil {
			return err
		}
		if err := s.persistStage(ctx, runID, run.Stages[i], blockedFrom); err != nil {
			return err
		}
	}
	if run.Status == model.RunFailed || run.Status == model.RunCancelled {
		run.Cancellation = nil
		return s.persistRunTransition(ctx, &run, model.RunRunning)
	}
	return nil
}

func (s *Scheduler) resetBlocked(exec *execution) error {
	exec.mu.Lock()
	var ids []string
	for _, stage := range exec.run.Stages {
		if stage.Status == model.StageBlocked {
			ids = append(ids, stage.ID)
		}
	}
	exec.mu.Unlock()
	for _, id := range ids {
		if err := s.transition(exec, id, model.StageWaitingForDependencies, func(stage *model.StageState) { stage.Failure = nil }); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) Cancel(ctx context.Context, runID, reason string) error {
	exec, err := s.execution(runID)
	if err != nil {
		return err
	}
	if err := s.requestCancellation(exec, reason); err != nil {
		return err
	}
	exec.cancel()
	notify(exec)
	return nil
}

func stageIndex(stages []model.StageState, id string) int {
	for i := range stages {
		if stages[i].ID == id {
			return i
		}
	}
	return -1
}

func notify(exec *execution) {
	select {
	case exec.wake <- struct{}{}:
	default:
	}
}
