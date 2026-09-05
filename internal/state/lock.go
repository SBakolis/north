package state

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/SBakolis/north/internal/orchestration"
)

type LockMetadata struct {
	Token     string    `json:"token"`
	Scope     string    `json:"scope"`
	ProjectID string    `json:"projectId"`
	RunID     string    `json:"runId,omitempty"`
	PID       int       `json:"pid"`
	Hostname  string    `json:"hostname"`
	CreatedAt time.Time `json:"createdAt"`
}

// Stale reports age-based staleness. It deliberately does not remove the lock.
func (m LockMetadata) Stale(now time.Time, maxAge time.Duration) bool {
	return maxAge >= 0 && !m.CreatedAt.IsZero() && now.Sub(m.CreatedAt) > maxAge
}

type LockHeldError struct {
	Path     string
	Metadata LockMetadata
	Cause    error
}

func (e *LockHeldError) Error() string { return fmt.Sprintf("%v: %s", ErrLocked, e.Path) }
func (e *LockHeldError) Unwrap() error { return ErrLocked }

// FileLock can only release a lock whose on-disk ownership token still matches.
type FileLock struct {
	path  string
	token string
	mu    sync.Mutex
	done  bool
}

func (l *FileLock) Token() string { return l.token }
func (l *FileLock) Path() string  { return l.path }

func (l *FileLock) Release() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.done {
		return nil
	}
	if err := removeOwnedLock(l.path, l.token); err != nil {
		return err
	}
	l.done = true
	return nil
}

func (s *Store) AcquireRepositoryLock(ctx context.Context, projectID string) (*FileLock, error) {
	if projectID == "" {
		return nil, errors.New("acquire repository lock: project ID is empty")
	}
	return s.acquireOwnedLock(ctx, filepath.Join(s.root, ".locks", "repository.lock"), LockMetadata{Scope: "repository", ProjectID: projectID})
}

func (s *Store) AcquireRunLock(ctx context.Context, projectID, runID string) (*FileLock, error) {
	if projectID == "" {
		return nil, errors.New("acquire run lock: project ID is empty")
	}
	runDir, err := s.runDir(runID)
	if err != nil {
		return nil, err
	}
	return s.acquireOwnedLock(ctx, filepath.Join(runDir, ".run.lock"), LockMetadata{Scope: "run", ProjectID: projectID, RunID: runID})
}

func (s *Store) AcquireSchedulerRunLock(ctx context.Context, projectID, runID string) (orchestration.RunLock, error) {
	return s.AcquireRunLock(ctx, projectID, runID)
}

func (s *Store) InspectRepositoryLock() (LockMetadata, error) {
	return inspectLock(filepath.Join(s.root, ".locks", "repository.lock"))
}

func (s *Store) InspectRunLock(runID string) (LockMetadata, error) {
	runDir, err := s.runDir(runID)
	if err != nil {
		return LockMetadata{}, err
	}
	return inspectLock(filepath.Join(runDir, ".run.lock"))
}

// ReleaseRepositoryLock releases a lock only when token matches its persisted
// owner. It is intended for explicit recovery after inspecting stale metadata.
func (s *Store) ReleaseRepositoryLock(token string) error {
	return releaseOwnedLock(filepath.Join(s.root, ".locks", "repository.lock"), token)
}

// ReleaseRunLock releases a lock only when token matches its persisted owner.
func (s *Store) ReleaseRunLock(runID, token string) error {
	runDir, err := s.runDir(runID)
	if err != nil {
		return err
	}
	return releaseOwnedLock(filepath.Join(runDir, ".run.lock"), token)
}

func (s *Store) acquireOwnedLock(ctx context.Context, path string, metadata LockMetadata) (*FileLock, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	hostname, _ := os.Hostname()
	metadata.Token = token
	metadata.PID = os.Getpid()
	metadata.Hostname = hostname
	metadata.CreatedAt = s.now().UTC()
	data, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		existing, inspectErr := inspectLock(path)
		return nil, &LockHeldError{Path: path, Metadata: existing, Cause: inspectErr}
	}
	if err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("write lock metadata: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("sync lock metadata: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("close lock metadata: %w", err)
	}
	return &FileLock{path: path, token: token}, nil
}

func inspectLock(path string) (LockMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LockMetadata{}, err
	}
	var metadata LockMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return LockMetadata{}, fmt.Errorf("decode lock metadata: %w", err)
	}
	if metadata.Token == "" || metadata.Scope == "" || metadata.CreatedAt.IsZero() {
		return LockMetadata{}, errors.New("invalid lock metadata")
	}
	return metadata, nil
}

func releaseOwnedLock(path, token string) error {
	if token == "" {
		return errors.New("release lock: ownership token is empty")
	}
	return removeOwnedLock(path, token)
}

func removeOwnedLock(path, token string) error {
	claimToken, err := randomToken()
	if err != nil {
		return err
	}
	claim := path + ".release." + claimToken
	if err := os.Link(path, claim); err != nil {
		return fmt.Errorf("claim lock for release: %w", err)
	}
	defer os.Remove(claim)
	metadata, err := inspectLock(claim)
	if err != nil {
		return fmt.Errorf("inspect lock for release: %w", err)
	}
	if metadata.Token != token {
		return errors.New("release lock: ownership token does not match")
	}
	original, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect lock for release: %w", err)
	}
	claimed, err := os.Stat(claim)
	if err != nil {
		return fmt.Errorf("inspect lock claim: %w", err)
	}
	if !os.SameFile(original, claimed) {
		return errors.New("release lock: lock changed while checking ownership")
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("release lock: %w", err)
	}
	return nil
}

func (s *Store) acquireMutationLock(ctx context.Context, runDir string) (func(), error) {
	path := filepath.Join(runDir, ".state.lock")
	deadline := time.Now().Add(5 * time.Second)
	for {
		lock, err := s.acquireOwnedLock(ctx, path, LockMetadata{Scope: "state-write", RunID: filepath.Base(runDir)})
		if err == nil {
			return func() { _ = lock.Release() }, nil
		}
		if !errors.Is(err, ErrLocked) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func randomToken() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate lock token: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}
