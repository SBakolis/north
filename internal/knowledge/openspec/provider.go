// Package openspec adapts the read-only OpenSpec JSON commands to North's
// provider-neutral knowledge model.
package openspec

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/SBakolis/north/internal/knowledge"
	"github.com/SBakolis/north/internal/orchestration"
)

const ID = "openspec"

// Runner makes command execution replaceable in tests and embedding programs.
// name is always "npx" and the first argument is always "openspec".
type Runner interface {
	Run(ctx context.Context, dir, name string, args ...string) ([]byte, error)
}

type RunnerFunc func(context.Context, string, string, ...string) ([]byte, error)

func (f RunnerFunc) Run(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	return f(ctx, dir, name, args...)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "OPENSPEC_TELEMETRY=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

type Diagnostic struct {
	Code    string
	Message string
}

type Detection struct {
	Root        string
	ConfigPath  string
	Executable  string
	Version     string
	Diagnostics []Diagnostic
}

type Provider struct {
	runner Runner
	mu     sync.RWMutex
	detect Detection
}

var _ orchestration.KnowledgeProvider = (*Provider)(nil)

func New(runner ...Runner) *Provider {
	r := Runner(ExecRunner{})
	if len(runner) > 0 && runner[0] != nil {
		r = runner[0]
	}
	return &Provider{runner: r}
}

func NewProvider(runner Runner) *Provider { return New(runner) }

func (*Provider) ID() string { return ID }

func (p *Provider) Detection() Detection {
	p.mu.RLock()
	defer p.mu.RUnlock()
	d := p.detect
	d.Diagnostics = append([]Diagnostic(nil), d.Diagnostics...)
	return d
}

func (p *Provider) Detect(ctx context.Context, project orchestration.ProjectContext) (bool, error) {
	d := Detection{}
	root, config, err := findRoot(project.Root)
	if err != nil {
		d.Diagnostics = []Diagnostic{{Code: "project", Message: err.Error()}}
		p.setDetection(d)
		return false, err
	}
	if root == "" {
		p.setDetection(d) // clear diagnostics and stale detection state
		return false, nil
	}
	d.Root, d.ConfigPath, d.Executable = root, config, "npx openspec"
	out, err := p.runner.Run(ctx, root, "npx", "openspec", "--version")
	if err != nil {
		d.Diagnostics = []Diagnostic{{Code: "executable", Message: err.Error()}}
		p.setDetection(d)
		return false, err
	}
	d.Version = strings.TrimSpace(string(out))
	if d.Version == "" {
		err = errors.New("npx openspec --version returned an empty version")
		d.Diagnostics = []Diagnostic{{Code: "version", Message: err.Error()}}
		p.setDetection(d)
		return false, err
	}
	p.setDetection(d)
	return true, nil
}

func (p *Provider) setDetection(d Detection) {
	p.mu.Lock()
	p.detect = d
	p.mu.Unlock()
}

func findRoot(start string) (string, string, error) {
	if start == "" {
		return "", "", errors.New("project root is required")
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", "", err
	}
	if info, statErr := os.Stat(dir); statErr == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		config := filepath.Join(dir, "openspec", "config.yaml")
		if info, statErr := os.Stat(config); statErr == nil && info.Mode().IsRegular() {
			return dir, config, nil
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", "", statErr
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", nil
		}
		dir = parent
	}
}

type statusOutput struct {
	ChangeName    string                         `json:"changeName"`
	ChangeRoot    string                         `json:"changeRoot"`
	ArtifactPaths map[string]artifactPathSummary `json:"artifactPaths"`
	Artifacts     []artifactStatus               `json:"artifacts"`
	Status        []commandDiagnostic            `json:"status"`
}

type artifactStatus struct {
	ID         string   `json:"id"`
	OutputPath string   `json:"outputPath"`
	Status     string   `json:"status"`
	Requires   []string `json:"requires"`
}

type artifactPathSummary struct {
	OutputPath          string   `json:"outputPath"`
	ResolvedOutputPath  string   `json:"resolvedOutputPath"`
	ExistingOutputPaths []string `json:"existingOutputPaths"`
}

type commandDiagnostic struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type validateOutput struct {
	Valid bool `json:"valid"`
	Items []struct {
		Valid  bool `json:"valid"`
		Issues []struct {
			Level   string `json:"level"`
			Path    string `json:"path"`
			Message string `json:"message"`
		} `json:"issues"`
	} `json:"items"`
	Status []commandDiagnostic `json:"status"`
}

type showOutput struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Why    string `json:"why"`
	Deltas []struct {
		Spec        string `json:"spec"`
		Operation   string `json:"operation"`
		Description string `json:"description"`
	} `json:"deltas"`
	Status []commandDiagnostic `json:"status"`
}

