// Package sqlitetest provides a temp-file store for tests.
package sqlitetest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/vlastrog/greenh/store/sqlite"
)

func New(t *testing.T) *sqlite.Store {
	t.Helper()
	s, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
