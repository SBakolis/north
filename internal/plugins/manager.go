// Package plugins manages North's small, explicitly allowed set of optional
// OpenCode plugins without taking ownership of user-managed registrations.
package plugins

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
)

const (
	CodexMeter = "opencode-codex-meter"
	OpenLoop   = "@sbakolis/open-loop"
)

// Specification describes an optional plugin. Enabled is deliberately false
// for every plugin in Catalog; activation must be explicit.
type Specification struct {
	Module      string
	Enabled     bool
	Description string
}

var catalog = []Specification{
	{Module: CodexMeter, Enabled: false, Description: "Codex usage meter"},
	{Module: OpenLoop, Enabled: false, Description: "Goal and time based loops"},
}

func Specifications() []Specification {
	return append([]Specification(nil), catalog...)
}

type ConfigRole string

const (
	RoleGlobal ConfigRole = "global"
	RoleServer ConfigRole = "server"
	RoleTUI    ConfigRole = "tui"
)

type CandidateConfig struct {
	Path string
	Role ConfigRole
}

type Paths struct {
	Global []string
	Server []string
	TUI    []string
}

func (p Paths) Candidates(module string) []CandidateConfig {
	var out []CandidateConfig
	add := func(role ConfigRole, paths []string) {
		for _, path := range paths {
			if path != "" {
				out = append(out, CandidateConfig{Path: path, Role: role})
			}
		}
	}
	add(RoleGlobal, p.Global)
	add(RoleServer, p.Server)
	if module == CodexMeter {
		add(RoleTUI, p.TUI)
	}
	return out
}

type Runner interface {
	Run(context.Context, string, ...string) error
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

type Files interface {
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte, fs.FileMode) error
}

type OSFiles struct{}

func (OSFiles) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func (OSFiles) WriteFile(path string, data []byte, mode fs.FileMode) error {
	return os.WriteFile(path, data, mode)
}

type Snapshot struct {
	Path   string
	Role   ConfigRole
	Exists bool
	Data   []byte
}

type RegistrationMethod string

const (
	StringRegistration RegistrationMethod = "string"
	TupleRegistration  RegistrationMethod = "tuple"
)

type Registration struct {
	Path        string
	Role        ConfigRole
	Module      string
	Method      RegistrationMethod
	Fingerprint string
	start, end  int
	arrayStart  int
}

type Ownership struct {
	Path        string             `json:"path"`
	Module      string             `json:"module"`
	Method      RegistrationMethod `json:"method"`
	Fingerprint string             `json:"fingerprint"`
}

type Action struct {
	Module       string
	Before       []Snapshot
	After        []Snapshot
	PreExisting  bool
	Owned        []Ownership
	Verification Verification
}

type Manager struct {
	runner Runner
	files  Files
	paths  Paths
}

func NewManager(runner Runner, files Files, paths Paths) *Manager {
	if runner == nil {
		runner = ExecRunner{}
	}
	if files == nil {
		files = OSFiles{}
	}
	return &Manager{runner: runner, files: files, paths: paths}
}

func Supported(module string) bool { return module == CodexMeter || module == OpenLoop }

func (m *Manager) SnapshotCandidates(module string) ([]Snapshot, error) {
	if !Supported(module) {
		return nil, fmt.Errorf("unsupported plugin %q", module)
	}
	var snapshots []Snapshot
	for _, candidate := range m.paths.Candidates(module) {
		data, err := m.files.ReadFile(candidate.Path)
		if errors.Is(err, fs.ErrNotExist) {
			snapshots = append(snapshots, Snapshot{Path: candidate.Path, Role: candidate.Role})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("snapshot %s: %w", candidate.Path, err)
		}
		snapshots = append(snapshots, Snapshot{Path: candidate.Path, Role: candidate.Role, Exists: true, Data: append([]byte(nil), data...)})
	}
	return snapshots, nil
}

// Enable snapshots every candidate before honoring the action contract. It
// never invokes the installer for a registration that is already user-owned.
func (m *Manager) Enable(ctx context.Context, module string) (Action, error) {
	before, err := m.SnapshotCandidates(module)
	action := Action{Module: module, Before: before}
	if err != nil {
		return action, err
	}
	beforeRegs, err := registrations(before, module)
	if err != nil {
		return action, err
	}
	if len(beforeRegs) > 0 {
		action.PreExisting = true
		action.After = cloneSnapshots(before)
		action.Verification = verify(module, beforeRegs)
		return action, nil
	}
	if err := m.runner.Run(ctx, "opencode", "plugin", module, "--global"); err != nil {
		return action, fmt.Errorf("install %s: %w", module, err)
	}
	after, err := m.SnapshotCandidates(module)
	action.After = after
	if err != nil {
		return action, err
	}
	afterRegs, err := registrations(after, module)
	if err != nil {
		return action, err
	}
	for _, reg := range afterRegs {
		action.Owned = append(action.Owned, Ownership{Path: reg.Path, Module: module, Method: reg.Method, Fingerprint: reg.Fingerprint})
	}
	action.Verification = verify(module, afterRegs)
	if len(afterRegs) == 0 {
		return action, fmt.Errorf("%s installer completed without a candidate config registration", module)
	}
	return action, nil
}

