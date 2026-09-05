package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// WriteExecutable creates a deterministic shell-backed fake command for integration tests.
func WriteExecutable(t testing.TB, dir, name, script string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := "#!/bin/sh\nset -eu\n" + script + "\n"
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("write fake executable: %v", err)
	}
	return path
}
