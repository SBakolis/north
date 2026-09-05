package install

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveInstructionSource(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "AGENT.md")
	if err := os.WriteFile(legacy, []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := ResolveInstructionSource(dir, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if source.Path != legacy || filepath.Base(source.BackupPath) != "AGENT-backup.md" {
		t.Fatalf("unexpected source: %+v", source)
	}
}

func TestResolveInstructionSourceRequiresChoiceForDifferentFiles(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"AGENTS.md": "canonical", "AGENT.md": "legacy"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ResolveInstructionSource(dir, "", true); err == nil || !strings.Contains(err.Error(), "--agent-source") {
		t.Fatalf("error = %v", err)
	}
	source, err := ResolveInstructionSource(dir, "AGENT.md", true)
	if err != nil || filepath.Base(source.Path) != "AGENT.md" {
		t.Fatalf("source = %+v, error = %v", source, err)
	}
	if len(source.Originals) != 2 || string(source.Originals[0].Content) != "canonical" || string(source.Originals[1].Content) != "legacy" {
		t.Fatalf("originals = %+v", source.Originals)
	}
}

func TestComposeInstructionsPreservesOutsideManagedBlock(t *testing.T) {
	first, err := ComposeInstructions(nil, []byte("Keep this exactly."), []byte("# North\nfirst"))
	if err != nil {
		t.Fatal(err)
	}
	current := append([]byte("user prefix\n"), first...)
	updated, err := ComposeInstructions(current, nil, []byte("# North\nsecond"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(updated)
	if !strings.Contains(got, "user prefix") || !strings.Contains(got, "Keep this exactly.") || strings.Contains(got, "first") || !strings.Contains(got, "second") {
		t.Fatalf("unexpected composition:\n%s", got)
	}
}

func TestComposeInstructionsRejectsMalformedMarkers(t *testing.T) {
	if _, err := ComposeInstructions([]byte(managedBegin), nil, []byte("rules")); err == nil {
		t.Fatal("expected malformed marker error")
	}
}

func TestMergeInstructionsPreservesOutsideEditsAndConflictsInside(t *testing.T) {
	base, err := ComposeInstructions(nil, []byte("user\n"), []byte("managed v1"))
	if err != nil {
		t.Fatal(err)
	}
	current := append([]byte("later edit\n"), base...)
	merged, _, conflict, err := MergeInstructions(base, current, []byte("managed v2"))
	if err != nil || conflict || !bytes.Contains(merged, []byte("later edit")) || bytes.Contains(merged, []byte("managed v1")) {
		t.Fatalf("merged = %q conflict=%v error=%v", merged, conflict, err)
	}
	changed := bytes.ReplaceAll(current, []byte("managed v1"), []byte("user changed managed block"))
	_, proposed, conflict, err := MergeInstructions(base, changed, []byte("managed v2"))
	if err != nil || !conflict || !bytes.Contains(proposed, []byte("managed v2")) {
		t.Fatalf("proposed = %q conflict=%v error=%v", proposed, conflict, err)
	}
}

func TestTransactionRollback(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing")
	created := filepath.Join(dir, "created")
	if err := os.WriteFile(existing, []byte("before"), 0o640); err != nil {
		t.Fatal(err)
	}
	transaction := NewTransaction()
	if err := transaction.WriteFile(existing, []byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := transaction.WriteFile(created, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(existing)
	if err != nil || string(data) != "before" {
		t.Fatalf("restored data = %q, error = %v", data, err)
	}
	if _, err := os.Stat(created); !os.IsNotExist(err) {
		t.Fatalf("created path remains: %v", err)
	}
}

func FuzzManagedInstructionMarkers(f *testing.F) {
	for _, seed := range []string{"", "user text", managedBegin + "\nrules\n" + managedEnd, managedBegin, managedEnd, managedEnd + managedBegin, managedBegin + managedBegin + managedEnd} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, current []byte) {
		composed, err := ComposeInstructions(current, nil, []byte("managed"))
		if err != nil {
			return
		}
		if bytes.Count(composed, []byte(managedBegin)) != 1 || bytes.Count(composed, []byte(managedEnd)) != 1 {
			t.Fatalf("successful composition has invalid markers: %q", composed)
		}
		removed, err := RemoveManagedInstructions(composed)
		if err != nil {
			t.Fatalf("remove composed instructions: %v", err)
		}
		if bytes.Contains(removed, []byte(managedBegin)) || bytes.Contains(removed, []byte(managedEnd)) {
			t.Fatalf("managed marker remains after removal: %q", removed)
		}
	})
}
