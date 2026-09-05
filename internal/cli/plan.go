package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SBakolis/north/internal/approval"
	"github.com/SBakolis/north/internal/dag"
	"github.com/SBakolis/north/internal/knowledge"
	"github.com/SBakolis/north/internal/knowledge/openspec"
	"github.com/SBakolis/north/internal/model"
	opencodeadapter "github.com/SBakolis/north/internal/opencode"
	"github.com/SBakolis/north/internal/orchestration"
	planpkg "github.com/SBakolis/north/internal/plan"
	planneradapter "github.com/SBakolis/north/internal/planner"
	"github.com/SBakolis/north/internal/platform"
)

func runPlanCommand(stdout, _ io.Writer, args []string, paths platform.Paths) error {
	if len(args) == 0 {
		return errors.New("usage: north plan <create|validate|approve>")
	}
	switch args[0] {
	case "validate":
		return planValidate(stdout, args[1:], paths)
	case "approve":
		return planApprove(stdout, args[1:], paths)
	case "create":
		return planCreate(stdout, args[1:], paths)
	default:
		return errors.New("usage: north plan <create|validate|approve>")
	}
}

func planValidate(stdout io.Writer, args []string, paths platform.Paths) error {
	file, jsonOutput, _, err := parsePlanFileFlags(args, true)
	if err != nil {
		return err
	}
	executionPlan, err := readPlan(file)
	if err != nil {
		return err
	}
	if err := validatePlanEnvironment(executionPlan, paths); err != nil {
		return err
	}
	warnings := planpkg.Warnings(executionPlan)
	if jsonOutput {
		return json.NewEncoder(stdout).Encode(struct {
			SchemaVersion int               `json:"schemaVersion"`
			Valid         bool              `json:"valid"`
			Warnings      []planpkg.Warning `json:"warnings"`
		}{1, true, warnings})
	}
	fmt.Fprintf(stdout, "valid: %s\n", file)
	for _, warning := range warnings {
		fmt.Fprintf(stdout, "warning [%s]: %s\n", warning.Code, warning.Message)
	}
	return nil
}

func planApprove(stdout io.Writer, args []string, paths platform.Paths) error {
	file, _, dryRun, err := parsePlanFileFlags(args, false)
	if err != nil {
		return err
	}
	executionPlan, err := readPlan(file)
	if err != nil {
		return err
	}
	hash, err := planpkg.ApprovalHash(executionPlan)
	if err != nil {
		return err
	}
	if !dryRun {
		if err := (approval.Store{Path: filepath.Join(paths.StateDir, "approvals.json")}).Approve(hash); err != nil {
			return err
		}
	}
	fmt.Fprintln(stdout, hash)
	return nil
}

func parsePlanFileFlags(args []string, allowJSON bool) (file string, jsonOutput, dryRun bool, err error) {
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--json":
			if !allowJSON {
				return "", false, false, errors.New("--json is not supported here")
			}
			jsonOutput = true
		case "--strict":
			// Parsing is strict from the first schema version.
		case "--dry-run":
			dryRun = true
		default:
			if strings.HasPrefix(args[index], "-") || file != "" {
				if allowJSON {
					return "", false, false, errors.New("usage: north plan validate <file> [--strict] [--json]")
				}
				return "", false, false, errors.New("usage: north plan approve <file> [--dry-run]")
			}
			file = args[index]
		}
	}
	if file == "" {
		return "", false, false, errors.New("plan file is required")
	}
	return file, jsonOutput, dryRun, nil
}

func planCreate(stdout io.Writer, args []string, paths platform.Paths) error {
	runtime := opencodeadapter.New(opencodeadapter.Config{
		Binary: os.Getenv("NORTH_OPENCODE_BINARY"), LogDir: filepath.Join(paths.StateDir, "logs", "planner"), StateDir: paths.StateDir,
	})
	return planCreateWithPlanner(context.Background(), stdout, args, paths, &planneradapter.AgentPlanner{Runtime: runtime})
}

