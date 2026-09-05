package config

import (
	"reflect"
	"strings"
	"testing"
)

const generatedYAML = `apiVersion: north/v1alpha1
kind: NorthConfig
installation:
  scope: global
parallelization:
  enabled: true
  runtime: opencode-cli
  isolation: git-worktree
  integration: progressive
  maxParallel: 4
  failFast: false
  autoIntegrateTarget: false
knowledge:
  provider: openspec
plugins:
  opencode-codex-meter:
    enabled: true
  "@sbakolis/open-loop":
    enabled: false
`

func TestParseGeneratedConfig(t *testing.T) {
	c, warnings, err := Parse([]byte(generatedYAML))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if c.APIVersion != APIVersionV1Alpha1 || c.Kind != KindNorthConfig || c.Installation.Scope != "global" {
		t.Fatalf("header = %+v", c)
	}
	if !c.Parallelization.Enabled || c.Parallelization.MaxParallel != 4 || c.Parallelization.FailFast || c.Parallelization.AutoIntegrateTarget {
		t.Fatalf("parallelization = %+v", c.Parallelization)
	}
	if c.Knowledge.Provider != "openspec" || !c.Plugins.OpenCodeCodexMeter.Enabled || c.Plugins.OpenLoop.Enabled {
		t.Fatalf("optional settings = %+v %+v", c.Knowledge, c.Plugins)
	}
}

func TestMigrateLegacyUnversionedConfig(t *testing.T) {
	legacy := []byte(`installation:
  scope: global
parallelization:
  enabled: true
  runtime: opencode-cli
  isolation: git-worktree
  integration: progressive
  maxParallel: 2
knowledge:
  provider: none
plugins: {}
`)
	migrated, err := Migrate(legacy)
	if err != nil {
		t.Fatal(err)
	}
	configuration, _, err := Parse(migrated)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.APIVersion != APIVersionV1Alpha1 || configuration.Kind != KindNorthConfig {
		t.Fatalf("configuration = %+v", configuration)
	}
}

func TestParseDefaultsAndPreservesExplicitFalse(t *testing.T) {
	c, warnings, err := ParseYAML([]byte("apiVersion: north/v1alpha1\nkind: NorthConfig\nparallelization:\n  enabled: false\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || c.Parallelization.Enabled {
		t.Fatalf("result = %+v, warnings = %+v", c, warnings)
	}
	want := Default()
	want.Parallelization.Enabled = false
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("config = %+v, want %+v", c, want)
	}
}

func TestUnknownKeysWarnOrFailStrictlyWithoutLosingKnownValues(t *testing.T) {
	data := []byte(`{
  "apiVersion": "north/v1alpha1",
  "kind": "NorthConfig",
  "future": true,
  "parallelization": {"maxParallel": 7, "later": "value"},
  "plugins": {"opencode-codex-meter": {"enabled": true, "option": 1}}
}`)
	c, warnings, err := ParseJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{"future", "parallelization.later", "plugins.opencode-codex-meter.option"}
	if got := warningPaths(warnings); !reflect.DeepEqual(got, wantPaths) {
		t.Fatalf("warning paths = %v, want %v", got, wantPaths)
	}
	if c.Parallelization.MaxParallel != 7 || !c.Plugins.OpenCodeCodexMeter.Enabled {
		t.Fatalf("known values lost: %+v", c)
	}

	strict, strictWarnings, err := ParseJSON(data, Options{Strict: true})
	if err == nil || !strings.Contains(err.Error(), "parallelization.later") {
		t.Fatalf("strict error = %v", err)
	}
	if strict.Parallelization.MaxParallel != 7 || len(strictWarnings) != 3 {
		t.Fatalf("strict parse did not preserve result: %+v, %+v", strict, strictWarnings)
	}
}

