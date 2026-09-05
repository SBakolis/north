package model

import "time"

const (
	APIVersionV1Alpha1 = "north/v1alpha1"
	ExecutionPlanKind  = "ExecutionPlan"
)

type ExecutionPlan struct {
	APIVersion string       `json:"apiVersion" yaml:"apiVersion"`
	Kind       string       `json:"kind" yaml:"kind"`
	Metadata   PlanMetadata `json:"metadata" yaml:"metadata"`
	Spec       PlanSpec     `json:"spec" yaml:"spec"`
}

type PlanMetadata struct {
	Name string `json:"name" yaml:"name"`
}

type PlanSpec struct {
	Goal    string     `json:"goal" yaml:"goal"`
	BaseRef string     `json:"baseRef" yaml:"baseRef"`
	Policy  PlanPolicy `json:"policy" yaml:"policy"`
	Stages  []Stage    `json:"stages" yaml:"stages"`
}

type PlanPolicy struct {
	MaxParallel               int  `json:"maxParallel" yaml:"maxParallel"`
	FailFast                  bool `json:"failFast" yaml:"failFast"`
	MaxAttemptsPerStage       int  `json:"maxAttemptsPerStage" yaml:"maxAttemptsPerStage"`
	FinalVerificationRequired bool `json:"finalVerificationRequired" yaml:"finalVerificationRequired"`
	AutoResolveConflicts      bool `json:"autoResolveConflicts" yaml:"autoResolveConflicts"`
}

type Stage struct {
	ID             string                `json:"id" yaml:"id"`
	Title          string                `json:"title" yaml:"title"`
	Description    string                `json:"description" yaml:"description"`
	DependsOn      []string              `json:"dependsOn" yaml:"dependsOn"`
	Agent          string                `json:"agent" yaml:"agent"`
	WriteScope     []string              `json:"writeScope" yaml:"writeScope"`
	Acceptance     []AcceptanceCriterion `json:"acceptance" yaml:"acceptance"`
	AllowNoChanges bool                  `json:"allowNoChanges,omitempty" yaml:"allowNoChanges,omitempty"`
}

type AcceptanceCriterion struct {
	ID      string        `json:"id" yaml:"id"`
	Type    string        `json:"type" yaml:"type"`
	Command []string      `json:"command,omitempty" yaml:"command,omitempty"`
	Path    string        `json:"path,omitempty" yaml:"path,omitempty"`
	Value   string        `json:"value,omitempty" yaml:"value,omitempty"`
	Timeout time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}
