package model

import (
	"encoding/json"
	"time"
)

type RunStatus string

const (
	RunPreparing        RunStatus = "Preparing"
	RunRunning          RunStatus = "Running"
	RunReadyToIntegrate RunStatus = "ReadyToIntegrate"
	RunCompleted        RunStatus = "Completed"
	RunFailed           RunStatus = "Failed"
	RunCancelled        RunStatus = "Cancelled"
)

type StageStatus string

const (
	StageWaitingForDependencies StageStatus = "WaitingForDependencies"
	StageReady                  StageStatus = "Ready"
	StagePreparing              StageStatus = "Preparing"
	StageRunning                StageStatus = "Running"
	StageVerifying              StageStatus = "Verifying"
	StageRetryScheduled         StageStatus = "RetryScheduled"
	StageCommitReady            StageStatus = "CommitReady"
	StageMerging                StageStatus = "Merging"
	StageMerged                 StageStatus = "Merged"
	StageMergeConflict          StageStatus = "MergeConflict"
	StageFailed                 StageStatus = "Failed"
	StageBlocked                StageStatus = "Blocked"
	StageNeedsHumanReview       StageStatus = "NeedsHumanReview"
	StageCancelled              StageStatus = "Cancelled"
	StageSkipped                StageStatus = "Skipped"
)

type RunState struct {
	SchemaVersion        int           `json:"schemaVersion"`
	ID                   string        `json:"id"`
	ProjectID            string        `json:"projectId"`
	Plan                 ExecutionPlan `json:"plan"`
	PlanHash             string        `json:"planHash"`
	BaseCommit           string        `json:"baseCommit"`
	RepositoryRoot       string        `json:"repositoryRoot,omitempty"`
	IntegrationBranch    string        `json:"integrationBranch"`
	IntegrationWorkspace string        `json:"integrationWorkspace,omitempty"`
	IntegrationHead      string        `json:"integrationHead,omitempty"`
	TargetBranch         string        `json:"targetBranch"`
	Status               RunStatus     `json:"status"`
	Stages               []StageState  `json:"stages"`
	Cancellation         *Cancellation `json:"cancellation,omitempty"`
	Failure              *StageFailure `json:"failure,omitempty"`
	PendingEvents        []Event       `json:"pendingEvents,omitempty"`
	CreatedAt            time.Time     `json:"createdAt"`
	UpdatedAt            time.Time     `json:"updatedAt"`
}

type StageState struct {
	SchemaVersion    int           `json:"schemaVersion"`
	ID               string        `json:"id"`
	Status           StageStatus   `json:"status"`
	Attempt          int           `json:"attempt"`
	RetryEligibleAt  time.Time     `json:"retryEligibleAt,omitempty"`
	Held             bool          `json:"held,omitempty"`
	HoldReason       string        `json:"holdReason,omitempty"`
	Workspace        string        `json:"workspace,omitempty"`
	Branch           string        `json:"branch,omitempty"`
	CommitSHA        string        `json:"commitSha,omitempty"`
	ExecutionID      string        `json:"executionId,omitempty"`
	SessionID        string        `json:"sessionId,omitempty"`
	WorkerPID        int           `json:"workerPid,omitempty"`
	StartedAt        time.Time     `json:"startedAt,omitempty"`
	LastActivity     time.Time     `json:"lastActivity,omitempty"`
	Failure          *StageFailure `json:"failure,omitempty"`
	ConflictingPaths StringList    `json:"conflictingPaths,omitempty"`
	ChangedPaths     StringList    `json:"changedPaths,omitempty"`
	Evidence         StringList    `json:"evidence,omitempty"`
}

// StringList stores a JSON string array in a comparable value, preserving the
// comparability of StageState used by durable snapshot consistency checks.
type StringList string

func NewStringList(values []string) StringList {
	if len(values) == 0 {
		return ""
	}
	encoded, _ := json.Marshal(values)
	return StringList(encoded)
}

func (l StringList) Values() []string {
	if l == "" {
		return nil
	}
	var values []string
	_ = json.Unmarshal([]byte(l), &values)
	return values
}

func (l StringList) MarshalJSON() ([]byte, error) {
	if l == "" {
		return []byte("[]"), nil
	}
	var values []string
	if err := json.Unmarshal([]byte(l), &values); err != nil {
		return nil, err
	}
	return json.Marshal(values)
}

func (l *StringList) UnmarshalJSON(data []byte) error {
	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	*l = NewStringList(values)
	return nil
}

type Cancellation struct {
	RequestedAt time.Time `json:"requestedAt"`
	Reason      string    `json:"reason,omitempty"`
}

type StageFailure struct {
	Class     string `json:"class"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type Event struct {
	ID            string         `json:"id,omitempty"`
	SchemaVersion int            `json:"schemaVersion"`
	Sequence      uint64         `json:"sequence"`
	Time          time.Time      `json:"time"`
	RunID         string         `json:"runId"`
	StageID       string         `json:"stageId,omitempty"`
	Type          string         `json:"type"`
	Message       string         `json:"message"`
	Data          map[string]any `json:"data,omitempty"`
}

type RunSummary struct {
	ID        string    `json:"id"`
	Status    RunStatus `json:"status"`
	UpdatedAt time.Time `json:"updatedAt"`
}
