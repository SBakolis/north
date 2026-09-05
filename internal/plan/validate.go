package plan

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/SBakolis/north/internal/model"
)

const MaxAcceptanceTimeout = 24 * time.Hour
const MaxParallel = 64

var (
	namePattern            = regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?(?:\.[a-z0-9](?:[-a-z0-9]*[a-z0-9])?)*$`)
	idPattern              = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?$`)
	windowsAbsolutePattern = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
)

type Warning struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Stages  []string `json:"stages,omitempty"`
}

type ValidationError struct{ Problems []string }

func (e *ValidationError) Error() string { return "invalid plan: " + strings.Join(e.Problems, "; ") }

type BaseRefResolver interface {
	ResolveBase(context.Context, string) (string, error)
}

type AgentResolver interface {
	ResolveAgent(context.Context, string) error
}

type ProviderResolver interface {
	ResolveProvider(context.Context, string) error
}

type ValidationOptions struct {
	BaseRefResolver  BaseRefResolver
	AgentResolver    AgentResolver
	ProviderResolver ProviderResolver
	KnownAgents      map[string]struct{}
	KnownProviders   map[string]struct{}
}

func Validate(p model.ExecutionPlan) error {
	_, err := ValidateWithOptions(context.Background(), p, ValidationOptions{})
	return err
}

func ValidateWithOptions(ctx context.Context, p model.ExecutionPlan, options ValidationOptions) ([]Warning, error) {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	if p.APIVersion != model.APIVersionV1Alpha1 {
		add("apiVersion must be %q", model.APIVersionV1Alpha1)
	}
	if p.Kind != model.ExecutionPlanKind {
		add("kind must be %q", model.ExecutionPlanKind)
	}
	if !namePattern.MatchString(p.Metadata.Name) || len(p.Metadata.Name) > 63 {
		add("metadata.name must be a lowercase DNS-style name of at most 63 characters")
	}
	if trim(p.Spec.Goal) == "" {
		add("spec.goal is required")
	}
	if trim(p.Spec.BaseRef) == "" {
		add("spec.baseRef is required")
	}
	if p.Spec.Policy.MaxParallel < 1 || p.Spec.Policy.MaxParallel > MaxParallel {
		add("spec.policy.maxParallel must be between 1 and %d", MaxParallel)
	}
	if p.Spec.Policy.MaxAttemptsPerStage < 1 || p.Spec.Policy.MaxAttemptsPerStage > 10 {
		add("spec.policy.maxAttemptsPerStage must be between 1 and 10")
	}
	if len(p.Spec.Stages) == 0 {
		add("spec.stages must contain at least one stage")
	}

	ids := make(map[string]model.Stage, len(p.Spec.Stages))
	for i, stage := range p.Spec.Stages {
		where := fmt.Sprintf("spec.stages[%d]", i)
		if !idPattern.MatchString(stage.ID) || len(stage.ID) > 63 {
			add("%s.id is invalid", where)
		}
		if _, exists := ids[stage.ID]; exists {
			add("duplicate stage ID %q", stage.ID)
		} else {
			ids[stage.ID] = stage
		}
		if trim(stage.Title) == "" {
			add("stage %q title is required", stage.ID)
		}
		if trim(stage.Description) == "" {
			add("stage %q description is required", stage.ID)
		}
		if err := validateAgent(ctx, stage.Agent, options); err != nil {
			add("stage %q agent: %v", stage.ID, err)
		}
		seenDeps := map[string]bool{}
		for _, dep := range stage.DependsOn {
			if dep == stage.ID {
				add("stage %q cannot depend on itself", stage.ID)
			}
			if seenDeps[dep] {
				add("stage %q has duplicate dependency %q", stage.ID, dep)
			}
			seenDeps[dep] = true
		}
		for _, scope := range stage.WriteScope {
			if err := validateSafePath(scope); err != nil {
				add("stage %q writeScope %q: %v", stage.ID, scope, err)
			}
		}
		seenAcceptance := map[string]bool{}
		for j, criterion := range stage.Acceptance {
			if !idPattern.MatchString(criterion.ID) || len(criterion.ID) > 63 {
				add("stage %q acceptance[%d].id is invalid", stage.ID, j)
			}
			if seenAcceptance[criterion.ID] {
				add("stage %q has duplicate acceptance ID %q", stage.ID, criterion.ID)
			}
			seenAcceptance[criterion.ID] = true
			validateCriterion(stage.ID, criterion, add)
		}
	}
	for _, stage := range p.Spec.Stages {
		for _, dep := range stage.DependsOn {
			if _, exists := ids[dep]; !exists {
				add("stage %q depends on unknown stage %q", stage.ID, dep)
			}
		}
	}
	if cycle := dependencyCycle(p.Spec.Stages); len(cycle) > 0 {
		add("dependency cycle: %s", strings.Join(cycle, " -> "))
	}
	if options.BaseRefResolver != nil && trim(p.Spec.BaseRef) != "" {
		if _, err := options.BaseRefResolver.ResolveBase(ctx, p.Spec.BaseRef); err != nil {
			add("spec.baseRef %q cannot be resolved: %v", p.Spec.BaseRef, err)
		}
	}
	sort.Strings(problems)
	warnings := Warnings(p)
	if len(problems) > 0 {
		return warnings, &ValidationError{Problems: problems}
	}
	return warnings, nil
}

