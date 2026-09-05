package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var knownFields = map[string]map[string]struct{}{
	"":                fieldSet("apiVersion", "kind", "installation", "parallelization", "knowledge", "plugins"),
	"installation":    fieldSet("scope"),
	"parallelization": fieldSet("enabled", "runtime", "isolation", "integration", "maxParallel", "failFast", "autoIntegrateTarget"),
	"knowledge":       fieldSet("provider"),
	"plugins":         fieldSet("opencode-codex-meter", "@sbakolis/open-loop"),
	"plugins.*":       fieldSet("enabled"),
}

func fieldSet(names ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(names))
	for _, name := range names {
		result[name] = struct{}{}
	}
	return result
}

// Parse auto-detects JSON objects; all other input is decoded as YAML.
func Parse(data []byte, options ...Options) (Config, []Warning, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return Config{}, nil, fmt.Errorf("North configuration is empty")
	}
	if trimmed[0] == '{' {
		return ParseJSON(trimmed, options...)
	}
	return ParseYAML(trimmed, options...)
}

// Migrate upgrades the legacy unversioned alpha configuration to the current schema.
// It returns normalized YAML suitable for an explicit ownership-safe rewrite.
func Migrate(data []byte) ([]byte, error) {
	var object map[string]any
	if err := yaml.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("decode configuration for migration: %w", err)
	}
	version, _ := object["apiVersion"].(string)
	if version != "" && version != APIVersionV1Alpha1 {
		return nil, fmt.Errorf("unsupported configuration apiVersion %q", version)
	}
	if version == "" {
		object["apiVersion"] = APIVersionV1Alpha1
	}
	if kind, _ := object["kind"].(string); kind == "" {
		object["kind"] = KindNorthConfig
	}
	migrated, err := yaml.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("encode migrated configuration: %w", err)
	}
	if _, _, err := Parse(migrated); err != nil {
		return nil, fmt.Errorf("validate migrated configuration: %w", err)
	}
	return migrated, nil
}

func ParseJSON(data []byte, options ...Options) (Config, []Warning, error) {
	var raw any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return Config{}, nil, fmt.Errorf("decode JSON North configuration: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Config{}, nil, err
	}
	object, ok := raw.(map[string]any)
	if !ok {
		return Config{}, nil, fmt.Errorf("decode JSON North configuration: top-level value must be an object")
	}
	warnings := unknownJSON(object, "")
	c := decodeDefaults()
	decoder = json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&c); err != nil {
		return Config{}, warnings, fmt.Errorf("decode JSON North configuration: %w", err)
	}
	return finishParse(c, warnings, parseOptions(options))
}

func ParseYAML(data []byte, options ...Options) (Config, []Warning, error) {
	var document yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&document); err != nil {
		return Config{}, nil, fmt.Errorf("decode YAML North configuration: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, nil, fmt.Errorf("decode YAML North configuration: multiple documents")
		}
		return Config{}, nil, fmt.Errorf("decode YAML North configuration: %w", err)
	}
	if len(document.Content) == 0 {
		return Config{}, nil, fmt.Errorf("decode YAML North configuration: document is empty")
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return Config{}, nil, fmt.Errorf("decode YAML North configuration: top-level value must be an object")
	}
	warnings := unknownYAML(root, "")
	c := decodeDefaults()
	if err := root.Decode(&c); err != nil {
		return Config{}, warnings, fmt.Errorf("decode YAML North configuration: %w", err)
	}
	return finishParse(c, warnings, parseOptions(options))
}

func finishParse(c Config, warnings []Warning, options Options) (Config, []Warning, error) {
	if options.Strict && len(warnings) != 0 {
		problems := make([]string, len(warnings))
		for i, warning := range warnings {
			problems[i] = warning.Message
		}
		return c, warnings, &ValidationError{Problems: problems}
	}
	if err := Validate(c); err != nil {
		return c, warnings, err
	}
	return c, warnings, nil
}

func decodeDefaults() Config {
	c := Default()
	c.APIVersion = ""
	c.Kind = ""
	return c
}

func parseOptions(options []Options) Options {
	if len(options) == 0 {
		return Options{}
	}
	return options[0]
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode JSON North configuration: multiple values")
		}
		return fmt.Errorf("decode JSON North configuration: %w", err)
	}
	return nil
}

func unknownJSON(object map[string]any, path string) []Warning {
	var warnings []Warning
	for name, value := range object {
		if _, ok := knownFields[fieldContext(path)][name]; !ok {
			warnings = append(warnings, unknownWarning(joinPath(path, name)))
			continue
		}
		if child, ok := value.(map[string]any); ok {
			warnings = append(warnings, unknownJSON(child, joinPath(path, name))...)
		}
	}
	sortWarnings(warnings)
	return warnings
}

func unknownYAML(node *yaml.Node, path string) []Warning {
	var warnings []Warning
	for i := 0; i < len(node.Content); i += 2 {
		name, value := node.Content[i].Value, node.Content[i+1]
		if _, ok := knownFields[fieldContext(path)][name]; !ok {
			warnings = append(warnings, unknownWarning(joinPath(path, name)))
			continue
		}
		if value.Kind == yaml.MappingNode {
			warnings = append(warnings, unknownYAML(value, joinPath(path, name))...)
		}
	}
	sortWarnings(warnings)
	return warnings
}

func fieldContext(path string) string {
	if strings.HasPrefix(path, "plugins.") {
		return "plugins.*"
	}
	return path
}

func joinPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}

func unknownWarning(path string) Warning {
	return Warning{Code: "unknown-key", Path: path, Message: fmt.Sprintf("unknown configuration key %q", path)}
}

func sortWarnings(warnings []Warning) {
	sort.Slice(warnings, func(i, j int) bool { return warnings[i].Path < warnings[j].Path })
}

// MarshalJSON returns stable, indented JSON for a validated normalized config.
func MarshalJSON(c Config) ([]byte, error) {
	if err := Validate(c); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// MarshalYAML returns stable YAML for a validated normalized config.
func MarshalYAML(c Config) ([]byte, error) {
	if err := Validate(c); err != nil {
		return nil, err
	}
	return yaml.Marshal(c)
}
