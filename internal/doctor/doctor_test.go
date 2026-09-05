package doctor

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/SBakolis/north/internal/application"
	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/platform"
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

func runGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
