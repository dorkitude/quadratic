package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareArchiveHandlesHistoryLargerThanSQLiteVariableLimit(t *testing.T) {
	store := newTestStore(t)
	const checkinCount = 32_767
	for i := range checkinCount {
		writeTestCheckin(t, store.checkinsDir, fmt.Sprintf("checkin-%d", i))
	}

	if err := store.PrepareArchive(context.Background()); err != nil {
		t.Fatalf("prepare archive: %v", err)
	}
	if got := store.countArchiveRows(); got != checkinCount {
		t.Fatalf("archive rows = %d, want %d", got, checkinCount)
	}
}

func TestPrepareArchiveRemovesRowsForDeletedBackups(t *testing.T) {
	store := newTestStore(t)
	firstPath := writeTestCheckin(t, store.checkinsDir, "first")
	writeTestCheckin(t, store.checkinsDir, "second")
	if err := store.PrepareArchive(context.Background()); err != nil {
		t.Fatalf("prepare initial archive: %v", err)
	}
	if err := os.Remove(firstPath); err != nil {
		t.Fatalf("remove backup: %v", err)
	}

	if err := store.PrepareArchive(context.Background()); err != nil {
		t.Fatalf("prepare archive after deletion: %v", err)
	}
	if got := store.countArchiveRows(); got != 1 {
		t.Fatalf("archive rows = %d, want 1", got)
	}
}

func TestPrepareArchiveSkipsUnchangedBackups(t *testing.T) {
	store := newTestStore(t)
	path := writeTestCheckin(t, store.checkinsDir, "unchanged")
	if err := store.PrepareArchive(context.Background()); err != nil {
		t.Fatalf("prepare initial archive: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if err := os.WriteFile(path, []byte(`not valid JSON`), 0o644); err != nil {
		t.Fatalf("replace backup: %v", err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatalf("restore backup mtime: %v", err)
	}

	if err := store.PrepareArchive(context.Background()); err != nil {
		t.Fatalf("prepare unchanged archive: %v", err)
	}
	if got := store.countArchiveRows(); got != 1 {
		t.Fatalf("archive rows = %d, want 1", got)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.db.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	return store
}

func writeTestCheckin(t *testing.T, checkinsDir, id string) string {
	t.Helper()
	path := filepath.Join(checkinsDir, id+".json")
	body := []byte(fmt.Sprintf(`{"id":%q,"createdAt":1700000000}`, id))
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write checkin %s: %v", id, err)
	}
	return path
}