func (p *Provider) Load(ctx context.Context, request orchestration.KnowledgeRequest) (knowledge.Snapshot, error) {
	p.mu.RLock()
	d := p.detect
	p.mu.RUnlock()
	if d.Root == "" {
		return knowledge.Snapshot{}, errors.New("openspec provider has not detected a project")
	}
	if err := validChangeID(request.ChangeID); err != nil {
		return knowledge.Snapshot{}, err
	}

	statusBytes, err := p.runJSON(ctx, d.Root, "status", "--change", request.ChangeID, "--json")
	if err != nil {
		return knowledge.Snapshot{}, err
	}
	var status statusOutput
	if err := decodeJSON(statusBytes, &status); err != nil {
		return knowledge.Snapshot{}, fmt.Errorf("decode openspec status: %w", err)
	}
	if err := diagnosticError(status.Status); err != nil {
		return knowledge.Snapshot{}, fmt.Errorf("openspec status: %w", err)
	}

	validateBytes, err := p.runJSON(ctx, d.Root, "validate", request.ChangeID, "--type", "change", "--json")
	if err != nil {
		return knowledge.Snapshot{}, err
	}
	var validation validateOutput
	if err := decodeJSON(validateBytes, &validation); err != nil {
		return knowledge.Snapshot{}, fmt.Errorf("decode openspec validate: %w", err)
	}
	if err := validationError(validation); err != nil {
		return knowledge.Snapshot{}, err
	}

	snapshot := knowledge.Snapshot{Provider: ID, SourceRoot: d.Root}
	seen := make(map[string]bool)
	for _, artifact := range status.Artifacts {
		if artifact.Status != "done" {
			continue
		}
		paths := status.ArtifactPaths[artifact.ID].ExistingOutputPaths
		for _, path := range paths {
			absolute, relative, err := safeArtifactPath(d.Root, status.ChangeRoot, path)
			if err != nil {
				return knowledge.Snapshot{}, fmt.Errorf("artifact %q: %w", artifact.ID, err)
			}
			if seen[absolute] {
				continue
			}
			seen[absolute] = true
			content, err := os.ReadFile(absolute)
			if err != nil {
				return knowledge.Snapshot{}, fmt.Errorf("read artifact %q: %w", relative, err)
			}
			hash := fmt.Sprintf("%x", sha256.Sum256(content))
			snapshot.RawArtifacts = append(snapshot.RawArtifacts, knowledge.ArtifactReference{Path: filepath.ToSlash(relative), SHA256: hash})
			normalizeMarkdown(&snapshot, artifact.ID, filepath.ToSlash(relative), string(content))
		}
	}

	// show provides semantic delta descriptions when the schema has a proposal.
	// A custom schema may intentionally omit it, so status artifacts remain the
	// authoritative fallback and a show failure is not treated as mutation.
	if showBytes, showErr := p.runJSON(ctx, d.Root, "show", request.ChangeID, "--type", "change", "--json"); showErr == nil {
		var show showOutput
		if decodeJSON(showBytes, &show) == nil && diagnosticError(show.Status) == nil {
			mergeShow(&snapshot, show)
		}
	}
	return snapshot, nil
}

func (p *Provider) runJSON(ctx context.Context, root string, args ...string) ([]byte, error) {
	full := append([]string{"openspec"}, args...)
	out, err := p.runner.Run(ctx, root, "npx", full...)
	if err != nil {
		return nil, fmt.Errorf("npx openspec %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func decodeJSON(data []byte, value any) error {
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(string(data))))
	if err := decoder.Decode(value); err != nil {
		return err
	}
	return nil
}

func diagnosticError(diagnostics []commandDiagnostic) error {
	var messages []string
	for _, d := range diagnostics {
		if strings.EqualFold(d.Severity, "error") {
			messages = append(messages, d.Message)
		}
	}
	if len(messages) > 0 {
		return errors.New(strings.Join(messages, "; "))
	}
	return nil
}

func validationError(v validateOutput) error {
	if err := diagnosticError(v.Status); err != nil {
		return fmt.Errorf("openspec validate: %w", err)
	}
	valid := v.Valid
	if len(v.Items) > 0 {
		valid = true
		var issues []string
		for _, item := range v.Items {
			valid = valid && item.Valid
			for _, issue := range item.Issues {
				if strings.EqualFold(issue.Level, "ERROR") {
					issues = append(issues, strings.TrimSpace(issue.Path+": "+issue.Message))
				}
			}
		}
		if !valid {
			return fmt.Errorf("openspec validation failed: %s", strings.Join(issues, "; "))
		}
	}
	if !valid {
		return errors.New("openspec validation failed")
	}
	return nil
}

var changeIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validChangeID(id string) error {
	if !changeIDPattern.MatchString(id) {
		return fmt.Errorf("invalid OpenSpec change ID %q", id)
	}
	return nil
}

func safeArtifactPath(root, changeRoot, candidate string) (string, string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", "", err
	}
	base := changeRoot
	if base == "" {
		base = filepath.Join(rootAbs, "openspec", "changes")
	}
	if !filepath.IsAbs(base) {
		base = filepath.Join(rootAbs, base)
	}
	path := candidate
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	path = filepath.Clean(path)
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(rootReal, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", "", fmt.Errorf("path %q escapes project root", candidate)
	}
	info, err := os.Stat(real)
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("path %q is not a regular file", candidate)
	}
	return real, rel, nil
}