func validateAgent(ctx context.Context, agent string, options ValidationOptions) error {
	if agent == "" {
		return nil
	}
	if err := ValidateAgentReference(agent); err != nil {
		return err
	}
	if len(options.KnownAgents) > 0 {
		if _, ok := options.KnownAgents[agent]; !ok {
			return fmt.Errorf("unknown agent %q", agent)
		}
	}
	if options.AgentResolver != nil {
		if err := options.AgentResolver.ResolveAgent(ctx, agent); err != nil {
			return err
		}
	}
	if provider, _, ok := strings.Cut(agent, ":"); ok {
		if provider == "" {
			return fmt.Errorf("provider is empty")
		}
		if len(options.KnownProviders) > 0 {
			if _, found := options.KnownProviders[provider]; !found {
				return fmt.Errorf("unknown provider %q", provider)
			}
		}
		if options.ProviderResolver != nil {
			if err := options.ProviderResolver.ResolveProvider(ctx, provider); err != nil {
				return err
			}
		}
	}
	return nil
}

func ValidateAgentReference(agent string) error {
	provider, name, qualified := strings.Cut(agent, ":")
	if !qualified {
		name = provider
	}
	if !idPattern.MatchString(name) || len(name) > 63 {
		return fmt.Errorf("agent name %q is invalid", name)
	}
	if qualified && (!idPattern.MatchString(provider) || len(provider) > 63) {
		return fmt.Errorf("provider %q is invalid", provider)
	}
	return nil
}

func validateCriterion(stageID string, c model.AcceptanceCriterion, add func(string, ...any)) {
	where := fmt.Sprintf("stage %q acceptance %q", stageID, c.ID)
	if c.Timeout < 0 || c.Timeout > MaxAcceptanceTimeout {
		add("%s timeout must be between 0 and %s", where, MaxAcceptanceTimeout)
	}
	switch c.Type {
	case "command", "exec":
		if len(c.Command) == 0 {
			add("%s command must not be empty", where)
		}
		for _, arg := range c.Command {
			if arg == "" {
				add("%s command arguments must not be empty", where)
				break
			}
		}
		if c.Path != "" || c.Value != "" {
			add("%s %s type cannot set path or value", where, c.Type)
		}
	case "shell":
		if len(c.Command) != 1 || c.Command[0] == "" {
			add("%s shell command must contain exactly one non-empty string", where)
		}
		if c.Path != "" || c.Value != "" {
			add("%s shell type cannot set path or value", where)
		}
	case "file-exists", "file-not-exists":
		if c.Path == "" {
			add("%s path is required", where)
		} else if err := validateSafePath(c.Path); err != nil {
			add("%s path %q: %v", where, c.Path, err)
		}
		if len(c.Command) != 0 || c.Value != "" {
			add("%s %s type only accepts path and timeout", where, c.Type)
		}
	case "contains", "matches":
		if c.Path == "" {
			add("%s path is required", where)
		} else if err := validateSafePath(c.Path); err != nil {
			add("%s path %q: %v", where, c.Path, err)
		}
		if c.Value == "" {
			add("%s value is required", where)
		}
		if c.Type == "matches" && c.Value != "" {
			if _, err := regexp.Compile(c.Value); err != nil {
				add("%s value is not a valid regular expression: %v", where, err)
			}
		}
		if len(c.Command) != 0 {
			add("%s %s type only accepts path, value, and timeout", where, c.Type)
		}
	case "git-diff-not-empty":
		if len(c.Command) != 0 || c.Path != "" {
			add("%s git-diff-not-empty type only accepts an optional base ref value and timeout", where)
		}
	default:
		add("%s has unknown type %q", where, c.Type)
	}
}

func validateSafePath(value string) error {
	if value == "" {
		return fmt.Errorf("path is empty")
	}
	normalized := strings.ReplaceAll(value, `\`, "/")
	if strings.HasPrefix(normalized, "/") || windowsAbsolutePattern.MatchString(value) {
		return fmt.Errorf("absolute paths are not allowed")
	}
	for _, part := range strings.Split(normalized, "/") {
		if part == ".." {
			return fmt.Errorf("parent traversal is not allowed")
		}
	}
	if path.Clean(normalized) == ".." || strings.HasPrefix(path.Clean(normalized), "../") {
		return fmt.Errorf("parent traversal is not allowed")
	}
	return nil
}

func dependencyCycle(stages []model.Stage) []string {
	deps := map[string][]string{}
	for _, s := range stages {
		deps[s.ID] = s.DependsOn
	}
	state, stack := map[string]uint8{}, []string{}
	var cycle []string
	var visit func(string) bool
	visit = func(id string) bool {
		if state[id] == 1 {
			for i, item := range stack {
				if item == id {
					cycle = append(append([]string{}, stack[i:]...), id)
					return true
				}
			}
		}
		if state[id] == 2 {
			return false
		}
		state[id] = 1
		stack = append(stack, id)
		next := append([]string(nil), deps[id]...)
		sort.Strings(next)
		for _, dep := range next {
			if _, exists := deps[dep]; exists && visit(dep) {
				return true
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = 2
		return false
	}
	ids := make([]string, 0, len(deps))
	for id := range deps {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if visit(id) {
			return cycle
		}
	}
	return nil
}
