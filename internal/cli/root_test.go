package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SBakolis/north/internal/install"
	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/orchestration"
	"github.com/SBakolis/north/internal/platform"
	"github.com/SBakolis/north/internal/testutil"
)

type plannerStub struct{}

func (plannerStub) CreatePlan(_ context.Context, input orchestration.PlanningInput) (model.ExecutionPlan, error) {
	return model.ExecutionPlan{
		APIVersion: model.APIVersionV1Alpha1, Kind: model.ExecutionPlanKind, Metadata: model.PlanMetadata{Name: "preview"},
		Spec: model.PlanSpec{
			Goal: input.Goal, BaseRef: input.BaseRef,
			Policy: model.PlanPolicy{MaxParallel: 1, MaxAttemptsPerStage: 1, FinalVerificationRequired: true},
			Stages: []model.Stage{{
				ID: "implementation", Title: "Implement", Description: "Implement", Agent: "north-worker", WriteScope: []string{"internal/**"},
				Acceptance: []model.AcceptanceCriterion{{ID: "tests", Type: "exec", Command: []string{"go", "test", "./..."}}},
			}},
		},
	}, nil
}

type missingAgentPlanner struct{ plannerStub }

func (missingAgentPlanner) CreatePlan(ctx context.Context, input orchestration.PlanningInput) (model.ExecutionPlan, error) {
	plan, err := (plannerStub{}).CreatePlan(ctx, input)
	plan.Spec.Stages[0].Agent = "unavailable"
	return plan, err
}

func TestRunVersion(t *testing.T) {
	var out bytes.Buffer
	if err := Run(&bytes.Buffer{}, &out, &bytes.Buffer{}, []string{"version"}, "0.1.0"); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "north 0.1.0\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	if err := Run(&bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}, []string{"unknown"}, "dev"); err == nil {
		t.Fatal("expected usage error")
	}
}

func TestRunInstallDryRunUsesRedirectedXDGPaths(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", root+"/config")
	t.Setenv("XDG_DATA_HOME", root+"/data")
	t.Setenv("XDG_STATE_HOME", root+"/state")
	t.Setenv("XDG_CACHE_HOME", root+"/cache")
	var output bytes.Buffer
	if err := Run(&bytes.Buffer{}, &output, &bytes.Buffer{}, []string{"install", "--dry-run", "--non-interactive"}, "0.1.0"); err != nil {
		t.Fatal(err)
	}
	if output.Len() == 0 {
		t.Fatal("expected dry-run operations")
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("dry-run wrote files: entries=%v error=%v", entries, err)
	}
}

func TestRunComponentsListHasNoSkillsOrMCPs(t *testing.T) {
	var output bytes.Buffer
	if err := Run(&bytes.Buffer{}, &output, &bytes.Buffer{}, []string{"components", "list"}, "dev"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "skill") || strings.Contains(output.String(), "mcp") {
		t.Fatalf("reserved component type exposed: %s", output.String())
	}
}

