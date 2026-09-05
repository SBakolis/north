package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

func FuzzJSONCRegistrationRemoval(f *testing.F) {
	f.Add([]byte(`{"plugin":["opencode-codex-meter"]}`))
	f.Add([]byte(`{/* comment */"plugin":[["@sbakolis/open-loop",{"enabled":true}],],}`))
	f.Add([]byte(`not jsonc`))
	f.Fuzz(func(t *testing.T, data []byte) {
		registrations, err := DetectRegistrations("config.jsonc", RoleGlobal, data, CodexMeter)
		if err != nil {
			return
		}
		claims := make([]Ownership, 0, len(registrations))
		for _, registration := range registrations {
			claims = append(claims, Ownership{Path: registration.Path, Module: registration.Module, Method: registration.Method, Fingerprint: registration.Fingerprint})
		}
		updated, removed, err := removeClaims(data, "config.jsonc", claims)
		if err != nil {
			t.Fatalf("remove detected claims: %v", err)
		}
		if removed != len(claims) {
			t.Fatalf("removed %d of %d detected claims", removed, len(claims))
		}
		if err := ValidateConfig(updated); err != nil {
			t.Fatalf("updated JSONC is invalid: %v", err)
		}
	})
}

func FuzzChangedRegistrationIsPreserved(f *testing.F) {
	f.Add("changed")
	f.Fuzz(func(t *testing.T, value string) {
		base := []byte(`{"plugin":[["opencode-codex-meter",{"value":"base"}]]}`)
		registrations, err := DetectRegistrations("config.jsonc", RoleGlobal, base, CodexMeter)
		if err != nil || len(registrations) != 1 {
			t.Fatalf("detect base registration: %v", err)
		}
		encoded, _ := json.Marshal(value)
		changed := []byte(`{"plugin":[["opencode-codex-meter",{"value":` + string(encoded) + `,"northFuzz":true}]]}`)
		claim := Ownership{Path: "config.jsonc", Module: CodexMeter, Method: registrations[0].Method, Fingerprint: registrations[0].Fingerprint}
		updated, removed, err := removeClaims(changed, "config.jsonc", []Ownership{claim})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(updated, changed) || removed != 0 {
			t.Fatalf("changed registration was removed: removed=%d", removed)
		}
	})
}

type fakeFiles struct {
	data   map[string][]byte
	reads  []string
	writes []string
}

func newFakeFiles(files map[string]string) *fakeFiles {
	f := &fakeFiles{data: map[string][]byte{}}
	for path, data := range files {
		f.data[path] = []byte(data)
	}
	return f
}

