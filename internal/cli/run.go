package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/SBakolis/north/internal/application"
	"github.com/SBakolis/north/internal/approval"
	"github.com/SBakolis/north/internal/model"
	planpkg "github.com/SBakolis/north/internal/plan"
	"github.com/SBakolis/north/internal/platform"
)

func runExecutionCommand(stdout, _ io.Writer, args []string, paths platform.Paths) error {
	if len(args) == 0 {
		return errors.New("usage: north run <plan>|status|stop|resume")
	}
	manager := runtimeManager(paths)
	switch args[0] {
	case "status":
		return runStatus(stdout, args[1:], manager)
	case "stop":
		if len(args) != 2 && (len(args) != 3 || args[2] != "--dry-run") {
			return errors.New("usage: north run stop <run-id> [--dry-run]")
		}
		if len(args) == 3 {
			fmt.Fprintf(stdout, "request cancellation for run %s\n", args[1])
			return nil
		}
		if err := manager.Stop(context.Background(), ".", args[1], "operator requested stop"); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "cancellation requested")
		return nil
	case "resume":
		if len(args) != 2 && (len(args) != 3 || args[2] != "--dry-run") {
			return errors.New("usage: north run resume <run-id> [--dry-run]")
		}
		if len(args) == 3 {
			fmt.Fprintf(stdout, "resume run %s\n", args[1])
			return nil
		}
		stored, err := manager.Load(context.Background(), ".", args[1])
		if err != nil {
			return err
		}
		approved, err := (approval.Store{Path: filepath.Join(paths.StateDir, "approvals.json")}).IsApproved(stored.PlanHash)
		if err != nil {
			return err
		}
		manager.AllowShell = approved
		run, err := manager.Resume(context.Background(), ".", args[1])
		if err != nil {
			return err
		}
		return printRun(stdout, run, false)
	default:
		return runPlanExecution(stdout, args, paths, manager)
	}
}

func runPlanExecution(stdout io.Writer, args []string, paths platform.Paths, manager application.Manager) error {
	planFile := ""
	maxParallel := 0
	failFast, setFailFast := false, false
	approvePlan, autoIntegrate := false, false
	autoResolveConflicts := false
	dryRun := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--max-parallel":
			index++
			if index >= len(args) {
				return errors.New("--max-parallel requires a value")
			}
			value, err := strconv.Atoi(args[index])
			if err != nil || value < 1 {
				return errors.New("--max-parallel must be a positive integer")
			}
			maxParallel = value
		case "--fail-fast":
			failFast, setFailFast = true, true
		case "--approve-plan":
			approvePlan = true
		case "--auto-integrate":
			autoIntegrate = true
		case "--auto-resolve-conflicts":
			autoResolveConflicts = true
		case "--dry-run":
			dryRun = true
		default:
			if strings.HasPrefix(args[index], "-") || planFile != "" {
				return fmt.Errorf("unknown run argument %q", args[index])
			}
			planFile = args[index]
		}
	}
	if planFile == "" {
		return errors.New("execution plan file is required")
	}
	executionPlan, err := readPlan(planFile)
	if err != nil {
		return err
	}
	if autoResolveConflicts {
		executionPlan.Spec.Policy.AutoResolveConflicts = true
	}
	if maxParallel > 0 {
		if maxParallel > planpkg.MaxParallel {
			return fmt.Errorf("--max-parallel must not exceed %d", planpkg.MaxParallel)
		}
		executionPlan.Spec.Policy.MaxParallel = maxParallel
	}
	if setFailFast {
		executionPlan.Spec.Policy.FailFast = failFast
	}
	hash, err := planpkg.ApprovalHash(executionPlan)
	if err != nil {
		return err
	}
	approvalStore := approval.Store{Path: paths.StateDir + string(os.PathSeparator) + "approvals.json"}
	approved, err := approvalStore.IsApproved(hash)
	if err != nil {
		return err
	}
	if approvePlan {
		if !dryRun {
			if err := approvalStore.Approve(hash); err != nil {
				return err
			}
		}
		approved = true
	}
	if hasHostCommands(executionPlan) && !approved {
		if dryRun {
			return printRunDryRun(stdout, executionPlan, hash, false)
		}
		return fmt.Errorf("plan %s contains host commands and is not approved; run `north plan approve %s` or pass --approve-plan", hash, planFile)
	}
	if dryRun {
		return printRunDryRun(stdout, executionPlan, hash, approved)
	}
	manager.AllowShell = approved
	run, err := manager.Start(context.Background(), ".", executionPlan, application.RunOptions{})
	if err != nil {
		return err
	}
	if err := printRun(stdout, run, false); err != nil {
		return err
	}
	if autoIntegrate && run.Status == model.RunReadyToIntegrate {
		run, err = manager.Integrate(context.Background(), ".", run.ID, "")
		if err != nil {
			return err
		}
		return printRun(stdout, run, false)
	}
	return nil
}

