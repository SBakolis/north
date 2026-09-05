package config

import (
	"fmt"
	"sort"
	"strings"
)

type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string {
	return "invalid North configuration: " + strings.Join(e.Problems, "; ")
}

func Validate(c Config) error {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}
	if c.APIVersion != APIVersionV1Alpha1 {
		add("apiVersion must be %q", APIVersionV1Alpha1)
	}
	if c.Kind != KindNorthConfig {
		add("kind must be %q", KindNorthConfig)
	}
	if c.Installation.Scope != "global" {
		add("installation.scope must be %q", "global")
	}
	if c.Parallelization.Runtime != "opencode-cli" {
		add("parallelization.runtime must be %q", "opencode-cli")
	}
	if c.Parallelization.Isolation != "git-worktree" {
		add("parallelization.isolation must be %q", "git-worktree")
	}
	if c.Parallelization.Integration != "progressive" {
		add("parallelization.integration must be %q", "progressive")
	}
	if c.Parallelization.MaxParallel < 1 || c.Parallelization.MaxParallel > MaxMaxParallel {
		add("parallelization.maxParallel must be between 1 and %d", MaxMaxParallel)
	}
	if c.Knowledge.Provider != "none" && c.Knowledge.Provider != "openspec" {
		add("knowledge.provider must be one of %q or %q", "none", "openspec")
	}
	sort.Strings(problems)
	if len(problems) != 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}
