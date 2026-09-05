package openspec

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/SBakolis/north/internal/orchestration"
)

type fixtureRunner struct {
	status   func(string) []byte
	commands [][]string
	version  error
}

func (r *fixtureRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, error) {
	r.commands = append(r.commands, append([]string{name}, args...))
	if !reflect.DeepEqual([]string{name, args[0]}, []string{"npx", "openspec"}) {
		return nil, errors.New("provider did not invoke npx openspec")
	}
	switch args[1] {
	case "--version":
		return []byte("1.2.3\n"), r.version
	case "status":
		return r.status(args[3]), nil
	case "validate":
		return []byte(`{"items":[{"valid":true,"issues":[]}]}`), nil
	case "show":
		return []byte(`{"id":"add-login","title":"Add login","why":"Users need access.","deltas":[]}`), nil
	default:
		return nil, errors.New("unexpected command")
	}
}

func TestLoadDefaultFixture(t *testing.T) {
	root := copyFixture(t, "default")
	runner := &fixtureRunner{status: func(string) []byte {
		return statusJSON(t, root, "add-login", []artifactFixture{
			{id: "proposal", path: "proposal.md"},
			{id: "specs", path: "specs/auth/spec.md"},
			{id: "design", path: "design.md"},
			{id: "tasks", path: "tasks.md"},
		})
	}}
	p := New(runner)
	if ok, err := p.Detect(context.Background(), orchestration.ProjectContext{Root: filepath.Join(root, "internal")}); err != nil || !ok {
		t.Fatalf("Detect() = %v, %v", ok, err)
	}
	snapshot, err := p.Load(context.Background(), orchestration.KnowledgeRequest{ChangeID: "add-login"})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Change == nil || snapshot.Change.Title != "Add login" {
		t.Fatalf("change = %#v", snapshot.Change)
	}
	if len(snapshot.Requirements) != 1 || len(snapshot.AcceptanceCriteria) != 1 || len(snapshot.DesignDecisions) != 1 {
		t.Fatalf("normalized snapshot = %#v", snapshot)
	}
	if len(snapshot.Tasks) != 2 || !snapshot.Tasks[0].Completed || snapshot.Tasks[1].Completed {
		t.Fatalf("tasks = %#v", snapshot.Tasks)
	}
	if len(snapshot.RawArtifacts) != 4 || len(snapshot.RawArtifacts[0].SHA256) != 64 {
		t.Fatalf("artifacts = %#v", snapshot.RawArtifacts)
	}
}

func TestLoadCustomSchemaUsesMachineArtifactOrder(t *testing.T) {
	root := copyFixture(t, "custom")
	runner := &fixtureRunner{status: func(string) []byte {
		return statusJSON(t, root, "research-login", []artifactFixture{
			{id: "research", path: "notes/discovery.md"},
			{id: "delivery", path: "work/implementation.checklist"},
		})
	}}
	p := New(runner)
	if ok, err := p.Detect(context.Background(), orchestration.ProjectContext{Root: root}); err != nil || !ok {
		t.Fatalf("Detect() = %v, %v", ok, err)
	}
	snapshot, err := p.Load(context.Background(), orchestration.KnowledgeRequest{ChangeID: "research-login"})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{snapshot.RawArtifacts[0].Path, snapshot.RawArtifacts[1].Path}
	want := []string{"openspec/changes/research-login/notes/discovery.md", "openspec/changes/research-login/work/implementation.checklist"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("artifact order = %v, want %v", got, want)
	}
	if len(snapshot.Tasks) != 1 || snapshot.Tasks[0].ID != "delivery:task:1" {
		t.Fatalf("custom tasks = %#v", snapshot.Tasks)
	}
}

func TestLoadRejectsArtifactPathEscape(t *testing.T) {
	root := copyFixture(t, "escape")
	outside := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []string{outside, filepath.Join(root, "openspec", "changes", "escape", "linked.md")} {
		t.Run(filepath.Base(candidate), func(t *testing.T) {
			if strings.HasSuffix(candidate, "linked.md") {
				if err := os.Symlink(outside, candidate); err != nil {
					t.Fatal(err)
				}
				defer os.Remove(candidate)
			}
			runner := &fixtureRunner{status: func(string) []byte {
				return rawStatusJSON(t, root, "escape", candidate)
			}}
			p := New(runner)
			if ok, err := p.Detect(context.Background(), orchestration.ProjectContext{Root: root}); err != nil || !ok {
				t.Fatalf("Detect() = %v, %v", ok, err)
			}
			if _, err := p.Load(context.Background(), orchestration.KnowledgeRequest{ChangeID: "escape"}); err == nil || !strings.Contains(err.Error(), "escapes project root") {
				t.Fatalf("Load() error = %v", err)
			}
		})
	}
}

func TestDetectClearsAbsentDiagnostics(t *testing.T) {
	root := copyFixture(t, "default")
	runner := &fixtureRunner{version: errors.New("missing")}
	p := New(runner)
	if _, err := p.Detect(context.Background(), orchestration.ProjectContext{Root: root}); err == nil || len(p.Detection().Diagnostics) == 0 {
		t.Fatalf("expected executable diagnostic: %#v, %v", p.Detection(), err)
	}
	if ok, err := p.Detect(context.Background(), orchestration.ProjectContext{Root: t.TempDir()}); err != nil || ok {
		t.Fatalf("absent Detect() = %v, %v", ok, err)
	}
	if got := p.Detection(); len(got.Diagnostics) != 0 || got.Root != "" {
		t.Fatalf("stale detection = %#v", got)
	}
}

type artifactFixture struct{ id, path string }

func statusJSON(t *testing.T, root, change string, artifacts []artifactFixture) []byte {
	t.Helper()
	changeRoot := filepath.Join(root, "openspec", "changes", change)
	value := statusOutput{ChangeName: change, ChangeRoot: changeRoot, ArtifactPaths: map[string]artifactPathSummary{}}
	for _, artifact := range artifacts {
		path := filepath.Join(changeRoot, filepath.FromSlash(artifact.path))
		value.Artifacts = append(value.Artifacts, artifactStatus{ID: artifact.id, OutputPath: artifact.path, Status: "done"})
		value.ArtifactPaths[artifact.id] = artifactPathSummary{OutputPath: artifact.path, ResolvedOutputPath: path, ExistingOutputPaths: []string{path}}
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func rawStatusJSON(t *testing.T, root, change, path string) []byte {
	t.Helper()
	value := statusOutput{
		ChangeName: change,
		ChangeRoot: filepath.Join(root, "openspec", "changes", change),
		Artifacts:  []artifactStatus{{ID: "artifact", OutputPath: path, Status: "done"}},
		ArtifactPaths: map[string]artifactPathSummary{
			"artifact": {OutputPath: path, ResolvedOutputPath: path, ExistingOutputPaths: []string{path}},
		},
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	source := filepath.Join("tests", "fixtures", name)
	destination := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == "." {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
	return destination
}
