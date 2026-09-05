// Package none provides the knowledge provider used when a project has no
// external knowledge system.
package none

import (
	"context"
	"sync"

	"github.com/SBakolis/north/internal/knowledge"
	"github.com/SBakolis/north/internal/orchestration"
)

const ID = "none"

type Provider struct {
	mu   sync.RWMutex
	root string
}

var _ orchestration.KnowledgeProvider = (*Provider)(nil)

func New() *Provider { return &Provider{} }

func NewProvider() *Provider { return New() }

func (*Provider) ID() string { return ID }

func (p *Provider) Detect(_ context.Context, project orchestration.ProjectContext) (bool, error) {
	p.mu.Lock()
	p.root = project.Root
	p.mu.Unlock()
	return true, nil
}

func (p *Provider) Load(context.Context, orchestration.KnowledgeRequest) (knowledge.Snapshot, error) {
	p.mu.RLock()
	root := p.root
	p.mu.RUnlock()
	return knowledge.Snapshot{Provider: ID, SourceRoot: root}, nil
}
