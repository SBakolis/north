package plan

import (
	"strings"
	"testing"
	"time"

	"github.com/SBakolis/north/internal/model"
)

const validYAML = `apiVersion: north/v1alpha1
kind: ExecutionPlan
metadata:
  name: example
spec:
  goal: Test the plan
  baseRef: main
  stages:
    - id: build
      title: Build
      description: Build the feature.
      agent: north-worker
      writeScope: [internal/plan]
      acceptance:
        - id: tests
          type: command
          command: [go, test, ./internal/plan]
          timeout: 90s
`

func TestParseDefaultsAndDurationRoundTrip(t *testing.T) {
	p, err := Parse([]byte(validYAML))
	if err != nil {
		t.Fatal(err)
	}
	if p.Spec.Policy.MaxParallel != 1 || p.Spec.Policy.MaxAttemptsPerStage != 1 || !p.Spec.Policy.FailFast || !p.Spec.Policy.FinalVerificationRequired {
		t.Fatalf("defaults not applied: %+v", p.Spec.Policy)
	}
	if got := p.Spec.Stages[0].Acceptance[0].Timeout; got != 90*time.Second {
		t.Fatalf("timeout = %s", got)
	}
	data, err := MarshalJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"timeout": "1m30s"`) {
		t.Fatalf("JSON duration is not a string:\n%s", data)
	}
	if _, err := ParseJSON(data); err != nil {
		t.Fatalf("round trip: %v", err)
	}
}

func TestParsePreservesExplicitFalsePolicy(t *testing.T) {
	data := strings.Replace(validYAML, "  stages:", "  policy:\n    failFast: false\n    finalVerificationRequired: false\n  stages:", 1)
	p, err := ParseYAML([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if p.Spec.Policy.FailFast || p.Spec.Policy.FinalVerificationRequired {
		t.Fatalf("explicit false values replaced: %+v", p.Spec.Policy)
	}
}

func TestAutoResolveConflictsRoundTrip(t *testing.T) {
	data := strings.Replace(validYAML, "  stages:", "  policy:\n    autoResolveConflicts: true\n  stages:", 1)
	p, err := ParseYAML([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if !p.Spec.Policy.AutoResolveConflicts {
		t.Fatal("autoResolveConflicts was not decoded")
	}
	encoded, err := MarshalJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"autoResolveConflicts": true`) {
		t.Fatalf("policy was not encoded:\n%s", encoded)
	}
}

func TestParseRejectsUnknownFields(t *testing.T) {
	data := strings.Replace(validYAML, "  goal:", "  typo: true\n  goal:", 1)
	if _, err := ParseYAML([]byte(data)); err == nil || !strings.Contains(err.Error(), "field typo not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRejectsUnsafeAndCyclicPlan(t *testing.T) {
	p := testPlan()
	p.Spec.Stages[0].DependsOn = []string{"two"}
	p.Spec.Stages[1].DependsOn = []string{"one"}
	p.Spec.Stages[1].WriteScope = []string{"..\\secret"}
	err := Validate(p)
	if err == nil || !strings.Contains(err.Error(), "dependency cycle") || !strings.Contains(err.Error(), "parent traversal") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRejectsUnsafeAgentReference(t *testing.T) {
	p, err := Parse([]byte(validYAML))
	if err != nil {
		t.Fatal(err)
	}
	p.Spec.Stages[0].Agent = "../../../tmp/external-agent"
	if err := Validate(p); err == nil || !strings.Contains(err.Error(), "agent name") {
		t.Fatalf("error = %v", err)
	}
}

func TestApprovalHashIgnoresSemanticOrdering(t *testing.T) {
	a := testPlan()
	b := testPlan()
	b.Spec.Stages[0], b.Spec.Stages[1] = b.Spec.Stages[1], b.Spec.Stages[0]
	a.Spec.Stages[1].WriteScope = []string{"docs", "internal"}
	b.Spec.Stages[0].WriteScope = []string{"internal", "docs"}
	ha, err := ApprovalHash(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := ApprovalHash(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("hashes differ: %s != %s", ha, hb)
	}
	if got := a.Spec.Stages[1].WriteScope; got[0] != "docs" {
		t.Fatalf("hash mutated caller: %v", got)
	}
}

func TestApprovalHashPreservesAcceptanceExecutionOrder(t *testing.T) {
	a := testPlan()
	a.Spec.Stages[0].Acceptance = append(a.Spec.Stages[0].Acceptance, model.AcceptanceCriterion{ID: "second", Type: "exec", Command: []string{"go", "vet", "./..."}})
	b := a
	b.Spec.Stages = append([]model.Stage(nil), a.Spec.Stages...)
	b.Spec.Stages[0].Acceptance = append([]model.AcceptanceCriterion(nil), a.Spec.Stages[0].Acceptance...)
	b.Spec.Stages[0].Acceptance[0], b.Spec.Stages[0].Acceptance[1] = b.Spec.Stages[0].Acceptance[1], b.Spec.Stages[0].Acceptance[0]
	ha, err := ApprovalHash(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := ApprovalHash(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha == hb {
		t.Fatal("ordered acceptance commands shared an approval hash")
	}
}

func testPlan() model.ExecutionPlan {
	return model.ExecutionPlan{
		APIVersion: model.APIVersionV1Alpha1,
		Kind:       model.ExecutionPlanKind,
		Metadata:   model.PlanMetadata{Name: "test-plan"},
		Spec: model.PlanSpec{
			Goal: "test", BaseRef: "main",
			Policy: model.PlanPolicy{MaxParallel: 2, MaxAttemptsPerStage: 1},
			Stages: []model.Stage{
				{ID: "one", Title: "One", Description: "First.", Agent: "worker", WriteScope: []string{"internal"}, Acceptance: []model.AcceptanceCriterion{{ID: "one-test", Type: "command", Command: []string{"go", "test"}}}},
				{ID: "two", Title: "Two", Description: "Second.", Agent: "worker", DependsOn: []string{"one"}, WriteScope: []string{"docs"}, Acceptance: []model.AcceptanceCriterion{{ID: "two-test", Type: "file-exists", Path: "docs/readme.md"}}},
			},
		},
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(validYAML))
	jsonSeed, _ := MarshalJSON(testPlan())
	f.Add(jsonSeed)
	f.Add([]byte("apiVersion: north/v1alpha1\nunknown: true\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := Parse(data)
		if err != nil {
			return
		}
		encoded, err := MarshalJSON(p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ParseJSON(encoded); err != nil {
			t.Fatalf("valid parse did not round trip: %v", err)
		}
	})
}

func FuzzValidateNeverPanics(f *testing.F) {
	f.Add("stage", "dependency", "scope/path")
	f.Add("same", "same", "../escape")
	f.Fuzz(func(t *testing.T, id, dependency, scope string) {
		p := testPlan()
		p.Spec.Stages[0].ID = id
		p.Spec.Stages[0].DependsOn = []string{dependency}
		p.Spec.Stages[0].WriteScope = []string{scope}
		_ = Validate(p)
	})
}