func runStatus(stdout io.Writer, args []string, manager application.Manager) error {
	runID := ""
	asJSON, watch := false, false
	for _, arg := range args {
		switch arg {
		case "--json":
			asJSON = true
		case "--watch":
			watch = true
		default:
			if strings.HasPrefix(arg, "-") || runID != "" {
				return fmt.Errorf("unknown run status argument %q", arg)
			}
			runID = arg
		}
	}
	if runID == "" {
		runs, err := manager.List(context.Background(), ".")
		if err != nil {
			return err
		}
		if watch && len(runs) > 0 {
			runID = runs[0].ID
		} else if asJSON {
			return json.NewEncoder(stdout).Encode(struct {
				SchemaVersion int                `json:"schemaVersion"`
				Runs          []model.RunSummary `json:"runs"`
			}{1, runs})
		} else {
			for _, run := range runs {
				fmt.Fprintf(stdout, "%s\t%s\t%s\n", run.ID, run.Status, run.UpdatedAt.Format(time.RFC3339))
			}
			return nil
		}
	}
	for {
		run, err := manager.Load(context.Background(), ".", runID)
		if err != nil {
			return err
		}
		if err := printRun(stdout, run, asJSON); err != nil {
			return err
		}
		if !watch || run.Status != model.RunRunning && run.Status != model.RunPreparing {
			return nil
		}
		time.Sleep(time.Second)
	}
}

func runStageCommand(stdout, _ io.Writer, args []string, paths platform.Paths) error {
	if len(args) != 3 && (len(args) != 4 || args[3] != "--dry-run") {
		return errors.New("usage: north stage <retry|hold|release> <run-id> <stage-id> [--dry-run]")
	}
	if len(args) == 4 {
		fmt.Fprintf(stdout, "%s stage %s in run %s\n", args[0], args[2], args[1])
		return nil
	}
	manager := runtimeManager(paths)
	var err error
	switch args[0] {
	case "retry":
		err = manager.Retry(context.Background(), ".", args[1], args[2])
	case "hold":
		err = manager.SetHold(context.Background(), ".", args[1], args[2], "operator hold", true)
	case "release":
		err = manager.SetHold(context.Background(), ".", args[1], args[2], "", false)
	default:
		return errors.New("usage: north stage <retry|hold|release> <run-id> <stage-id>")
	}
	if err == nil {
		fmt.Fprintln(stdout, "updated")
	}
	return err
}

func runIntegrateCommand(stdout, _ io.Writer, args []string, paths platform.Paths) error {
	if len(args) < 1 {
		return errors.New("usage: north integrate <run-id> [--target branch] [--dry-run]")
	}
	runID, target, dryRun := args[0], "", false
	for index := 1; index < len(args); index++ {
		switch args[index] {
		case "--target":
			index++
			if index >= len(args) {
				return errors.New("--target requires a value")
			}
			target = args[index]
		case "--dry-run":
			dryRun = true
		default:
			return fmt.Errorf("unknown integrate argument %q", args[index])
		}
	}
	if dryRun {
		fmt.Fprintf(stdout, "integrate run %s into %s\n", runID, valueOr(target, "recorded target"))
		return nil
	}
	run, err := runtimeManager(paths).Integrate(context.Background(), ".", runID, target)
	if err != nil {
		return err
	}
	return printRun(stdout, run, false)
}

func runCleanupCommand(stdout, _ io.Writer, args []string, paths platform.Paths) error {
	if len(args) < 1 || len(args) > 2 || len(args) == 2 && args[1] != "--dry-run" {
		return errors.New("usage: north cleanup <run-id> [--dry-run]")
	}
	if len(args) == 2 {
		fmt.Fprintf(stdout, "cleanup worktrees and branches for run %s\n", args[0])
		return nil
	}
	if err := runtimeManager(paths).Cleanup(context.Background(), ".", args[0]); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "cleanup complete")
	return nil
}

func runtimeManager(paths platform.Paths) application.Manager {
	return application.Manager{Paths: paths, OpenCodeBinary: os.Getenv("NORTH_OPENCODE_BINARY"), StageTimeout: 2 * time.Hour}
}

func printRun(stdout io.Writer, run model.RunState, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(stdout).Encode(struct {
			SchemaVersion int            `json:"schemaVersion"`
			Run           model.RunState `json:"run"`
		}{1, run})
	}
	fmt.Fprintf(stdout, "%s\t%s\n", run.ID, run.Status)
	for _, stage := range run.Stages {
		fmt.Fprintf(stdout, "%s\t%s\tattempt=%d\n", stage.ID, stage.Status, stage.Attempt)
	}
	return nil
}

func hasHostCommands(plan model.ExecutionPlan) bool {
	for _, stage := range plan.Spec.Stages {
		for _, criterion := range stage.Acceptance {
			if criterion.Type == "exec" || criterion.Type == "command" || criterion.Type == "shell" {
				return true
			}
		}
	}
	return false
}

func valueOr(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func printRunDryRun(stdout io.Writer, executionPlan model.ExecutionPlan, hash string, approved bool) error {
	fmt.Fprintf(stdout, "plan hash: %s\napproved: %t\nbase: %s\nmax parallel: %d\n", hash, approved, executionPlan.Spec.BaseRef, executionPlan.Spec.Policy.MaxParallel)
	for _, stage := range executionPlan.Spec.Stages {
		fmt.Fprintf(stdout, "stage %s depends=[%s] scope=[%s]\n", stage.ID, strings.Join(stage.DependsOn, ","), strings.Join(stage.WriteScope, ","))
		for _, criterion := range stage.Acceptance {
			if criterion.Type == "exec" || criterion.Type == "command" || criterion.Type == "shell" {
				fmt.Fprintf(stdout, "host command %s/%s: %q\n", stage.ID, criterion.ID, criterion.Command)
			}
		}
	}
	return nil
}
