// Package git provides the production Git adapter used by North.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Runner interface {
	Run(context.Context, string, ...string) (Result, error)
}

type CommandRunner struct {
	Path string
}

func (r CommandRunner) Run(ctx context.Context, dir string, args ...string) (Result, error) {
	path := r.Path
	if path == "" {
		path = "git"
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		result.ExitCode = exit.ExitCode()
	}
	return result, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(result.Stderr))
}
