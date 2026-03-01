// Package types defines unified message representations used across platforms.
package types

import "time"

// Message is the unified message representation used across Discord and Telegram.
type Message struct {
	ID            string       `json:"id"`
	Platform      string       `json:"platform"` // "discord" or "telegram"
	ChannelID     string       `json:"channel_id"`
	ChannelName   string       `json:"channel_name"`
	ServerID      string       `json:"server_id,omitempty"`   // Discord only
	ServerName    string       `json:"server_name,omitempty"` // Discord only
	ChatID        string       `json:"chat_id,omitempty"`     // Telegram only
	ChatName      string       `json:"chat_name,omitempty"`   // Telegram only
	Content       string       `json:"content"`
	Timestamp     time.Time    `json:"timestamp"`
	Type          string       `json:"type"` // "text", "image", "file", "embed", "link"
	HasAttachment bool         `json:"has_attachment"`
	HasLink       bool         `json:"has_link"`
	IsPinned      bool         `json:"is_pinned"`
	Attachments   []Attachment `json:"attachments,omitempty"`
}

// Attachment represents a file attached to a message.
type Attachment struct {
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

// Channel represents a Discord channel or Telegram chat for listing targets.
type Channel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	ServerID string `json:"server_id,omitempty"` // Discord only
	ChatID   string `json:"chat_id,omitempty"`   // Telegram only
}

// ScanResult holds the results of scanning a channel/chat.
type ScanResult struct {
	Channel      Channel   `json:"channel"`
	MessageCount int       `json:"message_count"`
	FirstDate    time.Time `json:"first_date"`
	LastDate     time.Time `json:"last_date"`
	TypesFound   []string  `json:"types_found"`
}