func (f *fakeFiles) ReadFile(path string) ([]byte, error) {
	f.reads = append(f.reads, path)
	data, ok := f.data[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

func (f *fakeFiles) WriteFile(path string, data []byte, _ fs.FileMode) error {
	f.writes = append(f.writes, path)
	f.data[path] = append([]byte(nil), data...)
	return nil
}

type runnerFunc func(context.Context, string, ...string) error

func (f runnerFunc) Run(ctx context.Context, name string, args ...string) error {
	return f(ctx, name, args...)
}

func (runnerFunc) ResolveVersion(context.Context, string) (string, error) { return "1.2.3", nil }

func TestCatalogIsAnExactDisabledAllowlist(t *testing.T) {
	want := []Specification{
		{Module: CodexMeter, Enabled: false, Description: "Codex usage meter"},
		{Module: OpenLoop, Enabled: false, Description: "Goal and time based loops"},
	}
	if got := Specifications(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Specifications = %#v, want %#v", got, want)
	}
	got := Specifications()
	got[0].Enabled = true
	if Specifications()[0].Enabled {
		t.Fatal("caller mutated catalog defaults")
	}
	if Supported("other") {
		t.Fatal("unexpected plugin is supported")
	}
}

func TestEnableSnapshotsAllCandidatesThenRunsExactCommand(t *testing.T) {
	files := newFakeFiles(map[string]string{
		"global.jsonc": "{\n  // keep\n  \"plugin\": []\n}\n",
		"tui.json":     "{\"plugin\": []}\n",
	})
	var command []string
	runner := runnerFunc(func(_ context.Context, name string, args ...string) error {
		if !reflect.DeepEqual(files.reads, []string{"global.jsonc", "server.json", "tui.json"}) {
			t.Fatalf("runner called before snapshots completed: reads %v", files.reads)
		}
		command = append([]string{name}, args...)
		files.data["global.jsonc"] = []byte("{\n  // keep\n  \"plugin\": [\"opencode-codex-meter@1.2.3\"]\n}\n")
		return nil
	})
	m := NewManager(runner, files, Paths{Global: []string{"global.jsonc"}, Server: []string{"server.json"}, TUI: []string{"tui.json"}})
	action, err := m.Enable(context.Background(), CodexMeter)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"opencode", "plugin", CodexMeter + "@1.2.3", "--global"}; !reflect.DeepEqual(command, want) {
		t.Fatalf("command = %v, want %v", command, want)
	}
	if action.Version != "1.2.3" || len(action.Before) != 3 || action.Before[1].Exists {
		t.Fatalf("unexpected snapshots: %#v", action.Before)
	}
	if len(action.Owned) != 1 || action.Owned[0].Method != StringRegistration {
		t.Fatalf("unexpected ownership: %#v", action.Owned)
	}
	if !action.Verification.GlobalOrServer || action.Verification.TUI {
		t.Fatalf("unexpected verification: %#v", action.Verification)
	}
	if got := action.Verification.Diagnostics; len(got) != 1 || got[0].Code != "codex_meter_tui_registration_missing" {
		t.Fatalf("diagnostics = %#v", got)
	}
}

func TestEnableDoesNotRunForPreExistingRegistration(t *testing.T) {
	files := newFakeFiles(map[string]string{"config.jsonc": `{ "plugin": [["@sbakolis/open-loop", {"bare_mode":"error"}]] }`})
	called := false
	m := NewManager(runnerFunc(func(context.Context, string, ...string) error { called = true; return nil }), files, Paths{Global: []string{"config.jsonc"}})
	action, err := m.Enable(context.Background(), OpenLoop)
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("runner called for a pre-existing registration")
	}
	if !action.PreExisting || len(action.Owned) != 0 {
		t.Fatalf("unexpected action: %#v", action)
	}
	if len(action.After) != 1 || string(action.After[0].Data) != string(action.Before[0].Data) {
		t.Fatal("pre-existing snapshots were not retained")
	}
}

func TestEnableRejectsUnsupportedBeforeReadingOrRunning(t *testing.T) {
	files := newFakeFiles(nil)
	called := false
	m := NewManager(runnerFunc(func(context.Context, string, ...string) error { called = true; return nil }), files, Paths{})
	if _, err := m.Enable(context.Background(), "evil-plugin"); err == nil {
		t.Fatal("expected error")
	}
	if called || len(files.reads) != 0 {
		t.Fatal("unsupported plugin caused side effects")
	}
}

func TestEnableReturnsRunnerFailureWithoutPostActionReads(t *testing.T) {
	files := newFakeFiles(map[string]string{"config.json": `{ "plugin": [] }`})
	m := NewManager(runnerFunc(func(context.Context, string, ...string) error { return errors.New("boom") }), files, Paths{Global: []string{"config.json"}})
	action, err := m.Enable(context.Background(), OpenLoop)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
	if len(action.Before) != 1 || len(action.After) != 0 || len(files.reads) != 1 {
		t.Fatalf("unexpected failed action: %#v, reads %v", action, files.reads)
	}
}

func TestDetectRegistrationsJSONCExactStringsAndTuples(t *testing.T) {
	data := []byte(`{
  // plugin registrations
  "plugin": [
    "opencode-codex-meter",
    ["opencode-codex-meter", {"mode": "compact"}],
    "opencode-codex-meter-extra",
    ["prefix-opencode-codex-meter", {}]
  ],
  "nested": {"plugin": ["opencode-codex-meter"]}
}`)
	regs, err := DetectRegistrations("config.jsonc", RoleGlobal, data, CodexMeter)
	if err != nil {
		t.Fatal(err)
	}
	if len(regs) != 2 || regs[0].Method != StringRegistration || regs[1].Method != TupleRegistration {
		t.Fatalf("registrations = %#v", regs)
	}
}

func TestDetectRegistrationsRecognizesPinnedPackageSpecs(t *testing.T) {
	data := []byte(`{"plugin":["opencode-codex-meter@1.2.3",["@sbakolis/open-loop@2.0.0-beta.1",{}]]}`)
	codex, err := DetectRegistrations("config.jsonc", RoleGlobal, data, CodexMeter)
	if err != nil || len(codex) != 1 || codex[0].Version != "1.2.3" {
		t.Fatalf("codex=%+v error=%v", codex, err)
	}
	loop, err := DetectRegistrations("config.jsonc", RoleGlobal, data, OpenLoop)
	if err != nil || len(loop) != 1 || loop[0].Version != "2.0.0-beta.1" {
		t.Fatalf("loop=%+v error=%v", loop, err)
	}
}

