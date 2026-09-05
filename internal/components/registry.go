package components

import (
	"fmt"
	"sort"

	"github.com/SBakolis/north/internal/model"
)

const componentKind = "Component"

// BuiltinRegistry returns the North-owned MVP components. Skills and MCPs are
// deliberately absent even though their type identifiers are reserved.
func BuiltinRegistry() []model.Component {
	return []model.Component{
		{
			APIVersion: model.APIVersionV1Alpha1, Kind: componentKind,
			ID: "core", Type: model.ComponentCore, Name: "North Core",
			Description: "Core OpenCode instructions and installation metadata.",
			Default:     true, Source: "bundled", VersionPolicy: "north-version",
			InstructionFragments: []string{"core.md"}, UninstallBehavior: "remove-managed",
		},
		{
			APIVersion: model.APIVersionV1Alpha1, Kind: componentKind,
			ID: "knowledge.none", Type: model.ComponentCore, Name: "No knowledge provider",
			Description: "Use goals and execution plans without an external knowledge provider.",
			Default:     true, Source: "bundled", VersionPolicy: "north-version",
			Requires: []string{"core"}, Conflicts: []string{"knowledge.openspec"}, UninstallBehavior: "remove-managed",
		},
		{
			APIVersion: model.APIVersionV1Alpha1, Kind: componentKind,
			ID: "knowledge.openspec", Type: model.ComponentCore, Name: "OpenSpec knowledge provider",
			Description: "Install instructions for the optional OpenSpec knowledge adapter.",
			Default:     false, Source: "bundled", VersionPolicy: "north-version",
			Requires: []string{"core"}, Conflicts: []string{"knowledge.none"},
			InstructionFragments: []string{"openspec.md"}, UninstallBehavior: "remove-managed",
		},
		{
			APIVersion: model.APIVersionV1Alpha1, Kind: componentKind,
			ID: "parallelization", Type: model.ComponentCore, Name: "Parallel orchestration",
			Description: "Install North agents, guardrails, and parallel orchestration rules.",
			Default:     true, Source: "bundled", VersionPolicy: "north-version",
			Requires: []string{"core"},
			ManagedDestinations: []string{
				"agents/north-planner.md", "agents/north-worker.md",
				"agents/north-verifier.md", "agents/north-conflict-resolver.md",
				"plugins/north-guardrails.ts",
			},
			InstructionFragments: []string{"parallelization.md"}, UninstallBehavior: "remove-managed",
		},
		{
			APIVersion: model.APIVersionV1Alpha1, Kind: componentKind,
			ID: "plugin.opencode-codex-meter", Type: model.ComponentPlugin, Name: "OpenCode Codex Meter",
			Description: "Install the optional Codex usage meter OpenCode plugin.",
			Default:     false, Source: "opencode-codex-meter", VersionPolicy: "plugin-manager",
			Requires: []string{"core"}, InstallStrategy: "opencode-plugin",
			DoctorChecks: []string{"plugin-registration"}, UninstallBehavior: "remove-owned-registration",
		},
		{
			APIVersion: model.APIVersionV1Alpha1, Kind: componentKind,
			ID: "plugin.open-loop", Type: model.ComponentPlugin, Name: "Open Loop",
			Description: "Install the optional goal and time based loop OpenCode plugin.",
			Default:     false, Source: "@sbakolis/open-loop", VersionPolicy: "plugin-manager",
			Requires: []string{"core"}, InstallStrategy: "opencode-plugin",
			InstructionFragments: []string{"open-loop.md"},
			DoctorChecks:         []string{"plugin-registration"}, UninstallBehavior: "remove-owned-registration",
		},
	}
}

// Defaults returns the IDs selected by default in stable order.
func Defaults(registry []model.Component) []string {
	var ids []string
	for _, component := range registry {
		if component.Default {
			ids = append(ids, component.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

// Resolve validates the registry and returns the selected components with all
// transitive requirements in dependency-first deterministic order.
func Resolve(registry []model.Component, selected []string) ([]model.Component, error) {
	byID := make(map[string]model.Component, len(registry))
	destinations := make(map[string]string)
	for _, component := range registry {
		if component.ID == "" {
			return nil, fmt.Errorf("component ID is required")
		}
		if _, exists := byID[component.ID]; exists {
			return nil, fmt.Errorf("duplicate component %q", component.ID)
		}
		if component.Type == model.ComponentSkill || component.Type == model.ComponentMCP {
			return nil, fmt.Errorf("component %q uses reserved unsupported type %q", component.ID, component.Type)
		}
		for _, destination := range component.ManagedDestinations {
			if owner, exists := destinations[destination]; exists {
				return nil, fmt.Errorf("components %q and %q both manage %q", owner, component.ID, destination)
			}
			destinations[destination] = component.ID
		}
		byID[component.ID] = component
	}

	wanted := make(map[string]bool)
	visiting := make(map[string]bool)
	visited := make(map[string]bool)
	var ordered []model.Component
	var visit func(string) error
	visit = func(id string) error {
		component, exists := byID[id]
		if !exists {
			return fmt.Errorf("unknown component %q", id)
		}
		wanted[id] = true
		if visiting[id] {
			return fmt.Errorf("component dependency cycle includes %q", id)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		requires := append([]string(nil), component.Requires...)
		sort.Strings(requires)
		for _, required := range requires {
			if err := visit(required); err != nil {
				return fmt.Errorf("component %q requires %q: %w", id, required, err)
			}
		}
		visiting[id] = false
		visited[id] = true
		ordered = append(ordered, component)
		return nil
	}

	ids := append([]string(nil), selected...)
	sort.Strings(ids)
	for _, id := range ids {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	for id := range wanted {
		for _, conflict := range byID[id].Conflicts {
			if wanted[conflict] {
				return nil, fmt.Errorf("components %q and %q conflict", id, conflict)
			}
			if _, exists := byID[conflict]; !exists {
				return nil, fmt.Errorf("component %q declares unknown conflict %q", id, conflict)
			}
		}
	}
	return ordered, nil
}
