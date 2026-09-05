package verification

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gitadapter "github.com/SBakolis/north/internal/git"
	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/orchestration"
)

func TestVerifierCriteriaScopeAndShellApproval(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("hello 42"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "base")
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("hello 43"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo, err := gitadapter.Open(context.Background(), root, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	v := New(Config{Git: repo, OutputCap: 8})
	criteria := []model.AcceptanceCriterion{
		{ID: "exists", Type: "file-exists", Path: "file.txt"},
		{ID: "missing", Type: "file-not-exists", Path: "missing.txt"},
		{ID: "contains", Type: "contains", Path: "file.txt", Value: "hello"},
		{ID: "matches", Type: "matches", Path: "file.txt", Value: `4[0-9]`},
		{ID: "diff", Type: "git-diff-not-empty"},
		{ID: "exec", Type: "exec", Command: []string{"sh", "-c", "printf 123456789012345"}},
	}
	result := v.Verify(context.Background(), orchestration.VerificationRequest{Workspace: root, WriteScope: []string{"*.txt"}, Criteria: criteria}, nil)
	if !result.Passed {
		t.Fatalf("result = %#v failure = %+v", result, result.Failure)
	}
	if !strings.Contains(result.Evidence[len(result.Evidence)-1], "truncated") {
		t.Fatalf("result = %#v", result)
	}

	denied := v.Verify(context.Background(), orchestration.VerificationRequest{Workspace: root, WriteScope: []string{"*.txt"}, Criteria: []model.AcceptanceCriterion{{ID: "shell", Type: "shell", Command: []string{"true"}}}}, nil)
	if denied.Passed || denied.Failure == nil {
		t.Fatalf("shell should be denied: %#v", denied)
	}
}

func TestVerifierTimeoutAndScopeFailure(t *testing.T) {
	root := t.TempDir()
	v := New(Config{Timeout: 10 * time.Millisecond})
	result := v.Verify(context.Background(), orchestration.VerificationRequest{Workspace: root, Criteria: []model.AcceptanceCriterion{{ID: "slow", Type: "exec", Command: []string{"sh", "-c", "sleep 1"}}}}, nil)
	if result.Passed || result.Failure == nil || !strings.Contains(result.Failure.Message, "timed out") {
		t.Fatalf("result = %#v", result)
	}
}

func TestVerifierBoundsFileReadsAndRedactsEvidence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "large.txt"), []byte("123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	bounded := New(Config{OutputCap: 8})
	result := bounded.Verify(context.Background(), orchestration.VerificationRequest{Workspace: root, Criteria: []model.AcceptanceCriterion{{ID: "large", Type: "contains", Path: "large.txt", Value: "1"}}}, nil)
	if result.Passed || result.Failure == nil || !strings.Contains(result.Failure.Message, "size limit") {
		t.Fatalf("result = %#v", result)
	}

	logs := filepath.Join(root, "logs")
	redacting := New(Config{Secrets: []string{"private-value"}, OutputCap: 8, LogDir: logs})
	result = redacting.Verify(context.Background(), orchestration.VerificationRequest{RunID: "run", StageID: "stage", Workspace: root, Criteria: []model.AcceptanceCriterion{{ID: "command", Type: "exec", Command: []string{"sh", "-c", "printf 'token=abc private-value and-complete-output'"}}}}, nil)
	if !result.Passed || strings.Contains(strings.Join(result.Evidence, ""), "token=abc") || strings.Contains(strings.Join(result.Evidence, ""), "private-value") {
		t.Fatalf("result = %#v failure = %+v", result, result.Failure)
	}
	logData, err := os.ReadFile(filepath.Join(logs, "run-stage-command.log"))
	if err != nil || !strings.Contains(string(logData), "and-complete-output") || strings.Contains(string(logData), "private-value") || strings.Contains(string(logData), "token=abc") {
		t.Fatalf("log = %q, error = %v", logData, err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
