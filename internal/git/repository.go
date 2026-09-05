package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ErrDirty               = errors.New("repository has uncommitted changes")
	ErrOutsideScope        = errors.New("changed path is outside write scope")
	ErrSymlinkEscape       = errors.New("path escapes workspace through symlink")
	ErrUnsafeCleanup       = errors.New("refusing unsafe worktree cleanup")
	ErrIntegrationConflict = errors.New("integration conflict")
	ErrTargetDiverged      = errors.New("target branch has diverged")
)

type Repository struct {
	Root         string
	WorktreeRoot string
	Runner       Runner
}

func Open(ctx context.Context, start, worktreeRoot string, runner Runner) (*Repository, error) {
	if runner == nil {
		runner = CommandRunner{}
	}
	result, err := runner.Run(ctx, start, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	root, err := canonicalPath(strings.TrimSpace(result.Stdout))
	if err != nil {
		return nil, err
	}
	repository := &Repository{Root: root, Runner: runner}
	if err := repository.SetWorktreeRoot(worktreeRoot); err != nil {
		return nil, err
	}
	return repository, nil
}

// SetWorktreeRoot canonicalizes existing ancestors so a symlinked cache cannot
// place managed worktrees inside the repository.
func (r *Repository) SetWorktreeRoot(worktreeRoot string) error {
	canonical, err := canonicalPath(worktreeRoot)
	if err != nil {
		return err
	}
	if within(r.Root, canonical) {
		return fmt.Errorf("worktree root must be outside repository: %s", canonical)
	}
	r.WorktreeRoot = canonical
	return nil
}

func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	current := absolute
	var missing []string
	for {
		_, err := os.Lstat(current)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
	current, err = filepath.EvalSymlinks(current)
	if err != nil {
		return "", err
	}
	for index := len(missing) - 1; index >= 0; index-- {
		current = filepath.Join(current, missing[index])
	}
	return filepath.Clean(current), nil
}

func (r *Repository) Clean(ctx context.Context) error {
	result, err := r.Runner.Run(ctx, r.Root, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return err
	}
	if result.Stdout != "" {
		return ErrDirty
	}
	return nil
}

func (r *Repository) ResolveBase(ctx context.Context, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	result, err := r.Runner.Run(ctx, r.Root, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func SanitizeBranch(namespace string, parts ...string) string {
	all := append([]string{namespace}, parts...)
	for i, part := range all {
		part = strings.ToLower(part)
		var b strings.Builder
		lastDash := false
		for _, ch := range part {
			valid := ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_'
			if valid {
				b.WriteRune(ch)
				lastDash = false
			} else if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
		all[i] = strings.Trim(b.String(), "-._/")
		if all[i] == "" {
			all[i] = "unnamed"
		}
	}
	return strings.Join(all, "/")
}

func (r *Repository) ChangedPaths(ctx context.Context, workspace, base string) ([]string, error) {
	if base == "" {
		base = "HEAD"
	}
	// Disabling rename detection reports both the deleted source and added destination.
	tracked, err := r.Runner.Run(ctx, workspace, "diff", "--no-renames", "--name-only", "-z", base, "--")
	if err != nil {
		return nil, err
	}
	untracked, err := r.Runner.Run(ctx, workspace, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{})
	for _, raw := range []string{tracked.Stdout, untracked.Stdout} {
		for _, path := range strings.Split(raw, "\x00") {
			if path != "" {
				set[filepath.ToSlash(filepath.Clean(path))] = struct{}{}
			}
		}
	}
	paths := make([]string, 0, len(set))
	for path := range set {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func ValidateScope(workspace string, changed, scope []string) error {
	root, err := filepath.Abs(workspace)
	if err != nil {
		return err
	}
	for _, path := range changed {
		clean := filepath.Clean(filepath.FromSlash(path))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("%w: %s", ErrOutsideScope, path)
		}
		allowed := false
		slashPath := filepath.ToSlash(clean)
		for _, pattern := range scope {
			pattern = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(pattern)), "./")
			if pattern == "." || pattern == "**" || pattern == "*" {
				allowed = true
				break
			}
			matched, matchErr := matchGlob(pattern, slashPath)
			if matchErr != nil {
				return fmt.Errorf("invalid scope pattern %q: %w", pattern, matchErr)
			}
			literalDirectory := !strings.ContainsAny(pattern, "*?[") && strings.HasPrefix(slashPath, strings.TrimSuffix(pattern, "/")+"/")
			if matched || literalDirectory || slashPath == strings.TrimSuffix(pattern, "/**") {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("%w: %s", ErrOutsideScope, path)
		}
		if err := rejectSymlinkEscape(root, clean); err != nil {
			return err
		}
	}
	return nil
}

func matchGlob(pattern, name string) (bool, error) {
	patternParts, nameParts := strings.Split(pattern, "/"), strings.Split(name, "/")
	var walk func(int, int) (bool, error)
	walk = func(pi, ni int) (bool, error) {
		if pi == len(patternParts) {
			return ni == len(nameParts), nil
		}
		if patternParts[pi] == "**" {
			for i := ni; i <= len(nameParts); i++ {
				if ok, err := walk(pi+1, i); ok || err != nil {
					return ok, err
				}
			}
			return false, nil
		}
		if ni == len(nameParts) {
			return false, nil
		}
		ok, err := filepath.Match(patternParts[pi], nameParts[ni])
		if !ok || err != nil {
			return ok, err
		}
		return walk(pi+1, ni+1)
	}
	return walk(0, 0)
}

func rejectSymlinkEscape(root, relative string) error {
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return err
			}
			if !within(root, resolved) {
				return fmt.Errorf("%w: %s", ErrSymlinkEscape, relative)
			}
			current = resolved
		}
	}
	return nil
}

// SafePath resolves a repository-relative path and rejects traversal and symlink escapes.
func SafePath(workspace, path string) (string, error) {
	root, err := filepath.Abs(workspace)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s", ErrOutsideScope, path)
	}
	if err := rejectSymlinkEscape(root, clean); err != nil {
		return "", err
	}
	return filepath.Join(root, clean), nil
}

func within(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
