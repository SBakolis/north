package plugins

import "fmt"

type Severity string

const (
	Warning Severity = "warning"
	Error   Severity = "error"
)

type Diagnostic struct {
	Code     string
	Severity Severity
	Message  string
	Paths    []string
}

type Verification struct {
	Module         string
	Registrations  []Registration
	GlobalOrServer bool
	TUI            bool
	Diagnostics    []Diagnostic
}

func verify(module string, regs []Registration) Verification {
	v := Verification{Module: module, Registrations: append([]Registration(nil), regs...)}
	for _, reg := range regs {
		v.GlobalOrServer = v.GlobalOrServer || reg.Role == RoleGlobal || reg.Role == RoleServer
		v.TUI = v.TUI || reg.Role == RoleTUI
	}
	if module == CodexMeter {
		if !v.GlobalOrServer {
			v.Diagnostics = append(v.Diagnostics, Diagnostic{Code: "codex_meter_server_registration_missing", Severity: Error, Message: "codex meter is not registered in a global/server candidate config"})
		}
		if !v.TUI {
			v.Diagnostics = append(v.Diagnostics, Diagnostic{Code: "codex_meter_tui_registration_missing", Severity: Error, Message: "codex meter is not registered in a TUI candidate config"})
		}
	}
	if module == OpenLoop {
		v.Diagnostics = append(v.Diagnostics, DiagnoseOpenLoop(regs)...)
	}
	return v
}

func Verify(module string, snapshots []Snapshot) (Verification, error) {
	regs, err := registrations(snapshots, module)
	if err != nil {
		return Verification{}, err
	}
	return verify(module, regs), nil
}

func DiagnoseOpenLoop(regs []Registration) []Diagnostic {
	var openLoop []Registration
	for _, reg := range regs {
		if reg.Module == OpenLoop {
			openLoop = append(openLoop, reg)
		}
	}
	if len(openLoop) < 2 {
		return nil
	}
	paths := make([]string, 0, len(openLoop))
	methods := map[RegistrationMethod]bool{}
	for _, reg := range openLoop {
		paths = append(paths, reg.Path)
		methods[reg.Method] = true
	}
	diagnostics := []Diagnostic{{Code: "open_loop_duplicate_registration", Severity: Error, Message: fmt.Sprintf("open-loop is registered %d times", len(openLoop)), Paths: paths}}
	if methods[StringRegistration] && methods[TupleRegistration] {
		diagnostics = append(diagnostics, Diagnostic{Code: "open_loop_conflicting_registration_methods", Severity: Error, Message: "open-loop uses both string and tuple registration methods", Paths: paths})
	}
	return diagnostics
}

// NorthRestrictedAgents documents the agents that open-loop must never
// autonomously continue. The representation is data-only and exposes no skill
// or MCP configuration.
var northRestrictedAgents = []string{"north-planner", "north-worker", "north-verifier", "north-conflict-resolver"}

func NorthAgentRestrictions() []string {
	return append([]string(nil), northRestrictedAgents...)
}

type OpenLoopConfig struct {
	RestrictedAgents []string `json:"restricted_agents"`
}

func RestrictedOpenLoopConfig() OpenLoopConfig {
	restricted := []string{"plan"}
	restricted = append(restricted, northRestrictedAgents...)
	return OpenLoopConfig{RestrictedAgents: restricted}
}

const RestrictedAgentsDocumentation = "The default plan agent and North agents are restricted from open-loop autonomous continuation: plan, north-planner, north-worker, north-verifier, north-conflict-resolver."
