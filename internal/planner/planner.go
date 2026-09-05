// Package planner adapts the North planner agent to the planning contract.
package planner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/orchestration"
	planpkg "github.com/SBakolis/north/internal/plan"
)

type AgentPlanner struct {
	Runtime orchestration.AgentRuntime
}

var _ orchestration.Planner = (*AgentPlanner)(nil)

func (p *AgentPlanner) CreatePlan(ctx context.Context, input orchestration.PlanningInput) (model.ExecutionPlan, error) {
	if p.Runtime == nil {
		return model.ExecutionPlan{}, errors.New("planner runtime is required")
	}
	if err := p.Runtime.Validate(ctx); err != nil {
		return model.ExecutionPlan{}, fmt.Errorf("validate planner runtime: %w", err)
	}
	knowledge, err := json.Marshal(input.Knowledge)
	if err != nil {
		return model.ExecutionPlan{}, err
	}
	prompt := fmt.Sprintf(`Create a North execution plan for the goal below.
Return only one YAML document conforming to north/v1alpha1 ExecutionPlan.
Infer focused write scopes, deterministic acceptance checks, and dependencies.
Never implement the work. Avoid a repository-wide ** scope unless no narrower safe scope can be inferred.
Use baseRef %q.

Goal:
%s

Normalized knowledge snapshot:
%s`, input.BaseRef, input.Goal, knowledge)
	sink := &messageSink{}
	result, err := p.Runtime.Execute(ctx, orchestration.AgentRequest{
		Workspace: input.ProjectRoot, Prompt: prompt, Agent: "north-planner", Role: "planner",
	}, sink)
	if err != nil {
		return model.ExecutionPlan{}, fmt.Errorf("execute planner: %w", err)
	}
	if result.ExitCode != 0 {
		return model.ExecutionPlan{}, fmt.Errorf("planner exited with code %d", result.ExitCode)
	}
	candidates := append([]string(nil), sink.messages...)
	candidates = append(candidates, result.Output)
	var parseErrors []error
	for _, candidate := range candidates {
		for _, document := range planDocuments(candidate) {
			executionPlan, err := planpkg.Parse([]byte(document))
			if err == nil {
				return executionPlan, nil
			}
			parseErrors = append(parseErrors, err)
		}
	}
	return model.ExecutionPlan{}, fmt.Errorf("planner returned no valid execution plan: %w", errors.Join(parseErrors...))
}

type messageSink struct{ messages []string }

func (s *messageSink) Emit(_ context.Context, event model.Event) error {
	if strings.TrimSpace(event.Message) != "" {
		s.messages = append(s.messages, event.Message)
	}
	return nil
}

func planDocuments(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	documents := []string{value}
	for _, marker := range []string{"```yaml", "```yml", "```json", "```"} {
		remaining := value
		for {
			start := strings.Index(remaining, marker)
			if start < 0 {
				break
			}
			remaining = remaining[start+len(marker):]
			end := strings.Index(remaining, "```")
			if end < 0 {
				break
			}
			documents = append(documents, strings.TrimSpace(remaining[:end]))
			remaining = remaining[end+3:]
		}
	}
	if start := strings.Index(value, "apiVersion:"); start >= 0 {
		documents = append(documents, value[start:])
	}
	return documents
}
