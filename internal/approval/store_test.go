package approval

import (
	"path/filepath"
	"testing"
)

func TestApprovalIsBoundToExactHash(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "approvals.json")}
	if approved, err := store.IsApproved("one"); err != nil || approved {
		t.Fatalf("before approval = %v, %v", approved, err)
	}
	if err := store.Approve("one"); err != nil {
		t.Fatal(err)
	}
	if approved, err := store.IsApproved("one"); err != nil || !approved {
		t.Fatalf("approved hash = %v, %v", approved, err)
	}
	if approved, err := store.IsApproved("two"); err != nil || approved {
		t.Fatalf("different hash = %v, %v", approved, err)
	}
}
