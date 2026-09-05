// Package dag provides deterministic dependency graph operations for plans.
package dag

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/SBakolis/north/internal/model"
)

type Graph struct {
	nodes map[string]model.Stage
	deps  map[string][]string
}

func New(p model.ExecutionPlan) (*Graph, error) { return NewStages(p.Spec.Stages) }

func NewStages(stages []model.Stage) (*Graph, error) {
	g := &Graph{nodes: make(map[string]model.Stage, len(stages)), deps: make(map[string][]string, len(stages))}
	for _, stage := range stages {
		if stage.ID == "" {
			return nil, fmt.Errorf("stage ID is required")
		}
		if _, exists := g.nodes[stage.ID]; exists {
			return nil, fmt.Errorf("duplicate stage ID %q", stage.ID)
		}
		g.nodes[stage.ID] = stage
		g.deps[stage.ID] = append([]string(nil), stage.DependsOn...)
		sort.Strings(g.deps[stage.ID])
	}
	for id, dependencies := range g.deps {
		seen := map[string]bool{}
		for _, dependency := range dependencies {
			if dependency == id {
				return nil, fmt.Errorf("stage %q depends on itself", id)
			}
			if seen[dependency] {
				return nil, fmt.Errorf("stage %q has duplicate dependency %q", id, dependency)
			}
			seen[dependency] = true
			if _, exists := g.nodes[dependency]; !exists {
				return nil, fmt.Errorf("stage %q depends on unknown stage %q", id, dependency)
			}
		}
	}
	if _, err := g.TopologicalOrder(); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *Graph) IDs() []string {
	ids := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (g *Graph) Stage(id string) (model.Stage, bool) { stage, ok := g.nodes[id]; return stage, ok }

// TopologicalOrder uses Kahn's algorithm and lexical tie breaking.
func (g *Graph) TopologicalOrder() ([]string, error) {
	indegree := make(map[string]int, len(g.nodes))
	dependents := make(map[string][]string, len(g.nodes))
	for id, dependencies := range g.deps {
		indegree[id] = len(dependencies)
		for _, dependency := range dependencies {
			dependents[dependency] = append(dependents[dependency], id)
		}
	}
	ready := make([]string, 0)
	for id, degree := range indegree {
		if degree == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)
	order := make([]string, 0, len(g.nodes))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		order = append(order, id)
		for _, dependent := range dependents[id] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
		sort.Strings(ready)
	}
	if len(order) != len(g.nodes) {
		remaining := make([]string, 0)
		for id, degree := range indegree {
			if degree > 0 {
				remaining = append(remaining, id)
			}
		}
		sort.Strings(remaining)
		return nil, fmt.Errorf("dependency cycle includes %s", strings.Join(remaining, ", "))
	}
	return order, nil
}

// Ready returns unfinished stages whose dependencies are all complete. Stages
// present in running are excluded. Unknown completion/running IDs are ignored.
func (g *Graph) Ready(completed map[string]bool, runningSets ...map[string]bool) []string {
	var running map[string]bool
	if len(runningSets) > 0 {
		running = runningSets[0]
	}
	var ready []string
	for _, id := range g.IDs() {
		if completed[id] || running[id] {
			continue
		}
		eligible := true
		for _, dependency := range g.deps[id] {
			if !completed[dependency] {
				eligible = false
				break
			}
		}
		if eligible {
			ready = append(ready, id)
		}
	}
	return ready
}

func TopologicalOrder(p model.ExecutionPlan) ([]string, error) {
	g, err := New(p)
	if err != nil {
		return nil, err
	}
	return g.TopologicalOrder()
}

func ReadySet(p model.ExecutionPlan, completed map[string]bool, running ...map[string]bool) ([]string, error) {
	g, err := New(p)
	if err != nil {
		return nil, err
	}
	return g.Ready(completed, running...), nil
}

// Text emits one dependency declaration per line in lexical stage order.
func (g *Graph) Text() string {
	var out strings.Builder
	for _, id := range g.IDs() {
		fmt.Fprint(&out, id)
		if len(g.deps[id]) > 0 {
			fmt.Fprintf(&out, " <- %s", strings.Join(g.deps[id], ", "))
		}
		out.WriteByte('\n')
	}
	return out.String()
}

type JSONGraph struct {
	Nodes []string   `json:"nodes"`
	Edges []JSONEdge `json:"edges"`
}

type JSONEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (g *Graph) JSON() ([]byte, error) {
	representation := JSONGraph{Nodes: g.IDs(), Edges: []JSONEdge{}}
	for _, to := range g.IDs() {
		for _, from := range g.deps[to] {
			representation.Edges = append(representation.Edges, JSONEdge{From: from, To: to})
		}
	}
	data, err := json.MarshalIndent(representation, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func (g *Graph) DOT() string {
	var out strings.Builder
	out.WriteString("digraph north_plan {\n")
	for _, id := range g.IDs() {
		fmt.Fprintf(&out, "  %s;\n", dotQuote(id))
	}
	for _, to := range g.IDs() {
		for _, from := range g.deps[to] {
			fmt.Fprintf(&out, "  %s -> %s;\n", dotQuote(from), dotQuote(to))
		}
	}
	out.WriteString("}\n")
	return out.String()
}

func dotQuote(value string) string { encoded, _ := json.Marshal(value); return string(encoded) }

func RenderText(p model.ExecutionPlan) (string, error) {
	g, err := New(p)
	if err != nil {
		return "", err
	}
	return g.Text(), nil
}
func RenderJSON(p model.ExecutionPlan) ([]byte, error) {
	g, err := New(p)
	if err != nil {
		return nil, err
	}
	return g.JSON()
}
func RenderDOT(p model.ExecutionPlan) (string, error) {
	g, err := New(p)
	if err != nil {
		return "", err
	}
	return g.DOT(), nil
}
