package planner

import (
	"context"
	"strings"
	"testing"

	"github.com/SBakolis/north/internal/orchestration"
)

type runtimeStub struct {
	request orchestration.AgentRequest
	output  string
}

func (*runtimeStub) Validate(context.Context) error       { return nil }
func (*runtimeStub) Cancel(context.Context, string) error { return nil }
func (r *runtimeStub) Execute(_ context.Context, request orchestration.AgentRequest, _ orchestration.EventSink) (orchestration.AgentResult, error) {
	r.request = request
	return orchestration.AgentResult{Output: r.output}, nil
}

func TestAgentPlannerProducesValidatedPlan(t *testing.T) {
	runtime := &runtimeStub{output: "```yaml\napiVersion: north/v1alpha1\nkind: ExecutionPlan\nmetadata:\n  name: test\nspec:\n  goal: test\n  baseRef: main\n  policy:\n    maxParallel: 1\n    maxAttemptsPerStage: 1\n    finalVerificationRequired: true\n  stages:\n    - id: implementation\n      title: Implement\n      description: Implement safely\n      agent: north-worker\n      writeScope: [internal/**]\n      acceptance:\n        - id: tests\n          type: exec\n          command: [go, test, ./internal/...]\n```"}
	executionPlan, err := (&AgentPlanner{Runtime: runtime}).CreatePlan(context.Background(), orchestration.PlanningInput{Goal: "test", ProjectRoot: "/repo", BaseRef: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if len(executionPlan.Spec.Stages) != 1 || runtime.request.Agent != "north-planner" || runtime.request.Role != "planner" || !strings.Contains(runtime.request.Prompt, "Infer focused write scopes") {
		t.Fatalf("plan = %+v request = %+v", executionPlan, runtime.request)
	}
}
