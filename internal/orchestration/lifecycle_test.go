package orchestration_test

import (
	"testing"
	"time"

	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/orchestration"
)

func TestStageTransitionsAreValidatedCentrally(t *testing.T) {
	stage := model.StageState{ID: "A", Status: model.StageWaitingForDependencies}
	now := time.Unix(10, 0)
	if err := orchestration.TransitionStage(&stage, model.StageRunning, now); err == nil {
		t.Fatal("WaitingForDependencies -> Running unexpectedly succeeded")
	}
	if err := orchestration.TransitionStage(&stage, model.StageReady, now); err != nil {
		t.Fatal(err)
	}
	if stage.Status != model.StageReady || !stage.LastActivity.Equal(now) {
		t.Fatalf("unexpected transition result: %+v", stage)
	}
}

func TestResumeStatusDoesNotGuessAboutActiveWork(t *testing.T) {
	for _, status := range []model.StageStatus{model.StagePreparing, model.StageRunning, model.StageVerifying} {
		if got, changed := orchestration.ResumeStatus(status); !changed || got != model.StageRetryScheduled {
			t.Fatalf("ResumeStatus(%s) = %s, %v", status, got, changed)
		}
	}
	if got, changed := orchestration.ResumeStatus(model.StageCommitReady); changed || got != model.StageCommitReady {
		t.Fatalf("ResumeStatus(CommitReady) = %s, %v", got, changed)
	}
	if got, changed := orchestration.ResumeStatus(model.StageMerging); !changed || got != model.StageNeedsHumanReview {
		t.Fatalf("ResumeStatus(Merging) = %s, %v", got, changed)
	}
}
