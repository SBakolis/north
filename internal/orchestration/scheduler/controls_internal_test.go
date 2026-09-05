package scheduler

import (
	"errors"
	"github.com/SBakolis/north/internal/model"
	"strings"
	"testing"
)

func TestConcurrentHoldSurvivesDependencyTransitionAndPreventsLaunch(t *testing.T) {
	current := model.RunState{Stages: []model.StageState{{ID: "a", Status: model.StageWaitingForDependencies, Held: true, HoldReason: "operator"}}}
	desired := model.RunState{Stages: []model.StageState{{ID: "a", Status: model.StageReady}}}
	event := model.Event{Type: "stage.transition", StageID: "a", Data: map[string]any{"from": model.StageWaitingForDependencies}}
	if err := applyEventMutation(&current, desired, event); err != nil {
		t.Fatal(err)
	}
	if !current.Stages[0].Held || current.Stages[0].HoldReason != "operator" {
		t.Fatal("lost concurrent hold")
	}
	desired.Stages[0].Status = model.StagePreparing
	event.Data["from"] = model.StageReady
	if err := applyEventMutation(&current, desired, event); !errors.Is(err, errStageHeld) {
		t.Fatalf("launch did not honor concurrent hold: %v", err)
	}
	if current.Stages[0].Status != model.StageReady {
		t.Fatal("held stage mutated")
	}
}

func TestPromptIncludesCriteriaAndRepairEvidence(t *testing.T) {
	stage := model.Stage{Acceptance: []model.AcceptanceCriterion{{ID: "check", Type: "contains", Path: "output.txt", Value: "expected"}}}
	prompt := RepairPrompt(model.ExecutionPlan{}, stage, model.StageState{Failure: &model.StageFailure{Class: "verification", Message: "check failed"}, Evidence: model.NewStringList([]string{"diagnostic details"})})
	for _, text := range []string{"output.txt", "expected", "check failed", "diagnostic details"} {
		if !strings.Contains(prompt, text) {
			t.Fatalf("missing %q from prompt: %s", text, prompt)
		}
	}
}
