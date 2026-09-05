package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SBakolis/north/internal/components"
	"github.com/SBakolis/north/internal/doctor"
	"github.com/SBakolis/north/internal/install"
	"github.com/SBakolis/north/internal/platform"
	"github.com/SBakolis/north/internal/plugins"
)

var errUsage = errors.New("usage: north <version|install|configure|update|status|doctor|uninstall|components|plan|graph|run|stage|integrate|cleanup>")

// Run dispatches the North command line without coupling commands to process globals.
func Run(stdin io.Reader, stdout, stderr io.Writer, args []string, version string) error {
	if len(args) == 0 {
		return errUsage
	}
	if args[0] == "version" {
		if len(args) != 1 {
			return errUsage
		}
		_, err := fmt.Fprintf(stdout, "north %s\n", version)
		return err
	}
	if args[0] == "components" {
		if len(args) != 2 || args[1] != "list" {
			return errUsage
		}
		for _, component := range components.BuiltinRegistry() {
			if _, err := fmt.Fprintf(stdout, "%s\t%s\n", component.ID, component.Name); err != nil {
				return err
			}
		}
		return nil
	}
	paths, err := platform.ResolvePaths(platform.OSEnvironment{})
	if err != nil {
		return err
	}
	switch args[0] {
	case "install", "configure", "update":
		return runInstall(stdin, stdout, stderr, args[0], args[1:], version, paths)
	case "plan":
		return runPlanCommand(stdout, stderr, args[1:], paths)
	case "graph":
		return runGraphCommand(stdout, stderr, args[1:], paths)
	case "run":
		return runExecutionCommand(stdout, stderr, args[1:], paths)
	case "stage":
		return runStageCommand(stdout, stderr, args[1:], paths)
	case "integrate":
		return runIntegrateCommand(stdout, stderr, args[1:], paths)
	case "cleanup":
		return runCleanupCommand(stdout, stderr, args[1:], paths)
	case "status":
		if len(args) != 1 {
			return errUsage
		}
		return printStatus(stdout, paths, false)
	case "doctor":
		return runDoctor(stdout, stderr, args[1:], version, paths)
	case "uninstall":
		return runUninstall(stdout, stderr, args[1:], paths)
	default:
		return errUsage
	}
}

func runInstall(stdin io.Reader, stdout, stderr io.Writer, command string, args []string, version string, paths platform.Paths) error {
	reader := bufio.NewReader(stdin)
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	dryRun := flags.Bool("dry-run", false, "show operations without writing")
	nonInteractive := flags.Bool("non-interactive", false, "disable prompts")
	agentSource := flags.String("agent-source", "", "AGENTS.md or AGENT.md")
	parallelization := flags.Bool("parallelization", true, "install parallel orchestration")
	noParallelization := flags.Bool("no-parallelization", false, "do not install parallel orchestration")
	knowledge := flags.String("knowledge", "none", "none or openspec")
	var enabledPlugins, disabledPlugins stringListFlag
	flags.Var(&enabledPlugins, "plugin", "enable an optional plugin (repeatable)")
	flags.Var(&disabledPlugins, "no-plugin", "disable an optional plugin (repeatable)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errUsage
	}
	provided := make(map[string]bool)
	flags.Visit(func(flag *flag.Flag) { provided[flag.Name] = true })
	if *noParallelization {
		*parallelization = false
	}
	if *knowledge != "none" && *knowledge != "openspec" {
		return fmt.Errorf("knowledge must be none or openspec")
	}
	selected := []string{"core", "knowledge.none", "parallelization"}
	existing, err := install.LoadManifestIfExists(install.ManifestPath(paths))
	if err != nil {
		return err
	}
	if existing != nil {
		selected = append([]string(nil), existing.Components...)
	}
	if *agentSource == "" && !*nonInteractive {
		differ, err := install.InstructionSourcesDiffer(paths.OpenCodeDir)
		if err != nil {
			return err
		}
		if differ {
			answer, err := promptLine(reader, stderr, "AGENTS.md and AGENT.md differ. Authoritative source [AGENTS.md/AGENT.md]: ")
			if err != nil {
				return err
			}
			*agentSource = answer
		}
	}
	if existing == nil && !*nonInteractive {
		if !provided["parallelization"] && !provided["no-parallelization"] {
			answer, err := promptLine(reader, stderr, "Enable parallel orchestration? [Y/n]: ")
			if err != nil {
				return err
			}
			*parallelization = answer == "" || strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")
		}
		if !provided["knowledge"] {
			answer, err := promptLine(reader, stderr, "Knowledge provider [none/openspec] (none): ")
			if err != nil {
				return err
			}
			if answer != "" {
				*knowledge = answer
			}
		}
		if !provided["plugin"] && !provided["no-plugin"] {
			for _, module := range []string{plugins.CodexMeter, plugins.OpenLoop} {
				answer, err := promptLine(reader, stderr, "Enable optional plugin "+module+"? [y/N]: ")
				if err != nil {
					return err
				}
				if strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes") {
					enabledPlugins = append(enabledPlugins, module)
				}
			}
		}
	}
	if *knowledge != "none" && *knowledge != "openspec" {
		return fmt.Errorf("knowledge must be none or openspec")
	}
	selected = withoutPrefix(selected, "plugin.")
	pluginModules := []string{}
	if existing != nil {
		for _, plugin := range existing.Plugins {
			pluginModules = append(pluginModules, plugin.Module)
		}
	}
	for _, module := range enabledPlugins {
		if !plugins.Supported(module) {
			return fmt.Errorf("unsupported plugin %q", module)
		}
		if !contains(pluginModules, module) {
			pluginModules = append(pluginModules, module)
		}
	}
	for _, module := range disabledPlugins {
		pluginModules = withoutValue(pluginModules, module)
	}
	if existing == nil || provided["knowledge"] {
		selected = withoutPrefix(selected, "knowledge.")
		selected = append(selected, "knowledge."+*knowledge)
	}
	if existing == nil || provided["parallelization"] || provided["no-parallelization"] {
		selected = withoutValue(selected, "parallelization")
		if *parallelization {
			selected = append(selected, "parallelization")
		}
	}
	if !*dryRun {
		commands := []string{"git", "opencode"}
		if contains(selected, "knowledge.openspec") {
			commands = append(commands, "npx")
		}
		for _, executable := range commands {
			if _, err := exec.LookPath(executable); err != nil {
				return fmt.Errorf("required executable %q not found: %w", executable, err)
			}
		}
		if contains(selected, "knowledge.openspec") {
			command := exec.CommandContext(context.Background(), "npx", "openspec", "--version")
			output, err := command.CombinedOutput()
			if err != nil {
				return fmt.Errorf("OpenSpec preflight failed: npx openspec --version: %w: %s", err, strings.TrimSpace(string(output)))
			}
			if strings.TrimSpace(string(output)) == "" {
				return errors.New("OpenSpec preflight failed: npx openspec --version returned no version")
			}
		}
	}
	result, err := install.Install(install.Options{
		Paths: paths, Version: version, Selected: selected, AgentSource: *agentSource,
		NonInteractive: *nonInteractive, DryRun: *dryRun, PluginModules: pluginModules,
		PluginPaths: pluginPathsFromEnvironment(paths),
	})
	if err != nil {
		return err
	}
	if *dryRun {
		for _, operation := range result.Operations {
			fmt.Fprintln(stdout, operation)
		}
		return nil
	}
	_, err = fmt.Fprintf(stdout, "North %s configured with %d components.\n", version, len(result.Manifest.Components))
	return err
}

