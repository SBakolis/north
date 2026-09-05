package plan

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SBakolis/north/internal/model"
)

const (
	WarningOverlappingScopes = "overlapping-write-scopes"
	WarningBroadScope        = "broad-write-scope"
	WarningNoAcceptance      = "no-acceptance"
	WarningNoChanges         = "no-changes"
	WarningSerializedGraph   = "serialized-graph"
	WarningStageTooLarge     = "stage-too-large"
	stageRecoveryUnitLimit   = 12
)

func Warnings(p model.ExecutionPlan) []Warning {
	var warnings []Warning
	stages := append([]model.Stage(nil), p.Spec.Stages...)
	sort.Slice(stages, func(i, j int) bool { return stages[i].ID < stages[j].ID })
	for _, stage := range stages {
		if len(stage.Acceptance) == 0 {
			warnings = append(warnings, Warning{Code: WarningNoAcceptance, Message: fmt.Sprintf("stage %q has no acceptance criteria", stage.ID), Stages: []string{stage.ID}})
		}
		if stage.AllowNoChanges {
			warnings = append(warnings, Warning{Code: WarningNoChanges, Message: fmt.Sprintf("stage %q allows successful execution with no changes", stage.ID), Stages: []string{stage.ID}})
		} else if len(stage.WriteScope) == 0 {
			warnings = append(warnings, Warning{Code: WarningNoChanges, Message: fmt.Sprintf("stage %q has no write scope and does not allow no changes", stage.ID), Stages: []string{stage.ID}})
		}
		for _, scope := range stage.WriteScope {
			if broadScope(scope) {
				warnings = append(warnings, Warning{Code: WarningBroadScope, Message: fmt.Sprintf("stage %q has broad write scope %q", stage.ID, scope), Stages: []string{stage.ID}})
			}
		}
		units := len(stage.WriteScope) + len(stage.Acceptance)
		if units > stageRecoveryUnitLimit {
			warnings = append(warnings, Warning{Code: WarningStageTooLarge, Message: fmt.Sprintf("stage %q has %d recovery units (limit %d); split it into independently recoverable stages", stage.ID, units, stageRecoveryUnitLimit), Stages: []string{stage.ID}})
		}
	}
	for i := range stages {
		for j := i + 1; j < len(stages); j++ {
			if scopesOverlap(stages[i].WriteScope, stages[j].WriteScope) {
				warnings = append(warnings, Warning{Code: WarningOverlappingScopes, Message: fmt.Sprintf("stages %q and %q have overlapping write scopes", stages[i].ID, stages[j].ID), Stages: []string{stages[i].ID, stages[j].ID}})
			}
		}
	}
	if len(stages) > 1 && graphSerialized(stages) {
		warnings = append(warnings, Warning{Code: WarningSerializedGraph, Message: "dependency graph has no parallel stages"})
	}
	sort.SliceStable(warnings, func(i, j int) bool {
		if warnings[i].Code != warnings[j].Code {
			return warnings[i].Code < warnings[j].Code
		}
		return warnings[i].Message < warnings[j].Message
	})
	return warnings
}

func broadScope(scope string) bool {
	s := strings.Trim(strings.ReplaceAll(scope, `\`, "/"), "/")
	return s == "" || s == "." || s == "*" || s == "**" || s == "**/*"
}

func scopesOverlap(a, b []string) bool {
	for _, left := range a {
		for _, right := range b {
			if scopeOverlap(left, right) {
				return true
			}
		}
	}
	return false
}

func scopeOverlap(a, b string) bool {
	a = strings.TrimPrefix(strings.TrimSuffix(strings.ReplaceAll(a, `\`, "/"), "/**"), "./")
	b = strings.TrimPrefix(strings.TrimSuffix(strings.ReplaceAll(b, `\`, "/"), "/**"), "./")
	if broadScope(a) || broadScope(b) {
		return true
	}
	if a == b {
		return true
	}
	return strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

func graphSerialized(stages []model.Stage) bool {
	reachable := map[string]map[string]bool{}
	deps := map[string][]string{}
	for _, s := range stages {
		deps[s.ID] = s.DependsOn
	}
	var walk func(string, string)
	walk = func(root, id string) {
		for _, dep := range deps[id] {
			if !reachable[root][dep] {
				reachable[root][dep] = true
				walk(root, dep)
			}
		}
	}
	for id := range deps {
		reachable[id] = map[string]bool{}
		walk(id, id)
	}
	for i := range stages {
		for j := i + 1; j < len(stages); j++ {
			if !reachable[stages[i].ID][stages[j].ID] && !reachable[stages[j].ID][stages[i].ID] {
				return false
			}
		}
	}
	return true
}
