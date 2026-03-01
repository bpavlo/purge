package checkpoint

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")
	m, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

func TestSaveAndLoad(t *testing.T) {
	m := newTestManager(t)

	cp := Checkpoint{
		Operation:       "delete",
		Platform:        "discord",
		ServerID:        "srv-1",
		LastProcessedID: "msg-42",
		DeletedCount:    10,
		FailedCount:     2,
		SkippedCount:    1,
		StartedAt:       time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
	}

	if err := m.Save(cp); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil checkpoint")
	}

	if loaded.Operation != "delete" {
		t.Errorf("expected operation 'delete', got %q", loaded.Operation)
	}
	if loaded.Platform != "discord" {
		t.Errorf("expected platform 'discord', got %q", loaded.Platform)
	}
	if loaded.LastProcessedID != "msg-42" {
		t.Errorf("expected last processed 'msg-42', got %q", loaded.LastProcessedID)
	}
	if loaded.DeletedCount != 10 {
		t.Errorf("expected 10 deleted, got %d", loaded.DeletedCount)
	}
	if loaded.FailedCount != 2 {
		t.Errorf("expected 2 failed, got %d", loaded.FailedCount)
	}
	if loaded.SkippedCount != 1 {
		t.Errorf("expected 1 skipped, got %d", loaded.SkippedCount)
	}
}

func TestLoadNonExistent(t *testing.T) {
	m := newTestManager(t)
	loaded, err := m.Load()
	if err != nil {
		t.Fatalf("Load should not error for missing file: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil checkpoint for missing file")
	}
}

func TestExists(t *testing.T) {
	m := newTestManager(t)

	if m.Exists() {
		t.Error("should not exist before save")
	}

	if err := m.Save(Checkpoint{Operation: "test"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if !m.Exists() {
		t.Error("should exist after save")
	}
}

func TestClear(t *testing.T) {
	m := newTestManager(t)

	if err := m.Save(Checkpoint{Operation: "test"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := m.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	if m.Exists() {
		t.Error("should not exist after clear")
	}
}

func TestClearNonExistent(t *testing.T) {
	m := newTestManager(t)
	if err := m.Clear(); err != nil {
		t.Errorf("Clear should not error on non-existent file: %v", err)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "checkpoint.json")
	m, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := m.Save(Checkpoint{Operation: "test"}); err != nil {
		t.Fatalf("Save should create dirs: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("checkpoint file should exist: %v", err)
	}
}
