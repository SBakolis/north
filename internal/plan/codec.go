package plan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/SBakolis/north/internal/model"
	"gopkg.in/yaml.v3"
)

const (
	DefaultMaxParallel         = 1
	DefaultMaxAttemptsPerStage = 1
)

type durationWire time.Duration

func (d *durationWire) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("duration must be a string: %w", err)
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	*d = durationWire(parsed)
	return nil
}

func (d *durationWire) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return fmt.Errorf("duration must be a string")
	}
	parsed, err := time.ParseDuration(node.Value)
	if err != nil {
		return err
	}
	*d = durationWire(parsed)
	return nil
}

func (d durationWire) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d durationWire) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

type document struct {
	APIVersion string       `json:"apiVersion" yaml:"apiVersion"`
	Kind       string       `json:"kind" yaml:"kind"`
	Metadata   metadataWire `json:"metadata" yaml:"metadata"`
	Spec       specWire     `json:"spec" yaml:"spec"`
}

type metadataWire struct {
	Name string `json:"name" yaml:"name"`
}

type specWire struct {
	Goal    string      `json:"goal" yaml:"goal"`
	BaseRef string      `json:"baseRef" yaml:"baseRef"`
	Policy  policyWire  `json:"policy" yaml:"policy"`
	Stages  []stageWire `json:"stages" yaml:"stages"`
}

type policyWire struct {
	MaxParallel               int  `json:"maxParallel" yaml:"maxParallel"`
	FailFast                  bool `json:"failFast" yaml:"failFast"`
	MaxAttemptsPerStage       int  `json:"maxAttemptsPerStage" yaml:"maxAttemptsPerStage"`
	FinalVerificationRequired bool `json:"finalVerificationRequired" yaml:"finalVerificationRequired"`
	AutoResolveConflicts      bool `json:"autoResolveConflicts" yaml:"autoResolveConflicts"`
}

type stageWire struct {
	ID             string           `json:"id" yaml:"id"`
	Title          string           `json:"title" yaml:"title"`
	Description    string           `json:"description" yaml:"description"`
	DependsOn      []string         `json:"dependsOn" yaml:"dependsOn"`
	Agent          string           `json:"agent" yaml:"agent"`
	WriteScope     []string         `json:"writeScope" yaml:"writeScope"`
	Acceptance     []acceptanceWire `json:"acceptance" yaml:"acceptance"`
	AllowNoChanges bool             `json:"allowNoChanges,omitempty" yaml:"allowNoChanges,omitempty"`
}

