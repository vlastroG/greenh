// Package sqlitetest provides temp-file stores for tests.
package sqlitetest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vlastrog/greenh/store/sqlite"
)

// New opens a store in a fresh temp dir and closes it when the test ends.
func New(t *testing.T) *sqlite.Store {
	t.Helper()
	s, err := sqlite.Open(context.Background(), filepath.Join(TempDir(t), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	return s
}

// TempDir is t.TempDir with retried removal. On Windows, WAL mode intermittently leaves
// the db file delete-pending for a few ms after Close, although no handle of ours is open
// (journal_mode=DELETE never showed it). Register stores' Close after calling it, so Close
// runs before the removal.
func TempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "greenh-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() {
		for range 50 {
			if err = os.RemoveAll(dir); err == nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Errorf("remove %s: %v", dir, err)
	})
	return dir
}