func planCreateWithPlanner(ctx context.Context, stdout io.Writer, args []string, paths platform.Paths, planner orchestration.Planner) error {
	var goal, providerID, changeID, output string
	dryRun := false
	providerID = "none"
	for index := 0; index < len(args); index++ {
		value := func() (string, error) {
			index++
			if index >= len(args) {
				return "", fmt.Errorf("%s requires a value", args[index-1])
			}
			return args[index], nil
		}
		var err error
		switch args[index] {
		case "--goal":
			goal, err = value()
		case "--knowledge":
			providerID, err = value()
		case "--change":
			changeID, err = value()
		case "--output", "-o":
			output, err = value()
		case "--dry-run":
			dryRun = true
		default:
			return fmt.Errorf("unknown plan create argument %q", args[index])
		}
		if err != nil {
			return err
		}
	}
	var snapshot knowledge.Snapshot
	if providerID == "openspec" {
		provider := openspec.New()
		detected, err := provider.Detect(ctx, orchestration.ProjectContext{Root: "."})
		if err != nil {
			return fmt.Errorf("OpenSpec knowledge is unavailable: %w", err)
		}
		if !detected {
			return errors.New("OpenSpec knowledge is unavailable: openspec/config.yaml was not found")
		}
		snapshot, err = provider.Load(ctx, orchestration.KnowledgeRequest{ChangeID: changeID})
		if err != nil {
			return err
		}
		if goal == "" && snapshot.Change != nil {
			goal = strings.TrimSpace(snapshot.Change.Title + ": " + snapshot.Change.Summary)
		}
	} else if providerID != "none" {
		return fmt.Errorf("knowledge must be none or openspec")
	}
	if strings.TrimSpace(goal) == "" {
		return errors.New("--goal is required when knowledge does not provide one")
	}
	base := currentGitBranch()
	root, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	executionPlan, err := planner.CreatePlan(ctx, orchestration.PlanningInput{Goal: goal, ProjectRoot: root, BaseRef: base, Knowledge: snapshot})
	if err != nil {
		return err
	}
	if err := validatePlanEnvironment(executionPlan, paths); err != nil {
		return err
	}
	data, err := planpkg.MarshalYAML(executionPlan)
	if err != nil {
		return err
	}
	if output != "" && !dryRun {
		return platform.WriteFileAtomic(output, data, 0o600)
	}
	_, err = stdout.Write(data)
	return err
}

func runGraphCommand(stdout, _ io.Writer, args []string, paths platform.Paths) error {
	format, file := "text", ""
	for index := 0; index < len(args); index++ {
		if args[index] == "--format" {
			index++
			if index >= len(args) {
				return errors.New("--format requires a value")
			}
			format = args[index]
		} else if file == "" {
			file = args[index]
		} else {
			return errors.New("usage: north graph <plan> [--format text|dot|json]")
		}
	}
	if file == "" {
		return errors.New("graph plan file is required")
	}
	executionPlan, err := readPlanOrRun(file, paths)
	if err != nil {
		return err
	}
	graph, err := dag.New(executionPlan)
	if err != nil {
		return err
	}
	switch format {
	case "text":
		_, err = io.WriteString(stdout, graph.Text())
	case "dot":
		_, err = io.WriteString(stdout, graph.DOT())
	case "json":
		var data []byte
		data, err = graph.JSON()
		if err == nil {
			_, err = stdout.Write(data)
		}
	default:
		return fmt.Errorf("unsupported graph format %q", format)
	}
	return err
}

func readPlanOrRun(identifier string, paths platform.Paths) (model.ExecutionPlan, error) {
	executionPlan, err := readPlan(identifier)
	if err == nil {
		return executionPlan, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return model.ExecutionPlan{}, err
	}
	run, runErr := runtimeManager(paths).Load(context.Background(), ".", identifier)
	if runErr != nil {
		return model.ExecutionPlan{}, fmt.Errorf("read plan file or run %q: %w", identifier, errors.Join(err, runErr))
	}
	return run.Plan, nil
}

func validatePlanEnvironment(executionPlan model.ExecutionPlan, paths platform.Paths) error {
	command := exec.Command("git", "rev-parse", "--verify", executionPlan.Spec.BaseRef+"^{commit}")
	command.Dir = "."
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("resolve baseRef %q: %w: %s", executionPlan.Spec.BaseRef, err, strings.TrimSpace(string(output)))
	}
	for _, stage := range executionPlan.Spec.Stages {
		if stage.Agent == "" {
			continue
		}
		if err := planpkg.ValidateAgentReference(stage.Agent); err != nil {
			return fmt.Errorf("stage %q references invalid agent %q: %w", stage.ID, stage.Agent, err)
		}
		path := filepath.Join(paths.OpenCodeDir, "agents", stage.Agent+".md")
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("stage %q references unavailable agent %q at %s: %w", stage.ID, stage.Agent, path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("stage %q references agent %q at non-file path %s", stage.ID, stage.Agent, path)
		}
	}
	return nil
}

func readPlan(path string) (model.ExecutionPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.ExecutionPlan{}, err
	}
	return planpkg.Parse(data)
}

func currentGitBranch() string {
	output, err := exec.Command("git", "branch", "--show-current").Output()
	if err == nil && strings.TrimSpace(string(output)) != "" {
		return strings.TrimSpace(string(output))
	}
	return "main"
}
