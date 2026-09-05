package components

import (
	"reflect"
	"strings"
	"testing"

	"github.com/SBakolis/north/internal/model"
)

func TestBuiltinRegistryResolvesDefaults(t *testing.T) {
	resolved, err := Resolve(BuiltinRegistry(), Defaults(BuiltinRegistry()))
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, component := range resolved {
		ids = append(ids, component.ID)
	}
	if want := []string{"core", "knowledge.none", "parallelization"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("resolved IDs = %v, want %v", ids, want)
	}
}

func TestBuiltinRegistryPluginMetadataIsExactAndOffByDefault(t *testing.T) {
	var plugins []model.Component
	for _, component := range BuiltinRegistry() {
		if component.Type == model.ComponentPlugin {
			plugins = append(plugins, component)
		}
	}
	if len(plugins) != 2 {
		t.Fatalf("plugin components = %#v", plugins)
	}
	if plugins[0].ID != "plugin.opencode-codex-meter" || plugins[0].Source != "opencode-codex-meter" || plugins[0].Default {
		t.Fatalf("codex meter component = %#v", plugins[0])
	}
	if plugins[1].ID != "plugin.open-loop" || plugins[1].Source != "@sbakolis/open-loop" || plugins[1].Default {
		t.Fatalf("open-loop component = %#v", plugins[1])
	}
	for _, component := range plugins {
		if component.InstallStrategy != "opencode-plugin" || component.UninstallBehavior != "remove-owned-registration" {
			t.Fatalf("plugin lifecycle metadata = %#v", component)
		}
	}
}

func TestResolveAddsRequirementsBeforeDependent(t *testing.T) {
	registry := []model.Component{
		{ID: "feature", Requires: []string{"core"}},
		{ID: "core"},
	}
	resolved, err := Resolve(registry, []string{"feature"})
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{resolved[0].ID, resolved[1].ID}; !reflect.DeepEqual(got, []string{"core", "feature"}) {
		t.Fatalf("resolution order = %v", got)
	}
}

func TestResolveRejectsInvalidRegistryAndSelection(t *testing.T) {
	tests := []struct {
		name     string
		registry []model.Component
		selected []string
		contains string
	}{
		{"unknown selection", []model.Component{{ID: "core"}}, []string{"missing"}, "unknown component"},
		{"missing requirement", []model.Component{{ID: "a", Requires: []string{"missing"}}}, []string{"a"}, "requires"},
		{"cycle", []model.Component{{ID: "a", Requires: []string{"b"}}, {ID: "b", Requires: []string{"a"}}}, []string{"a"}, "cycle"},
		{"conflict", []model.Component{{ID: "a", Conflicts: []string{"b"}}, {ID: "b"}}, []string{"a", "b"}, "conflict"},
		{"duplicate destination", []model.Component{{ID: "a", ManagedDestinations: []string{"same"}}, {ID: "b", ManagedDestinations: []string{"same"}}}, []string{"a"}, "both manage"},
		{"reserved skill", []model.Component{{ID: "skill.x", Type: model.ComponentSkill}}, []string{"skill.x"}, "reserved unsupported"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Resolve(test.registry, test.selected)
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("error = %v, want containing %q", err, test.contains)
			}
		})
	}
}
