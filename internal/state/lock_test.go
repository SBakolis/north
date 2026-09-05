package state

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunLockIsExclusiveAndTokenOwned(t *testing.T) {
	store := New(t.TempDir())
	lock, err := store.AcquireRunLock(context.Background(), "project-1", "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AcquireRunLock(context.Background(), "project-1", "run-1"); !errors.Is(err, ErrLocked) {
		t.Fatalf("second acquisition error = %v", err)
	}
	metadata, err := store.InspectRunLock("run-1")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Token != lock.Token() || metadata.PID == 0 || metadata.Hostname == "" {
		t.Fatalf("metadata = %#v", metadata)
	}
	if err := store.ReleaseRunLock("run-1", "wrong-token"); err == nil {
		t.Fatal("release with wrong token succeeded")
	}
	if err := store.ReleaseRunLock("run-1", lock.Token()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AcquireRunLock(context.Background(), "project-1", "run-1"); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
}

func TestLockStalenessIsObservational(t *testing.T) {
	created := time.Date(2026, 9, 5, 8, 0, 0, 0, time.UTC)
	metadata := LockMetadata{CreatedAt: created}
	if !metadata.Stale(created.Add(time.Hour), 30*time.Minute) {
		t.Fatal("expected stale lock")
	}
	if metadata.Stale(created.Add(time.Minute), 30*time.Minute) {
		t.Fatal("fresh lock classified stale")
	}
}
