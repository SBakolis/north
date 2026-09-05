package testutil

import (
	"os/exec"
	"testing"
)

// GitRepository initializes a temporary repository with a deterministic identity.
func GitRepository(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	commands := [][]string{
		{"init", "-b", "main"},
		{"config", "user.name", "North Test"},
		{"config", "user.email", "north@example.invalid"},
	}
	for _, args := range commands {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	return dir
}
