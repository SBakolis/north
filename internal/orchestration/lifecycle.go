package orchestration

import (
	"fmt"
	"time"

	"github.com/SBakolis/north/internal/model"
)

var stageTransitions = map[model.StageStatus]map[model.StageStatus]bool{
	model.StageWaitingForDependencies: {model.StageReady: true, model.StageBlocked: true, model.StageCancelled: true},
	model.StageReady:                  {model.StagePreparing: true, model.StageMerging: true, model.StageBlocked: true, model.StageCancelled: true},
	model.StagePreparing:              {model.StageRunning: true, model.StageRetryScheduled: true, model.StageFailed: true, model.StageCancelled: true, model.StageNeedsHumanReview: true},
	model.StageRunning:                {model.StageVerifying: true, model.StageRetryScheduled: true, model.StageFailed: true, model.StageCancelled: true, model.StageNeedsHumanReview: true},
	model.StageVerifying:              {model.StageCommitReady: true, model.StageRetryScheduled: true, model.StageFailed: true, model.StageCancelled: true, model.StageNeedsHumanReview: true},
	model.StageRetryScheduled:         {model.StageReady: true, model.StageCancelled: true},
	model.StageCommitReady:            {model.StageMerging: true, model.StageRetryScheduled: true, model.StageFailed: true, model.StageCancelled: true, model.StageNeedsHumanReview: true},
	model.StageMerging:                {model.StagePostMergeVerifying: true, model.StageMergeConflict: true, model.StageRetryScheduled: true, model.StageFailed: true, model.StageCancelled: true, model.StageNeedsHumanReview: true},
	model.StagePostMergeVerifying:     {model.StageMerged: true, model.StageFailed: true, model.StageCancelled: true, model.StageNeedsHumanReview: true},
	model.StageFailed:                 {model.StageReady: true, model.StagePostMergeVerifying: true},
	model.StageMergeConflict:          {model.StageReady: true},
	model.StageNeedsHumanReview:       {model.StageReady: true, model.StageCancelled: true},
	model.StageBlocked:                {model.StageReady: true, model.StageWaitingForDependencies: true, model.StageCancelled: true},
	model.StageCancelled:              {model.StageReady: true},
}

var runTransitions = map[model.RunStatus]map[model.RunStatus]bool{
	model.RunPreparing:        {model.RunRunning: true, model.RunFailed: true, model.RunCancelled: true},
	model.RunRunning:          {model.RunReadyToIntegrate: true, model.RunFailed: true, model.RunCancelled: true},
	model.RunReadyToIntegrate: {model.RunCompleted: true, model.RunFailed: true, model.RunCancelled: true},
	model.RunFailed:           {model.RunRunning: true},
	model.RunCancelled:        {model.RunRunning: true},
}

func TransitionRun(run *model.RunState, to model.RunStatus, now time.Time) error {
	if run == nil {
		return fmt.Errorf("run is nil")
	}
	if run.Status == to || !runTransitions[run.Status][to] {
		return fmt.Errorf("invalid run transition %s -> %s", run.Status, to)
	}
	run.Status = to
	run.UpdatedAt = now.UTC()
	return nil
}

func CanTransitionStage(from, to model.StageStatus) bool {
	return from != to && stageTransitions[from][to]
}

// TransitionStage is the single authority for stage lifecycle changes.
func TransitionStage(stage *model.StageState, to model.StageStatus, now time.Time) error {
	if stage == nil {
		return fmt.Errorf("stage is nil")
	}
	if !CanTransitionStage(stage.Status, to) {
		return fmt.Errorf("invalid stage transition %s -> %s", stage.Status, to)
	}
	stage.Status = to
	stage.LastActivity = now.UTC()
	return nil
}

func IsTerminalStage(status model.StageStatus) bool {
	switch status {
	case model.StageMerged, model.StageMergeConflict, model.StageFailed, model.StageBlocked,
		model.StageNeedsHumanReview, model.StageCancelled, model.StageSkipped:
		return true
	default:
		return false
	}
}

func IsActiveStage(status model.StageStatus) bool {
	switch status {
	case model.StagePreparing, model.StageRunning, model.StageVerifying, model.StageCommitReady, model.StageMerging, model.StagePostMergeVerifying:
		return true
	default:
		return false
	}
}

type StageTransition struct {
	StageID string
	From    model.StageStatus
	To      model.StageStatus
}

// NormalizeRunForResume marks interrupted external work for review. The caller
// persists each returned transition before resuming scheduling.
func NormalizeRunForResume(run *model.RunState, now time.Time) ([]StageTransition, error) {
	if run == nil {
		return nil, fmt.Errorf("run is nil")
	}
	var transitions []StageTransition
	for i := range run.Stages {
		to, changed := ResumeStatus(run.Stages[i].Status)
		if !changed {
			continue
		}
		from := run.Stages[i].Status
		if err := TransitionStage(&run.Stages[i], to, now); err != nil {
			return nil, err
		}
		transitions = append(transitions, StageTransition{StageID: run.Stages[i].ID, From: from, To: to})
	}
	return transitions, nil
}

func SetStageHold(stage *model.StageState, held bool, reason string) error {
	if stage == nil {
		return fmt.Errorf("stage is nil")
	}
	if held && IsActiveStage(stage.Status) {
		return fmt.Errorf("stage %q is active", stage.ID)
	}
	stage.Held = held
	if held {
		stage.HoldReason = reason
	} else {
		stage.HoldReason = ""
	}
	return nil
}

// ResumeStatus is conservative because completion of interrupted host or agent
// work cannot be inferred solely from a durable stage snapshot.
func ResumeStatus(status model.StageStatus) (model.StageStatus, bool) {
	if status == model.StagePostMergeVerifying {
		return status, false
	}
	if status == model.StageMerging {
		return model.StageNeedsHumanReview, true
	}
	if status == model.StageCommitReady {
		return status, false
	}
	if IsActiveStage(status) {
		return model.StageRetryScheduled, true
	}
	return status, false
}