func cloneSnapshots(in []Snapshot) []Snapshot {
	out := make([]Snapshot, len(in))
	copy(out, in)
	for i := range out {
		out[i].Data = append([]byte(nil), in[i].Data...)
	}
	return out
}

func registrations(snapshots []Snapshot, module string) ([]Registration, error) {
	if !Supported(module) {
		return nil, fmt.Errorf("unsupported plugin %q", module)
	}
	var all []Registration
	for _, snapshot := range snapshots {
		if !snapshot.Exists {
			continue
		}
		found, err := DetectRegistrations(snapshot.Path, snapshot.Role, snapshot.Data, module)
		if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", snapshot.Path, err)
		}
		all = append(all, found...)
	}
	return all, nil
}

// DetectRegistrations recognizes only exact string entries and tuples whose
// first element is the exact module string.
func DetectRegistrations(path string, role ConfigRole, data []byte, module string) ([]Registration, error) {
	if !Supported(module) {
		return nil, fmt.Errorf("unsupported plugin %q", module)
	}
	root, _, err := parseJSONC(data)
	if err != nil {
		return nil, err
	}
	var found []Registration
	for _, array := range pluginArrays(root) {
		for _, entry := range array.elements {
			method := RegistrationMethod("")
			if entry.kind == 's' && entry.text == module {
				method = StringRegistration
			}
			if entry.kind == '[' && len(entry.elements) > 0 && entry.elements[0].kind == 's' && entry.elements[0].text == module {
				method = TupleRegistration
			}
			if method != "" {
				found = append(found, Registration{Path: path, Role: role, Module: module, Method: method, Fingerprint: fingerprint(data, entry), start: entry.start, end: entry.end, arrayStart: array.start})
			}
		}
	}
	return found, nil
}

// RemoveOwned removes unchanged registrations recorded after Enable. Changed
// tuples and string-to-tuple customizations intentionally cease to be owned.
func (m *Manager) RemoveOwned(owned []Ownership) error {
	byPath := map[string][]Ownership{}
	for _, item := range owned {
		if !Supported(item.Module) {
			return fmt.Errorf("unsupported plugin %q", item.Module)
		}
		byPath[item.Path] = append(byPath[item.Path], item)
	}
	for path, claims := range byPath {
		data, err := m.files.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("owned plugin configuration %s is missing", path)
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		updated, removed, err := removeClaims(data, path, claims)
		if err != nil {
			return fmt.Errorf("edit %s: %w", path, err)
		}
		if removed != len(claims) {
			return fmt.Errorf("owned plugin registration at %s was changed; leaving it untouched", path)
		}
		if removed > 0 {
			if err := m.files.WriteFile(path, updated, 0o600); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}
		}
	}
	return nil
}

type span struct{ start, end int }

func removeClaims(data []byte, path string, claims []Ownership) ([]byte, int, error) {
	out := append([]byte(nil), data...)
	removed := 0
	for _, claim := range claims {
		root, tokens, err := parseJSONC(out)
		if err != nil {
			return nil, removed, err
		}
		matched := false
		for _, array := range pluginArrays(root) {
			for _, entry := range array.elements {
				method, module := entryIdentity(entry)
				if claim.Path == path && claim.Module == module && claim.Method == method && claim.Fingerprint == fingerprint(out, entry) {
					s := removalSpan(tokens, array, entry)
					out = append(out[:s.start], out[s.end:]...)
					removed++
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
	}
	return out, removed, nil
}

func entryIdentity(entry node) (RegistrationMethod, string) {
	if entry.kind == 's' {
		return StringRegistration, entry.text
	}
	if entry.kind == '[' && len(entry.elements) > 0 && entry.elements[0].kind == 's' {
		return TupleRegistration, entry.elements[0].text
	}
	return "", ""
}

func removalSpan(tokens []token, array, entry node) span {
	for _, t := range tokens {
		if t.start >= entry.end && t.end < array.end && t.kind == ',' {
			return span{entry.start, t.end}
		}
		if t.start >= entry.end {
			break
		}
	}
	var comma token
	for _, t := range tokens {
		if t.start <= array.start {
			continue
		}
		if t.end <= entry.start && t.kind == ',' {
			comma = t
		}
		if t.start >= entry.start {
			break
		}
	}
	if comma.kind == ',' {
		return span{comma.start, entry.end}
	}
	return span{entry.start, entry.end}
}
