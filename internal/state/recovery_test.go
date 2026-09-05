package state

import (
	"testing"
	"time"

	"github.com/SBakolis/north/internal/model"
)

func TestClassifyOrphanedStages(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	run := model.RunState{ID: "run-1", Stages: []model.StageState{
		{ID: "preparing", Status: model.StagePreparing, LastActivity: now.Add(-time.Hour)},
		{ID: "running", Status: model.StageRunning, LastActivity: now.Add(-time.Hour)},
		{ID: "verifying", Status: model.StageVerifying, LastActivity: now.Add(-time.Hour)},
		{ID: "merging", Status: model.StageMerging, LastActivity: now.Add(-time.Hour)},
		{ID: "fresh", Status: model.StageRunning, LastActivity: now.Add(-time.Minute)},
		{ID: "completed", Status: model.StageMerged, LastActivity: now.Add(-time.Hour)},
	}}
	orphaned := ClassifyOrphanedStages(run, now, 30*time.Minute)
	if len(orphaned) != 4 {
		t.Fatalf("orphaned = %#v", orphaned)
	}
	if run.Stages[0].Status != model.StagePreparing {
		t.Fatal("classification mutated run")
	}
}
