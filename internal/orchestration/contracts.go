package orchestration

import (
	"context"
	"time"

	"github.com/SBakolis/north/internal/knowledge"
	"github.com/SBakolis/north/internal/model"
)

type ProjectContext struct{ Root string }
type KnowledgeRequest struct{ ChangeID string }

type KnowledgeProvider interface {
	ID() string
	Detect(context.Context, ProjectContext) (bool, error)
	Load(context.Context, KnowledgeRequest) (knowledge.Snapshot, error)
}

type PlanningInput struct {
	Goal        string
	ProjectRoot string
	BaseRef     string
	Knowledge   knowledge.Snapshot
}
type Planner interface {
	CreatePlan(context.Context, PlanningInput) (model.ExecutionPlan, error)
}

type AgentRequest struct {
	RunID, StageID, Workspace, Prompt string
	Agent, SessionID, Role            string
	Timeout                           time.Duration
	Started                           func(AgentExecution) error
}
type AgentExecution struct {
	ExecutionID string
	PID         int
}
type AgentResult struct {
	ExecutionID, SessionID string
	Output                 string
	ExitCode               int
	PID                    int
}
type EventSink interface {
	Emit(context.Context, model.Event) error
}
type AgentRuntime interface {
	Validate(context.Context) error
	Execute(context.Context, AgentRequest, EventSink) (AgentResult, error)
	Cancel(context.Context, string) error
}

type IsolationRequest struct{ RunID, StageID, BaseCommit string }
type Workspace struct{ Path, Branch string }
type IsolationProvider interface {
	Prepare(context.Context, IsolationRequest) (Workspace, error)
	Cleanup(context.Context, Workspace) error
}

// ChangedPathInspector and WriteScopeVerifier keep repository-specific path
// handling outside the scheduler while making the host, not the agent,
// authoritative about what was changed.
type ChangedPathInspector interface {
	ChangedPaths(context.Context, string, string) ([]string, error)
}
type WriteScopeVerifier interface {
	VerifyWriteScope(context.Context, string, []string, []string) error
}

type ExactPathCommitRequest struct {
	RunID, StageID, Workspace, Message string
	Paths                              []string
}
type ExactPathCommitter interface {
	CommitPaths(context.Context, ExactPathCommitRequest) (string, error)
}

type VerificationRequest struct {
	RunID, StageID, Workspace string
	Criteria                  []model.AcceptanceCriterion
	WriteScope                []string
}
type VerificationResult struct {
	Passed   bool
	Evidence []string
	Failure  *model.StageFailure
}
type VerificationProvider interface {
	Verify(context.Context, VerificationRequest, EventSink) VerificationResult
}

type StageIntegrationRequest struct{ RunID, StageID, CommitSHA string }
type RunIntegrationRequest struct{ RunID, TargetBranch string }
type IntegrationResult struct {
	CommitSHA        string
	ConflictingPaths []string
	Err              error
}
type IntegrationProvider interface {
	IntegrateStage(context.Context, StageIntegrationRequest) IntegrationResult
	IntegrateRun(context.Context, RunIntegrationRequest) IntegrationResult
}

type ConflictResolutionRequest struct {
	RunID, StageID, IntegrationHead, CommitSHA string
	ConflictingPaths                           []string
	WriteScope                                 []string
	Criteria                                   []model.AcceptanceCriterion
	Started                                    func(AgentExecution) error
	Finished                                   func(AgentResult) error
}
type ConflictResolutionResult struct {
	CommitSHA string
	Evidence  []string
	Err       error
}
type ConflictResolutionProvider interface {
	ResolveConflict(context.Context, ConflictResolutionRequest, EventSink) ConflictResolutionResult
}

type StateStore interface {
	CreateRun(context.Context, model.RunState) error
	UpdateRun(context.Context, model.RunState) error
	UpdateStage(context.Context, string, model.StageState) error
	AppendEvent(context.Context, model.Event) error
	LoadRun(context.Context, string) (model.RunState, error)
	ListRuns(context.Context, string) ([]model.RunSummary, error)
}

// AtomicRunStateStore applies a read-modify-write operation while holding the
// store's cross-process run lock.
type AtomicRunStateStore interface {
	MutateRun(context.Context, string, func(*model.RunState) error) (model.RunState, error)
}

type RunLock interface{ Release() error }
type RunLockProvider interface {
	AcquireSchedulerRunLock(context.Context, string, string) (RunLock, error)
}

type RetryDecision struct {
	Retry      bool
	EligibleAt time.Time
}
type SchedulerPolicy interface {
	OrderReadyStages(context.Context, []model.StageState) []model.StageState
	RetryDecision(context.Context, model.StageFailure) RetryDecision
}

type FailureContext struct {
	Phase string
	Err   error
}
type FailureClassifier interface {
	Classify(context.Context, FailureContext) model.StageFailure
}
