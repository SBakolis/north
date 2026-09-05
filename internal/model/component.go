package model

type ComponentType string

const (
	ComponentCore   ComponentType = "core"
	ComponentAgent  ComponentType = "agent"
	ComponentHook   ComponentType = "hook"
	ComponentPlugin ComponentType = "plugin"
	ComponentSkill  ComponentType = "skill"
	ComponentMCP    ComponentType = "mcp"
)

type Component struct {
	APIVersion           string        `json:"apiVersion" yaml:"apiVersion"`
	Kind                 string        `json:"kind" yaml:"kind"`
	ID                   string        `json:"id" yaml:"id"`
	Type                 ComponentType `json:"type" yaml:"type"`
	Name                 string        `json:"name" yaml:"name"`
	Description          string        `json:"description" yaml:"description"`
	Default              bool          `json:"default" yaml:"default"`
	Source               string        `json:"source" yaml:"source"`
	VersionPolicy        string        `json:"versionPolicy" yaml:"versionPolicy"`
	Requires             []string      `json:"requires" yaml:"requires"`
	Conflicts            []string      `json:"conflicts" yaml:"conflicts"`
	InstallStrategy      string        `json:"installStrategy" yaml:"installStrategy"`
	ManagedDestinations  []string      `json:"managedDestinations" yaml:"managedDestinations"`
	InstructionFragments []string      `json:"instructionFragments" yaml:"instructionFragments"`
	DoctorChecks         []string      `json:"doctorChecks" yaml:"doctorChecks"`
	UninstallBehavior    string        `json:"uninstallBehavior" yaml:"uninstallBehavior"`
}
