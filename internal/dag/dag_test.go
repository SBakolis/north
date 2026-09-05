package dag

import (
	"reflect"
	"strings"
	"testing"

	"github.com/SBakolis/north/internal/model"
)

func TestTopologicalOrderAndReadyAreDeterministic(t *testing.T) {
	g, err := NewStages([]model.Stage{
		{ID: "finish", DependsOn: []string{"beta", "alpha"}},
		{ID: "beta"},
		{ID: "alpha"},
	})
	if err != nil {
		t.Fatal(err)
	}
	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"alpha", "beta", "finish"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	if want := []string{"alpha", "beta"}; !reflect.DeepEqual(g.Ready(nil, nil), want) {
		t.Fatalf("ready = %v, want %v", g.Ready(nil, nil), want)
	}
	if want := []string{"finish"}; !reflect.DeepEqual(g.Ready(map[string]bool{"alpha": true, "beta": true}, nil), want) {
		t.Fatalf("ready = %v, want %v", g.Ready(map[string]bool{"alpha": true, "beta": true}, nil), want)
	}
}

func TestCycleRejected(t *testing.T) {
	_, err := NewStages([]model.Stage{{ID: "a", DependsOn: []string{"b"}}, {ID: "b", DependsOn: []string{"a"}}})
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("error = %v", err)
	}
}

func TestRenderers(t *testing.T) {
	g, err := NewStages([]model.Stage{{ID: "b", DependsOn: []string{"a"}}, {ID: "a"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := g.Text(); got != "a\nb <- a\n" {
		t.Fatalf("text = %q", got)
	}
	if got := g.DOT(); !strings.Contains(got, `"a" -> "b"`) {
		t.Fatalf("DOT = %q", got)
	}
	jsonGraph, err := g.JSON()
	if err != nil || !strings.Contains(string(jsonGraph), `"from": "a"`) {
		t.Fatalf("JSON = %s, error = %v", jsonGraph, err)
	}
}

func FuzzNewStages(f *testing.F) {
	f.Add("a", "b")
	f.Add("same", "same")
	f.Fuzz(func(t *testing.T, first, second string) {
		g, err := NewStages([]model.Stage{{ID: first}, {ID: second, DependsOn: []string{first}}})
		if err != nil {
			return
		}
		order, err := g.TopologicalOrder()
		if err != nil {
			t.Fatal(err)
		}
		if len(order) != 2 {
			t.Fatalf("order = %v", order)
		}
	})
}
