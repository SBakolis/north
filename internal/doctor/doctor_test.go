package doctor

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SBakolis/north/internal/application"
	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/platform"
	"github.com/SBakolis/north/internal/testutil"
)

func TestCheckRunStateReportsCleanupCandidates(t *testing.T) {
	repository := t.TempDir()
	runGit(t, repository, "init", "-b", "main")
	runGit(t, repository, "config", "user.name", "North Tests")
	runGit(t, repository, "config", "user.email", "north@example.invalid")
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", "README.md")
	runGit(t, repository, "commit", "-m", "base")
	runGit(t, repository, "branch", "north/old-stage")

	paths := platform.Paths{StateDir: t.TempDir()}
	worktree := filepath.Join(t.TempDir(), "old-worktree")
	if err := os.MkdirAll(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	run := model.RunState{ID: "old-run", Status: model.RunCompleted, Stages: []model.StageState{{ID: "stage", Branch: "north/old-stage", Workspace: worktree}}}
	runDir := filepath.Join(paths.StateDir, "projects", application.ProjectID(repository), "runs", run.ID)
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	var checks []Check
	checkRunState(context.Background(), paths, repository, func(id string, severity Severity, message string) {
		checks = append(checks, Check{ID: id, Severity: severity, Message: message})
	})
	for _, check := range checks {
		if check.ID == "cleanup-candidate" && check.Severity == Advisory {
			return
		}
	}
	t.Fatalf("checks = %+v", checks)
}

func TestOpenSpecCheckIsLocalNoInstallAndBounded(t *testing.T) {
	bin := t.TempDir()
	testutil.WriteExecutable(t, bin, "npx", `[ "$1" = "--no-install" ] && [ "$2" = "openspec" ] && [ "$3" = "--version" ] && [ "$npm_config_offline" = "true" ] && [ "$npm_config_yes" = "false" ] && echo 1.2.3`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := localOpenSpecVersion(context.Background())
	if err != nil || strings.TrimSpace(string(output)) != "1.2.3" {
		t.Fatalf("output=%q error=%v", output, err)
	}

	testutil.WriteExecutable(t, bin, "npx", `sleep 1`)
	previous := openSpecCheckTimeout
	openSpecCheckTimeout = 20 * time.Millisecond
	t.Cleanup(func() { openSpecCheckTimeout = previous })
	started := time.Now()
	if _, err := localOpenSpecVersion(context.Background()); err == nil || time.Since(started) > 500*time.Millisecond {
		t.Fatalf("bounded check error=%v duration=%s", err, time.Since(started))
	}
}

func runGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