func TestParseRejectsEnumsBoundsAndTypes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"version", "apiVersion: north/v1\nkind: NorthConfig\n", "apiVersion"},
		{"scope", "apiVersion: north/v1alpha1\nkind: NorthConfig\ninstallation: {scope: project}\n", "installation.scope"},
		{"runtime", "apiVersion: north/v1alpha1\nkind: NorthConfig\nparallelization: {runtime: other}\n", "parallelization.runtime"},
		{"isolation", "apiVersion: north/v1alpha1\nkind: NorthConfig\nparallelization: {isolation: none}\n", "parallelization.isolation"},
		{"integration", "apiVersion: north/v1alpha1\nkind: NorthConfig\nparallelization: {integration: atomic}\n", "parallelization.integration"},
		{"minimum", "apiVersion: north/v1alpha1\nkind: NorthConfig\nparallelization: {maxParallel: 0}\n", "between 1 and 64"},
		{"maximum", "apiVersion: north/v1alpha1\nkind: NorthConfig\nparallelization: {maxParallel: 65}\n", "between 1 and 64"},
		{"provider", "apiVersion: north/v1alpha1\nkind: NorthConfig\nknowledge: {provider: custom}\n", "knowledge.provider"},
		{"type", "apiVersion: north/v1alpha1\nkind: NorthConfig\nparallelization: {enabled: 1}\n", "cannot unmarshal"},
		{"required version", "kind: NorthConfig\n", "apiVersion"},
		{"required kind", "apiVersion: north/v1alpha1\n", "kind"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseYAML([]byte(tt.body))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestStableMarshalRoundTrip(t *testing.T) {
	c, _, err := Parse([]byte(generatedYAML))
	if err != nil {
		t.Fatal(err)
	}
	yamlOne, err := MarshalYAML(c)
	if err != nil {
		t.Fatal(err)
	}
	yamlTwo, err := MarshalYAML(c)
	if err != nil || string(yamlOne) != string(yamlTwo) {
		t.Fatalf("unstable YAML: %v\n%s\n%s", err, yamlOne, yamlTwo)
	}
	jsonOne, err := MarshalJSON(c)
	if err != nil {
		t.Fatal(err)
	}
	jsonTwo, err := MarshalJSON(c)
	if err != nil || string(jsonOne) != string(jsonTwo) {
		t.Fatalf("unstable JSON: %v\n%s\n%s", err, jsonOne, jsonTwo)
	}
	for name, data := range map[string][]byte{"yaml": yamlOne, "json": jsonOne} {
		roundTrip, warnings, err := Parse(data)
		if err != nil || len(warnings) != 0 || !reflect.DeepEqual(roundTrip, c) {
			t.Fatalf("%s round trip = %+v, %+v, %v", name, roundTrip, warnings, err)
		}
	}
}

func TestRejectsMultipleDocumentsAndValues(t *testing.T) {
	if _, _, err := ParseYAML([]byte(generatedYAML + "---\n{}\n")); err == nil {
		t.Fatal("multiple YAML documents accepted")
	}
	if _, _, err := ParseJSON([]byte(`{"apiVersion":"north/v1alpha1","kind":"NorthConfig"} {}`)); err == nil {
		t.Fatal("multiple JSON values accepted")
	}
	if _, _, err := ParseYAML(nil); err == nil {
		t.Fatal("empty YAML accepted")
	}
}

func warningPaths(warnings []Warning) []string {
	paths := make([]string, len(warnings))
	for i, warning := range warnings {
		paths[i] = warning.Path
	}
	return paths
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(generatedYAML), false)
	f.Add([]byte(`{"apiVersion":"north/v1alpha1","kind":"NorthConfig","unknown":true}`), true)
	f.Add([]byte("skills:\n  token: secret\n"), false)
	f.Fuzz(func(t *testing.T, data []byte, strict bool) {
		c, _, err := Parse(data, Options{Strict: strict})
		if err != nil {
			return
		}
		yamlData, err := MarshalYAML(c)
		if err != nil {
			t.Fatal(err)
		}
		if _, warnings, err := ParseYAML(yamlData, Options{Strict: true}); err != nil || len(warnings) != 0 {
			t.Fatalf("valid config did not round trip: warnings=%+v err=%v", warnings, err)
		}
		jsonData, err := MarshalJSON(c)
		if err != nil {
			t.Fatal(err)
		}
		if _, warnings, err := ParseJSON(jsonData, Options{Strict: true}); err != nil || len(warnings) != 0 {
			t.Fatalf("valid config did not round trip: warnings=%+v err=%v", warnings, err)
		}
	})
}
