package state

import (
	"time"

	"github.com/SBakolis/north/internal/model"
)

type OrphanedStage struct {
	RunID    string
	Stage    model.StageState
	StaleFor time.Duration
}

// ClassifyOrphanedStages returns active stages whose last activity is older
// than staleAfter. It never modifies the supplied run or persisted state.
func ClassifyOrphanedStages(run model.RunState, now time.Time, staleAfter time.Duration) []OrphanedStage {
	var orphaned []OrphanedStage
	for _, stage := range run.Stages {
		if !canBecomeOrphaned(stage.Status) || stage.LastActivity.IsZero() {
			continue
		}
		staleFor := now.Sub(stage.LastActivity)
		if staleFor >= staleAfter {
			orphaned = append(orphaned, OrphanedStage{RunID: run.ID, Stage: stage, StaleFor: staleFor})
		}
	}
	return orphaned
}

func canBecomeOrphaned(status model.StageStatus) bool {
	switch status {
	case model.StagePreparing, model.StageRunning, model.StageVerifying, model.StageCommitReady, model.StageMerging:
		return true
	default:
		return false
	}
}
