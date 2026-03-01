package archive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bpavlo/purge/internal/types"
)

func TestArchiveCreatesDirectoryAndFile(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "subdir", "archives")

	a := NewArchiver(outputDir)
	msgs := []types.Message{
		{
			ID:        "1",
			Platform:  "discord",
			Content:   "hello world",
			Timestamp: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
			Type:      "text",
		},
	}
	meta := ArchiveMetadata{
		Platform:    "discord",
		ChannelID:   "123",
		ChannelName: "general",
		ServerName:  "My Server",
	}

	if err := a.Archive(msgs, meta); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	// Check directory was created
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("failed to read output dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	// Validate JSON structure
	data, err := os.ReadFile(filepath.Join(outputDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("failed to read archive file: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if output["platform"] != "discord" {
		t.Errorf("expected platform discord, got %v", output["platform"])
	}

	channel, ok := output["channel"].(map[string]interface{})
	if !ok {
		t.Fatal("missing channel field")
	}
	if channel["name"] != "general" {
		t.Errorf("expected channel name general, got %v", channel["name"])
	}

	messages, ok := output["messages"].([]interface{})
	if !ok {
		t.Fatal("missing messages field")
	}
	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}
}

func TestArchiveFileNaming(t *testing.T) {
	dir := t.TempDir()
	a := NewArchiver(dir)

	meta := ArchiveMetadata{
		Platform:    "telegram",
		ChannelID:   "456",
		ChannelName: "#my channel/test",
	}

	if err := a.Archive([]types.Message{}, meta); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	name := entries[0].Name()
	// Should not contain #, /, or spaces
	for _, bad := range []string{"#", "/", " "} {
		if contains(name, bad) {
			t.Errorf("filename %q should not contain %q", name, bad)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestArchiveInvalidDir(t *testing.T) {
	a := NewArchiver("/dev/null/impossible/path")
	err := a.Archive([]types.Message{}, ArchiveMetadata{ChannelName: "test"})
	if err == nil {
		t.Error("expected error for invalid directory")
	}
}
