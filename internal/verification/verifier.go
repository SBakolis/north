// Package verification executes host-controlled acceptance criteria.
package verification

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	gitadapter "github.com/SBakolis/north/internal/git"
	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/orchestration"
)

const defaultOutputCap = 1 << 20

type Config struct {
	Git        *gitadapter.Repository
	AllowShell bool
	OutputCap  int64
	Timeout    time.Duration
	Env        []string
	Secrets    []string
	LogDir     string
}

type Verifier struct{ config Config }

var _ orchestration.VerificationProvider = (*Verifier)(nil)

func New(config Config) *Verifier {
	if config.OutputCap <= 0 {
		config.OutputCap = defaultOutputCap
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Minute
	}
	environment := config.Env
	if environment == nil {
		environment = os.Environ()
	}
	for _, item := range environment {
		key, value, ok := strings.Cut(item, "=")
		if ok && sensitiveEnvironmentKey.MatchString(key) && value != "" {
			config.Secrets = append(config.Secrets, value)
		}
	}
	return &Verifier{config: config}
}

func (v *Verifier) Verify(ctx context.Context, req orchestration.VerificationRequest, sink orchestration.EventSink) orchestration.VerificationResult {
	var committedHead string
	if req.RequireClean && v.config.Git != nil {
		var err error
		committedHead, err = v.config.Git.ResolveAt(ctx, req.Workspace, "HEAD")
		if err != nil {
			return failure("verification-mutation", err)
		}
	}
	if v.config.Git != nil {
		paths, err := v.config.Git.ChangedPaths(ctx, req.Workspace, "HEAD")
		if err != nil {
			return failure("scope", err)
		}
		if err := gitadapter.ValidateScope(req.Workspace, paths, req.WriteScope); err != nil {
			return failure("scope", err)
		}
	}
	result := orchestration.VerificationResult{Passed: true}
	for _, criterion := range req.Criteria {
		evidence, err := v.verifyOne(ctx, req, criterion)
		evidence = v.redact(evidence)
		if evidence != "" {
			result.Evidence = append(result.Evidence, criterion.ID+": "+evidence)
		}
		if sink != nil {
			status := "passed"
			if err != nil {
				status = "failed"
			}
			_ = sink.Emit(ctx, model.Event{RunID: req.RunID, StageID: req.StageID, Type: "verification." + status, Message: criterion.ID, Data: map[string]any{"evidence": evidence}})
		}
		if err != nil {
			result.Passed = false
			result.Failure = &model.StageFailure{Class: "verification", Message: fmt.Sprintf("%s: %v", criterion.ID, err), Retryable: true}
			return result
		}
	}
	if v.config.Git != nil {
		paths, err := v.config.Git.ChangedPaths(ctx, req.Workspace, "HEAD")
		if err != nil {
			return failure("scope", err)
		}
		if err := gitadapter.ValidateScope(req.Workspace, paths, req.WriteScope); err != nil {
			return failure("scope", err)
		}
		if req.RequireClean {
			head, err := v.config.Git.ResolveAt(ctx, req.Workspace, "HEAD")
			if err != nil {
				return failure("verification-mutation", err)
			}
			if head != committedHead {
				return failure("verification-mutation", errors.New("verification changed the integration commit"))
			}
		}
		if req.RequireClean && len(paths) > 0 {
			return failure("verification-mutation", fmt.Errorf("verification changed the committed tree: %s", strings.Join(paths, ", ")))
		}
	}
	return result
}

func failure(class string, err error) orchestration.VerificationResult {
	return orchestration.VerificationResult{Failure: &model.StageFailure{Class: class, Message: err.Error(), Retryable: false}}
}

