// Package checkpoint provides save/load/resume functionality for long-running operations.
package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Checkpoint tracks the state of a long-running delete operation for resume support.
type Checkpoint struct {
	Operation       string    `json:"operation"`
	Platform        string    `json:"platform"`
	ServerID        string    `json:"server_id,omitempty"`
	ChatID          string    `json:"chat_id,omitempty"`
	LastProcessedID string    `json:"last_processed_id"`
	DeletedCount    int       `json:"deleted_count"`
	FailedCount     int       `json:"failed_count"`
	SkippedCount    int       `json:"skipped_count"`
	StartedAt       time.Time `json:"started_at"`
}

// Manager handles checkpoint persistence.
type Manager struct {
	mu   sync.Mutex
	path string
	// current holds the latest checkpoint for signal-handler saves.
	current *Checkpoint
}

// NewManager creates a Manager that stores checkpoints at the given path.
// If path is empty, it defaults to ~/.config/purge/checkpoint.json.
func NewManager(path string) (*Manager, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting home directory: %w", err)
		}
		path = filepath.Join(home, ".config", "purge", "checkpoint.json")
	}
	return &Manager{path: path}, nil
}

// Save writes the checkpoint to disk.
func (m *Manager) Save(cp Checkpoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.current = &cp
	return m.saveLocked(cp)
}

func (m *Manager) saveLocked(cp Checkpoint) error {
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating checkpoint directory: %w", err)
	}

	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling checkpoint: %w", err)
	}

	if err := os.WriteFile(m.path, data, 0o644); err != nil {
		return fmt.Errorf("writing checkpoint: %w", err)
	}

	return nil
}

// Load reads an existing checkpoint from disk.
func (m *Manager) Load() (*Checkpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading checkpoint: %w", err)
	}

	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("parsing checkpoint: %w", err)
	}

	m.current = &cp
	return &cp, nil
}

// Clear removes the checkpoint file after successful completion.
func (m *Manager) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.current = nil
	err := os.Remove(m.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing checkpoint: %w", err)
	}
	return nil
}

// Exists checks if a checkpoint file exists on disk.
func (m *Manager) Exists() bool {
	_, err := os.Stat(m.path)
	return err == nil
}

// RegisterSignalHandler sets up a SIGINT handler that saves the current
// checkpoint before exiting. Call the returned function to stop listening.
func (m *Manager) RegisterSignalHandler() func() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)

	done := make(chan struct{})

	go func() {
		select {
		case <-sigCh:
			m.mu.Lock()
			if m.current != nil {
				_ = m.saveLocked(*m.current)
			}
			m.mu.Unlock()
			os.Exit(1)
		case <-done:
			return
		}
	}()

	return func() {
		signal.Stop(sigCh)
		close(done)
	}
}