func TestRunInstallPromptsForDifferentInstructionSources(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	openCodeDir := filepath.Join(root, "config", "opencode")
	if err := os.MkdirAll(openCodeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(openCodeDir, "AGENTS.md"), []byte("canonical"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(openCodeDir, "AGENT.md"), []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	var prompt bytes.Buffer
	if err := Run(strings.NewReader("AGENT.md\n"), &bytes.Buffer{}, &prompt, []string{"install", "--dry-run"}, "dev"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt.String(), "Authoritative source") {
		t.Fatalf("prompt = %q", prompt.String())
	}
}

func TestInteractiveInstallPersistsSelectedComponents(t *testing.T) {
	root := t.TempDir()
	bin := t.TempDir()
	for _, name := range []string{"git", "opencode"} {
		testutil.WriteExecutable(t, bin, name, "exit 0")
	}
	testutil.WriteExecutable(t, bin, "npx", `[ "$1" = "openspec" ] && [ "$2" = "--version" ] && echo 1.0.0`)
	t.Setenv("PATH", bin)
	paths := platform.Paths{
		ConfigDir: filepath.Join(root, "config", "north"), DataDir: filepath.Join(root, "data", "north"),
		StateDir: filepath.Join(root, "state", "north"), CacheDir: filepath.Join(root, "cache", "north"),
		OpenCodeDir: filepath.Join(root, "config", "opencode"),
	}
	input := strings.NewReader("n\nopenspec\nn\nn\n")
	if err := runInstall(input, &bytes.Buffer{}, &bytes.Buffer{}, "install", nil, "dev", paths); err != nil {
		t.Fatal(err)
	}
	manifest, err := install.LoadManifest(install.ManifestPath(paths))
	if err != nil {
		t.Fatal(err)
	}
	if contains(manifest.Components, "parallelization") || !contains(manifest.Components, "knowledge.openspec") {
		t.Fatalf("components = %v", manifest.Components)
	}
}

func TestInstallRejectsUnavailableOpenSpecCommand(t *testing.T) {
	root := t.TempDir()
	bin := t.TempDir()
	testutil.WriteExecutable(t, bin, "git", "exit 0")
	testutil.WriteExecutable(t, bin, "opencode", "exit 0")
	testutil.WriteExecutable(t, bin, "npx", "echo unavailable >&2; exit 1")
	t.Setenv("PATH", bin)
	paths := platform.Paths{ConfigDir: filepath.Join(root, "config", "north"), DataDir: filepath.Join(root, "data", "north"), StateDir: filepath.Join(root, "state", "north"), CacheDir: filepath.Join(root, "cache", "north"), OpenCodeDir: filepath.Join(root, "config", "opencode")}
	err := runInstall(&bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}, "install", []string{"--non-interactive", "--knowledge", "openspec"}, "dev", paths)
	if err == nil || !strings.Contains(err.Error(), "npx openspec --version") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(install.ManifestPath(paths)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("manifest written after failed preflight: %v", statErr)
	}
}

func TestInstallRejectsEmptyOpenSpecVersion(t *testing.T) {
	root := t.TempDir()
	bin := t.TempDir()
	for _, name := range []string{"git", "opencode", "npx"} {
		testutil.WriteExecutable(t, bin, name, "exit 0")
	}
	t.Setenv("PATH", bin)
	paths := platform.Paths{ConfigDir: filepath.Join(root, "config", "north"), DataDir: filepath.Join(root, "data", "north"), StateDir: filepath.Join(root, "state", "north"), CacheDir: filepath.Join(root, "cache", "north"), OpenCodeDir: filepath.Join(root, "config", "opencode")}
	err := runInstall(&bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}, "install", []string{"--non-interactive", "--knowledge", "openspec"}, "dev", paths)
	if err == nil || !strings.Contains(err.Error(), "returned no version") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunDryRunShowsEffectiveHostCommandsWithoutWritingApproval(t *testing.T) {
	root := t.TempDir()
	planFile := filepath.Join(root, "plan.yaml")
	data := `apiVersion: north/v1alpha1
kind: ExecutionPlan
metadata:
  name: dry-run
spec:
  goal: preview
  baseRef: main
  policy:
    maxParallel: 1
    maxAttemptsPerStage: 1
    finalVerificationRequired: true
  stages:
    - id: check
      title: Check
      description: Check
      agent: north-worker
      writeScope: [README.md]
      allowNoChanges: true
      acceptance:
        - id: test
          type: exec
          command: [go, test, ./...]
`
	if err := os.WriteFile(planFile, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	paths := platform.Paths{StateDir: filepath.Join(root, "state"), CacheDir: filepath.Join(root, "cache")}
	var output bytes.Buffer
	if err := runExecutionCommand(&output, &bytes.Buffer{}, []string{planFile, "--max-parallel", "3", "--approve-plan", "--dry-run"}, paths); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "max parallel: 3") || !strings.Contains(output.String(), `host command check/test: ["go" "test" "./..."]`) {
		t.Fatalf("output = %q", output.String())
	}
	if _, err := os.Stat(paths.StateDir); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote state: %v", err)
	}
}

func TestPlanCreateAndApproveDryRunsDoNotWrite(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "plan.yaml")
	bin := t.TempDir()
	testutil.WriteExecutable(t, bin, "git", "exit 0")
	t.Setenv("PATH", bin)
	paths := platform.Paths{OpenCodeDir: filepath.Join(root, "opencode")}
	if err := os.MkdirAll(filepath.Join(paths.OpenCodeDir, "agents"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths.OpenCodeDir, "agents", "north-worker.md"), []byte("worker"), 0o600); err != nil {
		t.Fatal(err)
	}
	var plan bytes.Buffer
	if err := planCreateWithPlanner(context.Background(), &plan, []string{"--goal", "Preview changes", "--output", output, "--dry-run"}, paths, plannerStub{}); err != nil {
		t.Fatal(err)
	}
	if plan.Len() == 0 {
		t.Fatal("expected plan preview")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("plan create dry-run wrote output: %v", err)
	}
	if err := os.WriteFile(output, plan.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	approvalPaths := platform.Paths{StateDir: filepath.Join(root, "state")}
	if err := planApprove(&bytes.Buffer{}, []string{output, "--dry-run"}, approvalPaths); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(approvalPaths.StateDir); !os.IsNotExist(err) {
		t.Fatalf("plan approve dry-run wrote state: %v", err)
	}
}

func TestPlanCreateEnvironmentValidatesPlannerOutputBeforeEmission(t *testing.T) {
	root := t.TempDir()
	bin := t.TempDir()
	testutil.WriteExecutable(t, bin, "git", "exit 0")
	t.Setenv("PATH", bin)
	var output bytes.Buffer
	err := planCreateWithPlanner(context.Background(), &output, []string{"--goal", "Preview"}, platform.Paths{OpenCodeDir: filepath.Join(root, "opencode")}, missingAgentPlanner{})
	if err == nil || !strings.Contains(err.Error(), `references unavailable agent "unavailable"`) {
		t.Fatalf("error = %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("invalid planner output was emitted: %q", output.String())
	}
}
