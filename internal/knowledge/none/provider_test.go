package none

import (
	"context"
	"testing"

	"github.com/SBakolis/north/internal/orchestration"
)

func TestProvider(t *testing.T) {
	p := New()
	ok, err := p.Detect(context.Background(), orchestration.ProjectContext{Root: "/project"})
	if err != nil || !ok {
		t.Fatalf("Detect() = %v, %v", ok, err)
	}
	snapshot, err := p.Load(context.Background(), orchestration.KnowledgeRequest{ChangeID: "ignored"})
	if err != nil || snapshot.Provider != ID || snapshot.SourceRoot != "/project" {
		t.Fatalf("Load() = %#v, %v", snapshot, err)
	}
}
