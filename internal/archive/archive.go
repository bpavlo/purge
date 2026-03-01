// Package archive handles exporting messages to JSON files.
package archive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pavlo/purge/internal/types"
)

// ArchiveMetadata holds metadata about the export operation.
type ArchiveMetadata struct {
	Platform    string `json:"platform"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	ServerName  string `json:"server_name,omitempty"`
	ChatName    string `json:"chat_name,omitempty"`
}

// archiveOutput is the top-level JSON structure for archived messages.
type archiveOutput struct {
	Platform   string          `json:"platform"`
	ExportedAt time.Time       `json:"exported_at"`
	Channel    channelInfo     `json:"channel"`
	Messages   []types.Message `json:"messages"`
}

type channelInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Server string `json:"server,omitempty"`
	Chat   string `json:"chat,omitempty"`
}

// Archiver writes message archives to disk.
type Archiver struct {
	OutputDir string
}

// NewArchiver creates an Archiver that writes to the given directory.
func NewArchiver(outputDir string) *Archiver {
	return &Archiver{OutputDir: outputDir}
}

// Archive writes messages to a JSON file in the output directory.
// File naming: messages_{channelname}_{date}.json
func (a *Archiver) Archive(messages []types.Message, metadata ArchiveMetadata) error {
	if err := os.MkdirAll(a.OutputDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	output := archiveOutput{
		Platform:   metadata.Platform,
		ExportedAt: time.Now().UTC(),
		Channel: channelInfo{
			ID:     metadata.ChannelID,
			Name:   metadata.ChannelName,
			Server: metadata.ServerName,
			Chat:   metadata.ChatName,
		},
		Messages: messages,
	}

	safeName := sanitizeFilename(metadata.ChannelName)
	date := time.Now().UTC().Format("2006-01-02")
	filename := fmt.Sprintf("messages_%s_%s.json", safeName, date)
	path := filepath.Join(a.OutputDir, filename)

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling archive: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing archive file: %w", err)
	}

	return nil
}

// sanitizeFilename replaces characters that are unsafe in filenames.
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		" ", "_",
		"#", "",
		":", "_",
	)
	return replacer.Replace(name)
}