type acceptanceWire struct {
	ID      string        `json:"id" yaml:"id"`
	Type    string        `json:"type" yaml:"type"`
	Command []string      `json:"command,omitempty" yaml:"command,omitempty"`
	Path    string        `json:"path,omitempty" yaml:"path,omitempty"`
	Value   string        `json:"value,omitempty" yaml:"value,omitempty"`
	Timeout *durationWire `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

func defaultDocument() document {
	return document{Spec: specWire{Policy: policyWire{
		MaxParallel: DefaultMaxParallel, MaxAttemptsPerStage: DefaultMaxAttemptsPerStage,
		FailFast: true, FinalVerificationRequired: true,
	}}}
}

// Parse reads either a JSON object or a YAML document, rejects unknown fields,
// applies defaults, and validates the resulting plan.
func Parse(data []byte) (model.ExecutionPlan, error) {
	var wire document
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return model.ExecutionPlan{}, fmt.Errorf("plan is empty")
	}
	var err error
	if trimmed[0] == '{' {
		wire, err = decodeJSON(trimmed)
	} else {
		wire, err = decodeYAML(trimmed)
	}
	if err != nil {
		return model.ExecutionPlan{}, err
	}
	result := fromDocument(wire)
	if err := Validate(result); err != nil {
		return model.ExecutionPlan{}, err
	}
	return result, nil
}

func ParseJSON(data []byte) (model.ExecutionPlan, error) {
	wire, err := decodeJSON(data)
	if err != nil {
		return model.ExecutionPlan{}, err
	}
	result := fromDocument(wire)
	if err := Validate(result); err != nil {
		return model.ExecutionPlan{}, err
	}
	return result, nil
}

func ParseYAML(data []byte) (model.ExecutionPlan, error) {
	wire, err := decodeYAML(data)
	if err != nil {
		return model.ExecutionPlan{}, err
	}
	result := fromDocument(wire)
	if err := Validate(result); err != nil {
		return model.ExecutionPlan{}, err
	}
	return result, nil
}

func decodeJSON(data []byte) (document, error) {
	wire := defaultDocument()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return document{}, fmt.Errorf("decode JSON plan: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return document{}, err
	}
	return wire, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode JSON plan: multiple values")
		}
		return fmt.Errorf("decode JSON plan: %w", err)
	}
	return nil
}

func decodeYAML(data []byte) (document, error) {
	wire := defaultDocument()
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&wire); err != nil {
		return document{}, fmt.Errorf("decode YAML plan: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return document{}, fmt.Errorf("decode YAML plan: multiple documents")
		}
		return document{}, fmt.Errorf("decode YAML plan: %w", err)
	}
	return wire, nil
}

// MarshalJSON returns stable, indented JSON with durations represented as strings.
func MarshalJSON(plan model.ExecutionPlan) ([]byte, error) {
	ApplyDefaults(&plan)
	data, err := json.MarshalIndent(toDocument(plan), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// MarshalYAML returns stable YAML with durations represented as strings.
func MarshalYAML(plan model.ExecutionPlan) ([]byte, error) {
	ApplyDefaults(&plan)
	return yaml.Marshal(toDocument(plan))
}

// ApplyDefaults fills zero-valued policy limits. Boolean defaults are applied
// while decoding, where omission can be distinguished from an explicit false.
func ApplyDefaults(plan *model.ExecutionPlan) {
	emptyPolicy := plan.Spec.Policy == (model.PlanPolicy{})
	if plan.Spec.Policy.MaxParallel == 0 {
		plan.Spec.Policy.MaxParallel = DefaultMaxParallel
	}
	if plan.Spec.Policy.MaxAttemptsPerStage == 0 {
		plan.Spec.Policy.MaxAttemptsPerStage = DefaultMaxAttemptsPerStage
	}
	if emptyPolicy {
		plan.Spec.Policy.FailFast = true
		plan.Spec.Policy.FinalVerificationRequired = true
	}
}

func fromDocument(w document) model.ExecutionPlan {
	p := model.ExecutionPlan{APIVersion: w.APIVersion, Kind: w.Kind, Metadata: model.PlanMetadata{Name: w.Metadata.Name}}
	p.Spec.Goal, p.Spec.BaseRef = w.Spec.Goal, w.Spec.BaseRef
	p.Spec.Policy = model.PlanPolicy(w.Spec.Policy)
	for _, s := range w.Spec.Stages {
		stage := model.Stage{ID: s.ID, Title: s.Title, Description: s.Description, DependsOn: s.DependsOn, Agent: s.Agent, WriteScope: s.WriteScope, AllowNoChanges: s.AllowNoChanges}
		for _, a := range s.Acceptance {
			criterion := model.AcceptanceCriterion{ID: a.ID, Type: a.Type, Command: a.Command, Path: a.Path, Value: a.Value}
			if a.Timeout != nil {
				criterion.Timeout = time.Duration(*a.Timeout)
			}
			stage.Acceptance = append(stage.Acceptance, criterion)
		}
		p.Spec.Stages = append(p.Spec.Stages, stage)
	}
	return p
}

func toDocument(p model.ExecutionPlan) document {
	w := document{APIVersion: p.APIVersion, Kind: p.Kind, Metadata: metadataWire{Name: p.Metadata.Name}}
	w.Spec.Goal, w.Spec.BaseRef = p.Spec.Goal, p.Spec.BaseRef
	w.Spec.Policy = policyWire(p.Spec.Policy)
	for _, s := range p.Spec.Stages {
		stage := stageWire{ID: s.ID, Title: s.Title, Description: s.Description, DependsOn: s.DependsOn, Agent: s.Agent, WriteScope: s.WriteScope, AllowNoChanges: s.AllowNoChanges}
		for _, a := range s.Acceptance {
			criterion := acceptanceWire{ID: a.ID, Type: a.Type, Command: a.Command, Path: a.Path, Value: a.Value}
			if a.Timeout != 0 {
				timeout := durationWire(a.Timeout)
				criterion.Timeout = &timeout
			}
			stage.Acceptance = append(stage.Acceptance, criterion)
		}
		w.Spec.Stages = append(w.Spec.Stages, stage)
	}
	return w
}

func trim(value string) string { return strings.TrimSpace(value) }
