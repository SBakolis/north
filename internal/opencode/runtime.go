// Package opencode implements North's OpenCode CLI runtime adapter.
package opencode

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/orchestration"
)

const defaultOutputCap = 4 << 20

type Config struct {
	Binary    string
	Agent     string
	LogDir    string
	StateDir  string
	Env       map[string]string
	Secrets   []string
	OutputCap int64
}

type Event struct {
	Type      string          `json:"type,omitempty"`
	SessionID string          `json:"sessionID,omitempty"`
	Message   string          `json:"message,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

type Runtime struct {
	config Config
	mu     sync.Mutex
	cancel map[string]context.CancelFunc
}

var _ orchestration.AgentRuntime = (*Runtime)(nil)

func New(config Config) *Runtime {
	if config.Binary == "" {
		config.Binary = "opencode"
	}
	if config.Agent == "" {
		config.Agent = "north-worker"
	}
	if config.LogDir == "" {
		config.LogDir = filepath.Join(os.TempDir(), "north-opencode")
	}
	if config.OutputCap <= 0 {
		config.OutputCap = defaultOutputCap
	}
	return &Runtime{config: config, cancel: make(map[string]context.CancelFunc)}
}

type OutputPaths struct {
	Stdout string
	Stderr string
}

func (r *Runtime) OutputPaths(executionID string) OutputPaths {
	return OutputPaths{
		Stdout: filepath.Join(r.config.LogDir, executionID+".stdout.jsonl"),
		Stderr: filepath.Join(r.config.LogDir, executionID+".stderr.log"),
	}
}

// ParseEvent accepts known and future OpenCode event shapes while retaining the original JSON.
func ParseEvent(data []byte) (Event, error) {
	event := Event{Raw: append(json.RawMessage(nil), data...)}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		event.Type = "unknown"
		return event, err
	}
	event.Type, _ = fields["type"].(string)
	if event.Type == "" {
		event.Type = "unknown"
	}
	event.SessionID = extractSession(fields)
	event.Message = eventMessage(fields)
	return event, nil
}

func (r *Runtime) Validate(ctx context.Context) error {
	for _, args := range [][]string{{"--version"}, {"run", "--help"}} {
		cmd := exec.CommandContext(ctx, r.config.Binary, args...)
		cmd.Env = r.environment(nil)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("validate opencode %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		}
		if len(bytes.TrimSpace(output)) == 0 {
			return fmt.Errorf("validate opencode %s: empty output", strings.Join(args, " "))
		}
		if args[0] == "run" {
			text := string(output)
			for _, flag := range []string{"--dir", "--agent", "--format"} {
				if !strings.Contains(text, flag) {
					return fmt.Errorf("opencode run does not support required flag %s", flag)
				}
			}
		}
	}
	return nil
}

func (r *Runtime) Execute(ctx context.Context, req orchestration.AgentRequest, sink orchestration.EventSink) (orchestration.AgentResult, error) {
	executionID, err := randomID()
	if err != nil {
		return orchestration.AgentResult{}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	ctx = context.WithValue(ctx, executionIDKey{}, executionID)
	r.mu.Lock()
	r.cancel[executionID] = cancel
	r.mu.Unlock()
	defer func() { cancel(); r.mu.Lock(); delete(r.cancel, executionID); r.mu.Unlock() }()
	if req.Timeout > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, req.Timeout)
		defer timeoutCancel()
	}
	if err := os.MkdirAll(r.config.LogDir, 0o700); err != nil {
		return orchestration.AgentResult{}, err
	}
	paths := r.OutputPaths(executionID)
	stdoutPath, stderrPath := paths.Stdout, paths.Stderr
	stdoutFile, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return orchestration.AgentResult{}, err
	}
	defer stdoutFile.Close()
	stderrFile, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return orchestration.AgentResult{}, err
	}
	defer stderrFile.Close()

	agent := req.Agent
	if agent == "" {
		agent = r.config.Agent
	}
	args := []string{"run", "--dir", req.Workspace, "--agent", agent, "--format", "json"}
	if req.SessionID != "" {
		args = append(args, "--session", req.SessionID)
	}
	args = append(args, req.Prompt)
	cmd := exec.CommandContext(ctx, r.config.Binary, args...)
	cmd.Dir = req.Workspace
	cmd.Env = r.environment(map[string]string{
		"NORTH_ACTIVE": "1", "NORTH_RUN_ID": req.RunID, "NORTH_STAGE_ID": req.StageID,
		"NORTH_EXECUTION_ID": executionID, "NORTH_WORKTREE": req.Workspace,
		"NORTH_STATE_DIR": r.config.StateDir, "NORTH_AGENT_ROLE": valueOr(req.Role, "worker"),
	})
	configureProcess(cmd)
	pipeReader, pipeWriter := io.Pipe()
	stdoutMemory, stderrMemory := &limitedBuffer{limit: r.config.OutputCap}, &limitedBuffer{limit: r.config.OutputCap}
	stdoutLog := newRedactingWriter(stdoutFile, r.redact)
	stderrLog := newRedactingWriter(stderrFile, r.redact)
	cmd.Stdout = io.MultiWriter(stdoutLog, stdoutMemory, pipeWriter)
	cmd.Stderr = io.MultiWriter(stderrLog, stderrMemory)

	parseDone := make(chan parseResult, 1)
	go func() { parseDone <- r.parseEvents(ctx, pipeReader, req, sink) }()
	if err := cmd.Start(); err != nil {
		pipeWriter.Close()
		<-parseDone
		return orchestration.AgentResult{ExecutionID: executionID, ExitCode: -1}, err
	}
	pid := cmd.Process.Pid
	if req.Started != nil {
		if err := req.Started(orchestration.AgentExecution{ExecutionID: executionID, PID: pid}); err != nil {
			cancel()
			_ = cmd.Wait()
			pipeWriter.Close()
			<-parseDone
			return orchestration.AgentResult{ExecutionID: executionID, ExitCode: -1, PID: pid}, fmt.Errorf("persist worker start: %w", err)
		}
	}
	runErr := cmd.Wait()
	pipeWriter.Close()
	_ = stdoutLog.Close()
	_ = stderrLog.Close()
	parsed := <-parseDone
	result := orchestration.AgentResult{ExecutionID: executionID, SessionID: parsed.sessionID, Output: r.redact(stdoutMemory.String()), PID: pid}
	if runErr == nil && parsed.err == nil {
		return result, nil
	}
	var exit *exec.ExitError
	if errors.As(runErr, &exit) {
		result.ExitCode = exit.ExitCode()
	} else if runErr != nil {
		result.ExitCode = -1
	}
	redacted := r.redact(stderrMemory.String())
	if parsed.err != nil {
		return result, errors.Join(runErr, parsed.err)
	}
	return result, fmt.Errorf("opencode exited with code %d: %s: %w", result.ExitCode, redacted, runErr)
}

func (r *Runtime) Cancel(ctx context.Context, executionID string) error {
	r.mu.Lock()
	cancel := r.cancel[executionID]
	r.mu.Unlock()
	if cancel == nil {
		return fmt.Errorf("unknown execution %q", executionID)
	}
	cancel()
	return nil
}

type parseResult struct {
	sessionID string
	err       error
}

type executionIDKey struct{}

func executionIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(executionIDKey{}).(string)
	return id
}

func (r *Runtime) parseEvents(ctx context.Context, reader io.Reader, req orchestration.AgentRequest, sink orchestration.EventSink) parseResult {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), int(r.config.OutputCap)+1)
	var result parseResult
	for scanner.Scan() {
		raw := append(json.RawMessage(nil), scanner.Bytes()...)
		event, parseErr := ParseEvent(raw)
		if parseErr != nil {
			if sink != nil {
				if err := sink.Emit(ctx, model.Event{RunID: req.RunID, StageID: req.StageID, Type: "opencode.unknown", Message: r.redact(string(raw)), Data: map[string]any{"raw": r.redact(string(raw)), "executionId": executionIDFrom(ctx)}}); err != nil && result.err == nil {
					result.err = err
				}
			}
			continue
		}
		if result.sessionID == "" {
			result.sessionID = event.SessionID
		}
		if sink != nil {
			err := sink.Emit(ctx, model.Event{RunID: req.RunID, StageID: req.StageID, Type: "opencode." + event.Type, Message: r.redact(event.Message), Data: map[string]any{"raw": r.redact(string(event.Raw)), "executionId": executionIDFrom(ctx), "sessionId": event.SessionID}})
			if err != nil && result.err == nil {
				result.err = err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		result.err = errors.Join(result.err, err)
		_, _ = io.Copy(io.Discard, reader)
	}
	return result
}

func extractSession(value any) string {
	switch value := value.(type) {
	case map[string]any:
		for _, key := range []string{"sessionID", "sessionId", "session_id"} {
			if id, ok := value[key].(string); ok {
				return id
			}
		}
		for _, child := range value {
			if id := extractSession(child); id != "" {
				return id
			}
		}
	case []any:
		for _, child := range value {
			if id := extractSession(child); id != "" {
				return id
			}
		}
	}
	return ""
}

func eventMessage(value any) string {
	switch value := value.(type) {
	case map[string]any:
		for _, key := range []string{"message", "text", "content"} {
			if text, ok := value[key].(string); ok {
				return text
			}
		}
		for _, child := range value {
			if text := eventMessage(child); text != "" {
				return text
			}
		}
	case []any:
		for _, child := range value {
			if text := eventMessage(child); text != "" {
				return text
			}
		}
	}
	return ""
}

func (r *Runtime) environment(extra map[string]string) []string {
	allowed := map[string]bool{"PATH": true, "HOME": true, "TMPDIR": true, "LANG": true, "LC_ALL": true, "TERM": true, "XDG_CONFIG_HOME": true, "XDG_DATA_HOME": true, "XDG_STATE_HOME": true, "XDG_CACHE_HOME": true}
	env := make(map[string]string)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok && allowed[key] {
			env[key] = value
		}
	}
	for key, value := range r.config.Env {
		env[key] = value
	}
	for key, value := range extra {
		env[key] = value
	}
	result := make([]string, 0, len(env))
	for key, value := range env {
		result = append(result, key+"="+value)
	}
	return result
}

var secretPattern = regexp.MustCompile(`(?i)(bearer\s+|(?:api[_-]?key|token|secret|password)["'=:\s]+)([^\s"']+)`)

func (r *Runtime) redact(value string) string {
	value = secretPattern.ReplaceAllString(value, "$1[REDACTED]")
	for _, secret := range r.config.Secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}

type limitedBuffer struct {
	buffer  bytes.Buffer
	limit   int64
	written int64
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - b.written
	if remaining > 0 {
		if int64(len(p)) > remaining {
			p = p[:remaining]
		}
		_, _ = b.buffer.Write(p)
		b.written += int64(len(p))
	}
	return n, nil
}
func (b *limitedBuffer) String() string { return b.buffer.String() }

type redactingWriter struct {
	destination io.Writer
	redact      func(string) string
	pending     bytes.Buffer
}

func newRedactingWriter(destination io.Writer, redact func(string) string) *redactingWriter {
	return &redactingWriter{destination: destination, redact: redact}
}

func (w *redactingWriter) Write(p []byte) (int, error) {
	n := len(p)
	_, _ = w.pending.Write(p)
	for {
		line, err := w.pending.ReadString('\n')
		if err != nil {
			_, _ = w.pending.WriteString(line)
			if w.pending.Len() > 64<<10 {
				flush := w.pending.Next(w.pending.Len() - 4<<10)
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

func (w *redactingWriter) Close() error {
	if w.pending.Len() == 0 {
		return nil
	}
	_, err := io.WriteString(w.destination, w.redact(w.pending.String()))
	w.pending.Reset()
	return err
}

func randomID() (string, error) {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func valueOr(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