func TestEnableRejectsInconsistentPreExistingVersions(t *testing.T) {
	files := newFakeFiles(map[string]string{
		"one.json": `{"plugin":["opencode-codex-meter@1.0.0"]}`,
		"two.json": `{"plugin":["opencode-codex-meter@2.0.0"]}`,
	})
	manager := NewManager(runnerFunc(func(context.Context, string, ...string) error {
		t.Fatal("runner invoked")
		return nil
	}), files, Paths{Global: []string{"one.json", "two.json"}})
	if _, err := manager.Enable(context.Background(), CodexMeter); err == nil || !strings.Contains(err.Error(), "inconsistent") {
		t.Fatalf("error = %v", err)
	}
}

func TestDetectRegistrationsRejectsInvalidJSONC(t *testing.T) {
	if _, err := DetectRegistrations("bad.jsonc", RoleGlobal, []byte(`{"plugin":["x"`), OpenLoop); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestRemoveOwnedPreservesCommentsAndUnrelatedEntries(t *testing.T) {
	original := `{
  // root comment
  "plugin": [
    // this comment must survive
    "opencode-codex-meter",
    ["other", {"enabled": true}], // other comment
  ]
}
`
	files := newFakeFiles(map[string]string{"config.jsonc": original})
	regs, err := DetectRegistrations("config.jsonc", RoleGlobal, []byte(original), CodexMeter)
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(nil, files, Paths{})
	claim := Ownership{Path: regs[0].Path, Module: regs[0].Module, Method: regs[0].Method, Fingerprint: regs[0].Fingerprint}
	if err := m.RemoveOwned([]Ownership{claim}); err != nil {
		t.Fatal(err)
	}
	got := string(files.data["config.jsonc"])
	for _, retained := range []string{"// root comment", "// this comment must survive", `["other", {"enabled": true}]`, "// other comment"} {
		if !strings.Contains(got, retained) {
			t.Fatalf("removed retained content %q:\n%s", retained, got)
		}
	}
	if strings.Contains(got, `"opencode-codex-meter"`) {
		t.Fatalf("owned registration remains:\n%s", got)
	}
	if _, _, err := parseJSONC([]byte(got)); err != nil {
		t.Fatalf("result is invalid JSONC: %v\n%s", err, got)
	}
}

func TestRemoveOwnedHandlesLastEntry(t *testing.T) {
	original := `{"plugin":["other", "@sbakolis/open-loop"]}`
	files := newFakeFiles(map[string]string{"config.json": original})
	regs, _ := DetectRegistrations("config.json", RoleGlobal, []byte(original), OpenLoop)
	if err := NewManager(nil, files, Paths{}).RemoveOwned([]Ownership{{Path: "config.json", Module: OpenLoop, Method: regs[0].Method, Fingerprint: regs[0].Fingerprint}}); err != nil {
		t.Fatal(err)
	}
	if got := string(files.data["config.json"]); got != `{"plugin":["other"]}` {
		t.Fatalf("got %q", got)
	}
}

func TestRemoveOwnedHandlesAdjacentClaims(t *testing.T) {
	original := `{"plugin":["other","opencode-codex-meter","opencode-codex-meter"]}`
	files := newFakeFiles(map[string]string{"config.json": original})
	regs, _ := DetectRegistrations("config.json", RoleGlobal, []byte(original), CodexMeter)
	claims := []Ownership{
		{Path: "config.json", Module: CodexMeter, Method: regs[0].Method, Fingerprint: regs[0].Fingerprint},
		{Path: "config.json", Module: CodexMeter, Method: regs[1].Method, Fingerprint: regs[1].Fingerprint},
	}
	if err := NewManager(nil, files, Paths{}).RemoveOwned(claims); err != nil {
		t.Fatal(err)
	}
	if got := string(files.data["config.json"]); got != `{"plugin":["other"]}` {
		t.Fatalf("got %q", got)
	}
}

func TestRemoveOwnedPreservesCustomizedTuple(t *testing.T) {
	original := `{"plugin":[["@sbakolis/open-loop",{"bare_mode":"goal"}]]}`
	files := newFakeFiles(map[string]string{"config.json": original})
	regs, _ := DetectRegistrations("config.json", RoleGlobal, []byte(original), OpenLoop)
	claim := Ownership{Path: "config.json", Module: OpenLoop, Method: TupleRegistration, Fingerprint: regs[0].Fingerprint}
	files.data["config.json"] = []byte(`{"plugin":[["@sbakolis/open-loop",{"bare_mode":"error"}]]}`)
	if err := NewManager(nil, files, Paths{}).RemoveOwned([]Ownership{claim}); err == nil {
		t.Fatal("expected customized registration conflict")
	}
	if len(files.writes) != 0 {
		t.Fatal("customized tuple was rewritten")
	}
	if !strings.Contains(string(files.data["config.json"]), `"bare_mode":"error"`) {
		t.Fatal("customized tuple was removed")
	}
}

func TestRemoveOwnedPreservesReformattedTuple(t *testing.T) {
	original := `{"plugin":[["@sbakolis/open-loop",{"bare_mode":"goal"}]]}`
	files := newFakeFiles(map[string]string{"config.json": original})
	regs, _ := DetectRegistrations("config.json", RoleGlobal, []byte(original), OpenLoop)
	claim := Ownership{Path: "config.json", Module: OpenLoop, Method: TupleRegistration, Fingerprint: regs[0].Fingerprint}
	files.data["config.json"] = []byte(`{"plugin":[["@sbakolis/open-loop", { "bare_mode": "goal" }]]}`)
	if err := NewManager(nil, files, Paths{}).RemoveOwned([]Ownership{claim}); err == nil {
		t.Fatal("expected reformatted registration conflict")
	}
	if len(files.writes) != 0 {
		t.Fatal("reformatted tuple was treated as unchanged ownership")
	}
}

func TestRemoveOwnedPreservesStringConvertedToTuple(t *testing.T) {
	files := newFakeFiles(map[string]string{"config.json": `{"plugin":[["opencode-codex-meter",{}]]}`})
	claim := Ownership{Path: "config.json", Module: CodexMeter, Method: StringRegistration, Fingerprint: `"opencode-codex-meter"`}
	if err := NewManager(nil, files, Paths{}).RemoveOwned([]Ownership{claim}); err == nil {
		t.Fatal("expected registration method conflict")
	}
	if len(files.writes) != 0 {
		t.Fatal("converted tuple was rewritten")
	}
}

func TestVerifyCodexMeterChecksGlobalServerAndTUI(t *testing.T) {
	snapshots := []Snapshot{
		{Path: "server.json", Role: RoleServer, Exists: true, Data: []byte(`{"plugin":["opencode-codex-meter"]}`)},
		{Path: "tui.jsonc", Role: RoleTUI, Exists: true, Data: []byte(`{"plugin":[["opencode-codex-meter",{}]]}`)},
	}
	v, err := Verify(CodexMeter, snapshots)
	if err != nil {
		t.Fatal(err)
	}
	if !v.GlobalOrServer || !v.TUI || len(v.Diagnostics) != 0 {
		t.Fatalf("verification = %#v", v)
	}
}

func TestOpenLoopDuplicateAndMethodConflictDiagnostics(t *testing.T) {
	snapshots := []Snapshot{
		{Path: "a.json", Role: RoleGlobal, Exists: true, Data: []byte(`{"plugin":["@sbakolis/open-loop"]}`)},
		{Path: "b.jsonc", Role: RoleServer, Exists: true, Data: []byte(`{"plugin":[["@sbakolis/open-loop",{}]]}`)},
	}
	v, err := Verify(OpenLoop, snapshots)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"open_loop_duplicate_registration", "open_loop_conflicting_registration_methods"}
	var got []string
	for _, diagnostic := range v.Diagnostics {
		got = append(got, diagnostic.Code)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diagnostic codes = %v, want %v", got, want)
	}
}

func TestRestrictedOpenLoopConfigIsDataOnlyAndDefensivelyCopied(t *testing.T) {
	config := RestrictedOpenLoopConfig()
	want := append([]string{"plan"}, NorthAgentRestrictions()...)
	if !reflect.DeepEqual(config.RestrictedAgents, want) {
		t.Fatalf("config = %#v", config)
	}
	config.RestrictedAgents[0] = "changed"
	if RestrictedOpenLoopConfig().RestrictedAgents[0] != "plan" || NorthAgentRestrictions()[0] != "north-planner" {
		t.Fatal("caller mutated restricted agent declaration")
	}
	if strings.Contains(strings.ToLower(RestrictedAgentsDocumentation), "mcp") || strings.Contains(strings.ToLower(RestrictedAgentsDocumentation), "skill") {
		t.Fatal("documentation exposes forbidden surfaces")
	}
}