func promptLine(reader *bufio.Reader, output io.Writer, prompt string) (string, error) {
	if _, err := fmt.Fprint(output, prompt); err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

type stringListFlag []string

func (values *stringListFlag) String() string { return strings.Join(*values, ",") }
func (values *stringListFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func withoutPrefix(values []string, prefix string) []string {
	result := values[:0]
	for _, value := range values {
		if !strings.HasPrefix(value, prefix) {
			result = append(result, value)
		}
	}
	return result
}

func withoutValue(values []string, unwanted string) []string {
	result := values[:0]
	for _, value := range values {
		if value != unwanted {
			result = append(result, value)
		}
	}
	return result
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func pluginPathsFromEnvironment(paths platform.Paths) plugins.Paths {
	global := []string{filepath.Join(paths.OpenCodeDir, "opencode.jsonc"), filepath.Join(paths.OpenCodeDir, "opencode.json")}
	if configured := os.Getenv("OPENCODE_CONFIG"); configured != "" {
		global = append([]string{configured}, global...)
	}
	tui := []string{filepath.Join(paths.OpenCodeDir, "tui.jsonc"), filepath.Join(paths.OpenCodeDir, "tui.json")}
	if configured := os.Getenv("OPENCODE_TUI_CONFIG"); configured != "" {
		tui = append([]string{configured}, tui...)
	}
	return plugins.Paths{Global: global, TUI: tui}
}

func runUninstall(stdout, stderr io.Writer, args []string, paths platform.Paths) error {
	flags := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dryRun := flags.Bool("dry-run", false, "show operations without writing")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errUsage
	}
	result, err := install.Uninstall(paths, *dryRun, install.PluginLifecycleOptions{Paths: pluginPathsFromEnvironment(paths)})
	if err != nil {
		return err
	}
	if *dryRun {
		for _, operation := range result.Operations {
			fmt.Fprintln(stdout, operation)
		}
		return nil
	}
	if len(result.Operations) == 0 {
		_, err = fmt.Fprintln(stdout, "North is not installed.")
	} else {
		_, err = fmt.Fprintln(stdout, "North uninstalled; stable backups were preserved.")
	}
	return err
}

func printStatus(stdout io.Writer, paths platform.Paths, asJSON bool) error {
	status, err := install.Inspect(paths)
	if err != nil {
		return err
	}
	if asJSON {
		return json.NewEncoder(stdout).Encode(struct {
			SchemaVersion int `json:"schemaVersion"`
			install.Status
		}{1, status})
	}
	if !status.Installed {
		_, err = fmt.Fprintln(stdout, "North is not installed.")
		return err
	}
	health := "healthy"
	if !status.Healthy {
		health = "unhealthy: " + strings.Join(status.Issues, "; ")
	}
	_, err = fmt.Fprintf(stdout, "North %s: %s (%s)\n", status.Version, health, strings.Join(status.Components, ", "))
	return err
}

func runDoctor(stdout, stderr io.Writer, args []string, version string, paths platform.Paths) error {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	asJSON := flags.Bool("json", false, "emit stable JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errUsage
	}
	report := doctor.Run(context.Background(), version, paths, ".")
	if *asJSON {
		if err := json.NewEncoder(stdout).Encode(report); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(stdout, "North %s (%s)\n", version, report.Platform)
		for _, check := range report.Checks {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", check.Severity, check.ID, check.Message)
		}
	}
	if !report.Healthy {
		return errors.New("doctor found actionable errors")
	}
	return nil
}
