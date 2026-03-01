// Package discord implements the Discord user API client for message scanning and deletion.
package discord

import (
	"strings"
	"time"

	"github.com/bpavlo/purge/internal/types"
)

// User represents a Discord user.
type User struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar,omitempty"`
	Bot           bool   `json:"bot,omitempty"`
}

// Guild represents a Discord server.
type Guild struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
}

// ChannelType represents the type of a Discord channel.
type ChannelType int

const (
	ChannelTypeGuildText     ChannelType = 0
	ChannelTypeDM            ChannelType = 1
	ChannelTypeGuildVoice    ChannelType = 2
	ChannelTypeGroupDM       ChannelType = 3
	ChannelTypeGuildCategory ChannelType = 4
	ChannelTypeGuildNews     ChannelType = 5
	ChannelTypeGuildForum    ChannelType = 15
)

// Channel represents a Discord channel.
type Channel struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Type       ChannelType `json:"type"`
	GuildID    string      `json:"guild_id,omitempty"`
	Recipients []User      `json:"recipients,omitempty"`
}

// DMName returns a display name for DM channels which lack a Name field.
func (c *Channel) DMName() string {
	if c.Name != "" {
		return c.Name
	}
	if len(c.Recipients) > 0 {
		names := make([]string, len(c.Recipients))
		for i, r := range c.Recipients {
			names[i] = r.Username
		}
		return strings.Join(names, ", ")
	}
	return "DM-" + c.ID
}

// MessageType represents the type of a Discord message.
type MessageType int

const (
	MessageTypeDefault           MessageType = 0
	MessageTypeRecipientAdd      MessageType = 1
	MessageTypeRecipientRemove   MessageType = 2
	MessageTypeCall              MessageType = 3
	MessageTypeChannelNameChange MessageType = 4
	MessageTypeChannelIconChange MessageType = 5
	MessageTypePinnedMessage     MessageType = 6
	MessageTypeGuildMemberJoin   MessageType = 7
	MessageTypeReply             MessageType = 19
	MessageTypeChatInputCommand  MessageType = 20
)

// Attachment represents a file attached to a Discord message.
type Attachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type,omitempty"`
}

// Embed represents a Discord message embed.
type Embed struct {
	Type        string `json:"type,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
}

// Message represents a Discord message.
type Message struct {
	ID          string       `json:"id"`
	Content     string       `json:"content"`
	Timestamp   time.Time    `json:"timestamp"`
	Author      User         `json:"author"`
	ChannelID   string       `json:"channel_id"`
	GuildID     string       `json:"guild_id,omitempty"`
	Attachments []Attachment `json:"attachments"`
	Embeds      []Embed      `json:"embeds"`
	Pinned      bool         `json:"pinned"`
	Type        MessageType  `json:"type"`
}

// hasLink checks whether the message content contains a URL.
func (m *Message) hasLink() bool {
	return strings.Contains(m.Content, "http://") || strings.Contains(m.Content, "https://")
}

// messageType returns a string describing the message type for the common format.
func (m *Message) messageType() string {
	if len(m.Attachments) > 0 {
		return "file"
	}
	if len(m.Embeds) > 0 {
		return "embed"
	}
	if m.hasLink() {
		return "link"
	}
	return "text"
}

// ToCommon converts a Discord Message to the shared types.Message format.
func (m *Message) ToCommon(channelName, serverID, serverName string) *types.Message {
	attachments := make([]types.Attachment, len(m.Attachments))
	for i, a := range m.Attachments {
		attachments[i] = types.Attachment{
			Filename:    a.Filename,
			URL:         a.URL,
			Size:        a.Size,
			ContentType: a.ContentType,
		}
	}

	return &types.Message{
		ID:            m.ID,
		Platform:      "discord",
		ChannelID:     m.ChannelID,
		ChannelName:   channelName,
		ServerID:      serverID,
		ServerName:    serverName,
		Content:       m.Content,
		Timestamp:     m.Timestamp,
		Type:          m.messageType(),
		HasAttachment: len(m.Attachments) > 0,
		HasLink:       m.hasLink(),
		IsPinned:      m.Pinned,
		Attachments:   attachments,
	}
}

// SearchOptions configures message search queries.
type SearchOptions struct {
	AuthorID string
	Content  string
	MinID    string // for date-based filtering (after)
	MaxID    string // for date-based filtering (before)
	Offset   int
}

// SearchResponse represents the response from a Discord message search.
type SearchResponse struct {
	Messages     [][]Message `json:"messages"` // messages grouped with context
	TotalResults int         `json:"total_results"`
}

// ExtractMessages returns the primary message from each search result group.
// Discord search results return arrays of [context, target, context] — the
// target message is determined by matching the author ID.
func (sr *SearchResponse) ExtractMessages(authorID string) []Message {
	var result []Message
	for _, group := range sr.Messages {
		for _, msg := range group {
			if msg.Author.ID == authorID {
				result = append(result, msg)
				break
			}
		}
	}
	return result
}
