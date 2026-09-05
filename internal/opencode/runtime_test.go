package opencode

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SBakolis/north/internal/model"
	"github.com/SBakolis/north/internal/orchestration"
)

type sink struct{ events []model.Event }

func (s *sink) Emit(_ context.Context, event model.Event) error {
	s.events = append(s.events, event)
	return nil
}

func TestRuntimeDoesNotBlockOnLongNewlineFreeOutput(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "opencode")
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then echo 'opencode 1.0'; exit 0; fi
if [ "$1" = "run" ] && [ "$2" = "--help" ]; then echo '--dir --agent --format'; exit 0; fi
i=0
while [ "$i" -lt 100000 ]; do printf x; i=$((i + 1)); done
`
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	runtime := New(Config{Binary: binary, LogDir: filepath.Join(dir, "logs"), OutputCap: 64 << 10})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := runtime.Execute(ctx, orchestration.AgentRequest{Workspace: dir, Prompt: "work"}, nil)
	if err == nil || ctx.Err() != nil {
		t.Fatalf("error = %v, context = %v", err, ctx.Err())
	}
}

func TestRuntimeValidationExecutionSessionAndRedaction(t *testing.T) {
	dir := t.TempDir()
	logs := filepath.Join(dir, "logs")
	argsPath := filepath.Join(dir, "args")
	rolePath := filepath.Join(dir, "role")
	binary := filepath.Join(dir, "opencode")
	script := `#!/bin/sh
set -eu
if [ "$1" = "--version" ]; then echo 'opencode 1.0'; exit 0; fi
if [ "$1" = "run" ] && [ "$2" = "--help" ]; then echo '--dir --agent --format'; exit 0; fi
printf '%s\n' "$@" > "$ARG_FILE"
printf '%s\n' "$NORTH_AGENT_ROLE" > "$ROLE_FILE"
printf '%s\n' '{"type":"start","session":{"sessionID":"session-1"},"token":"topsecret"}' 'not-json'
printf '%s\n' 'token=topsecret' >&2
`
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	runtime := New(Config{Binary: binary, Agent: "worker", LogDir: logs, Env: map[string]string{"ARG_FILE": argsPath, "ROLE_FILE": rolePath}, Secrets: []string{"topsecret"}})
	if err := runtime.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := &sink{}
	started := orchestration.AgentExecution{}
	result, err := runtime.Execute(context.Background(), orchestration.AgentRequest{RunID: "run", StageID: "stage", Workspace: dir, Prompt: "do work", Role: "conflict-resolver", Started: func(value orchestration.AgentExecution) error {
		started = value
		return nil
	}}, s)
	if err != nil {
		t.Fatal(err)
	}
	if result.SessionID != "session-1" || result.ExecutionID == "" {
		t.Fatalf("result = %#v", result)
	}
	if started.ExecutionID != result.ExecutionID || started.PID <= 0 {
		t.Fatalf("started = %#v, result = %#v", started, result)
	}
	if role, _ := os.ReadFile(rolePath); strings.TrimSpace(string(role)) != "conflict-resolver" {
		t.Fatalf("role = %q", role)
	}
	args, _ := os.ReadFile(argsPath)
	if got := string(args); !strings.Contains(got, "--dir\n"+dir+"\n--agent\nworker\n--format\njson\ndo work") {
		t.Fatalf("argv = %q", got)
	}
	if len(s.events) != 2 || s.events[1].Type != "opencode.unknown" {
		t.Fatalf("events = %#v", s.events)
	}
	stdout, _ := os.ReadFile(filepath.Join(logs, result.ExecutionID+".stdout.jsonl"))
	stderr, _ := os.ReadFile(filepath.Join(logs, result.ExecutionID+".stderr.log"))
	if strings.Contains(string(stdout)+string(stderr), "topsecret") {
		t.Fatal("secret persisted in logs")
	}
}