func (v *Verifier) verifyOne(parent context.Context, req orchestration.VerificationRequest, criterion model.AcceptanceCriterion) (string, error) {
	workspace := req.Workspace
	timeout := criterion.Timeout
	if timeout <= 0 {
		timeout = v.config.Timeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	switch criterion.Type {
	case "command", "exec":
		if len(criterion.Command) == 0 {
			return "", errors.New("command argv is empty")
		}
		return v.run(ctx, workspace, criterion.Command, req.RunID, req.StageID, criterion.ID)
	case "shell":
		if !v.config.AllowShell {
			return "", errors.New("shell criterion is not approved")
		}
		if len(criterion.Command) != 1 {
			return "", errors.New("shell criterion requires one command string")
		}
		shell, flag := "/bin/sh", "-c"
		if runtime.GOOS == "windows" {
			shell, flag = "cmd.exe", "/C"
		}
		return v.run(ctx, workspace, []string{shell, flag, criterion.Command[0]}, req.RunID, req.StageID, criterion.ID)
	case "file-exists", "fileExists", "file-not-exists", "fileNotExists", "contains", "fileContains", "matches", "fileMatches":
		path, err := gitadapter.SafePath(workspace, criterion.Path)
		if err != nil {
			return "", err
		}
		info, statErr := os.Stat(path)
		if criterion.Type == "file-exists" || criterion.Type == "fileExists" {
			if statErr != nil {
				return "", statErr
			}
			return criterion.Path + " exists", nil
		}
		if criterion.Type == "file-not-exists" || criterion.Type == "fileNotExists" {
			if os.IsNotExist(statErr) {
				return criterion.Path + " does not exist", nil
			}
			if statErr != nil {
				return "", statErr
			}
			return "", fmt.Errorf("%s exists", criterion.Path)
		}
		if statErr != nil {
			return "", statErr
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("%s is not a regular file", criterion.Path)
		}
		if info.Size() > v.config.OutputCap {
			return "", fmt.Errorf("%s exceeds verification size limit of %d bytes", criterion.Path, v.config.OutputCap)
		}
		content, err := readBounded(path, v.config.OutputCap)
		if err != nil {
			return "", err
		}
		if criterion.Type == "contains" || criterion.Type == "fileContains" {
			if !bytes.Contains(content, []byte(criterion.Value)) {
				return "", fmt.Errorf("%s does not contain expected value", criterion.Path)
			}
			return criterion.Path + " contains expected value", nil
		}
		expression, err := regexp.Compile(criterion.Value)
		if err != nil {
			return "", fmt.Errorf("invalid regular expression: %w", err)
		}
		if !expression.Match(content) {
			return "", fmt.Errorf("%s does not match expected expression", criterion.Path)
		}
		return criterion.Path + " matches expected expression", nil
	case "git-diff-not-empty", "gitDiffNotEmpty":
		if v.config.Git == nil {
			return "", errors.New("git repository is not configured")
		}
		base := criterion.Value
		if base == "" {
			base = "HEAD"
		}
		paths, err := v.config.Git.ChangedPaths(ctx, workspace, base)
		if err != nil {
			return "", err
		}
		if len(paths) == 0 {
			return "", errors.New("git diff is empty")
		}
		return strings.Join(paths, "\n"), nil
	default:
		return "", fmt.Errorf("unsupported criterion type %q", criterion.Type)
	}
}

func readBounded(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		return nil, fmt.Errorf("%s exceeds verification size limit of %d bytes", path, limit)
	}
	return content, nil
}

func (v *Verifier) run(ctx context.Context, workspace string, argv []string, runID, stageID, criterionID string) (string, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = workspace
	configureProcess(cmd)
	if v.config.Env != nil {
		cmd.Env = append([]string(nil), v.config.Env...)
	}
	output := &limitedBuffer{limit: v.config.OutputCap}
	destination := io.Writer(output)
	var logFile *os.File
	var logWriter *redactingLogWriter
	logPath := ""
	if v.config.LogDir != "" {
		if err := os.MkdirAll(v.config.LogDir, 0o700); err != nil {
			return "", err
		}
		logPath = filepath.Join(v.config.LogDir, safeLogName(runID)+"-"+safeLogName(stageID)+"-"+safeLogName(criterionID)+".log")
		var err error
		logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return "", err
		}
		defer logFile.Close()
		logWriter = &redactingLogWriter{destination: logFile, redact: v.redact}
		defer logWriter.Close()
		destination = io.MultiWriter(output, logWriter)
	}
	cmd.Stdout, cmd.Stderr = destination, destination
	err := cmd.Run()
	text := output.String()
	if logPath != "" {
		text += "\n[full log: " + logPath + "]"
	}
	if ctx.Err() != nil {
		return text, fmt.Errorf("command timed out: %w", ctx.Err())
	}
	if err != nil {
		return text, fmt.Errorf("command %q failed: %w", argv[0], err)
	}
	return text, nil
}

func safeLogName(value string) string {
	value = filepath.Base(value)
	if value == "." || value == "" {
		return "unknown"
	}
	return value
}

type redactingLogWriter struct {
	destination io.Writer
	redact      func(string) string
	pending     bytes.Buffer
}

func (w *redactingLogWriter) Write(p []byte) (int, error) {
	n := len(p)
	_, _ = w.pending.Write(p)
	for {
		line, err := w.pending.ReadString('\n')
		if err != nil {
			_, _ = w.pending.WriteString(line)
			if w.pending.Len() > 64<<10 {
				flush := w.pending.Next(w.pending.Len() - (4 << 10))
				if _, writeErr := io.WriteString(w.destination, w.redact(string(flush))); writeErr != nil {
					return 0, writeErr
				}
			}
			return n, nil
		}
		if _, err := io.WriteString(w.destination, w.redact(line)); err != nil {
			return 0, err
		}
	}
}

func (w *redactingLogWriter) Close() error {
	if w.pending.Len() == 0 {
		return nil
	}
	_, err := io.WriteString(w.destination, w.redact(w.pending.String()))
	w.pending.Reset()
	return err
}

var credentialPattern = regexp.MustCompile(`(?i)(bearer\s+|(?:api[_-]?key|token|secret|password)["'=:\s]+)([^\s"']+)`)
var sensitiveEnvironmentKey = regexp.MustCompile(`(?i)(token|secret|password|api[_-]?key|credential|auth|private[_-]?key|access[_-]?key)`)

func (v *Verifier) redact(value string) string {
	value = credentialPattern.ReplaceAllString(value, "$1[REDACTED]")
	secrets := append([]string(nil), v.config.Secrets...)
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	limit     int64
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - int64(b.buffer.Len())
	if remaining > 0 {
		if int64(len(p)) > remaining {
			p = p[:remaining]
			b.truncated = true
		}
		_, _ = b.buffer.Write(p)
	} else {
		b.truncated = true
	}
	return n, nil
}
func (b *limitedBuffer) String() string {
	value := b.buffer.String()
	if b.truncated {
		value += "\n[output truncated]"
	}
	return value
}
