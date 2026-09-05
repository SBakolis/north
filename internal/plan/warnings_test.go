package plan

import (
	"strings"
	"testing"

	"github.com/SBakolis/north/internal/model"
)

func TestWarningsIdentifyStagesAboveRecoveryUnitLimit(t *testing.T) {
	stage := model.Stage{ID: "large", Title: "Large", Description: "Large", WriteScope: make([]string, stageRecoveryUnitLimit), Acceptance: []model.AcceptanceCriterion{{ID: "check", Type: "file-exists", Path: "result"}}}
	warnings := Warnings(model.ExecutionPlan{Spec: model.PlanSpec{Stages: []model.Stage{stage}}})
	found := false
	for _, warning := range warnings {
		if warning.Code == WarningStageTooLarge {
			found = true
			if len(warning.Stages) != 1 || warning.Stages[0] != stage.ID || !strings.Contains(warning.Message, "13 recovery units") {
				t.Fatalf("warning = %+v", warning)
			}
		}
	}
	if !found {
		t.Fatalf("warnings = %+v", warnings)
	}
	stage.Acceptance = nil
	for _, warning := range Warnings(model.ExecutionPlan{Spec: model.PlanSpec{Stages: []model.Stage{stage}}}) {
		if warning.Code == WarningStageTooLarge {
			t.Fatalf("stage at limit was warned: %+v", warning)
		}
	}
}
