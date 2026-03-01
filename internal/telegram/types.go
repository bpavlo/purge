package telegram

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gotd/td/tg"

	"github.com/bpavlo/purge/internal/types"
)

// Sentinel errors.
var (
	ErrUserNotFound  = errors.New("telegram: authenticated user not found")
	ErrNotAuthorized = errors.New("telegram: not authorized")
)

// ChatType represents the type of a Telegram chat.
type ChatType string

const (
	ChatTypePrivate    ChatType = "private"
	ChatTypeGroup      ChatType = "group"
	ChatTypeSupergroup ChatType = "supergroup"
	ChatTypeChannel    ChatType = "channel"
)

// Chat represents a resolved Telegram chat/dialog.
type Chat struct {
	ID         int64
	AccessHash int64
	Title      string
	Type       ChatType
}

// IsChannel returns true if the chat is a channel or supergroup (requires channels.* API).
func (c Chat) IsChannel() bool {
	return c.Type == ChatTypeSupergroup || c.Type == ChatTypeChannel
}

// InputChannel returns the tg.InputChannel for channel/supergroup API calls.
func (c Chat) InputChannel() *tg.InputChannel {
	return &tg.InputChannel{
		ChannelID:  c.ID,
		AccessHash: c.AccessHash,
	}
}

// InputPeer returns the appropriate InputPeer for this chat type.
func (c Chat) InputPeer() tg.InputPeerClass {
	switch c.Type {
	case ChatTypePrivate:
		return &tg.InputPeerUser{
			UserID:     c.ID,
			AccessHash: c.AccessHash,
		}
	case ChatTypeGroup:
		return &tg.InputPeerChat{
			ChatID: c.ID,
		}
	case ChatTypeSupergroup, ChatTypeChannel:
		return &tg.InputPeerChannel{
			ChannelID:  c.ID,
			AccessHash: c.AccessHash,
		}
	default:
		return &tg.InputPeerChat{ChatID: c.ID}
	}
}

// SearchOptions controls message search/fetch parameters.
type SearchOptions struct {
	Query    string
	MinDate  time.Time
	MaxDate  time.Time
	Limit    int
	OffsetID int
	FromSelf bool
}

// MessageToCommon converts a tg.Message and its Chat context to the shared types.Message.
func MessageToCommon(msg *tg.Message, chat Chat) *types.Message {
	msgType := detectMessageType(msg)
	attachments := extractAttachments(msg)
	content := msg.Message
	hasLink := strings.Contains(content, "http://") || strings.Contains(content, "https://")

	return &types.Message{
		ID:            fmt.Sprintf("%d", msg.ID),
		Platform:      "telegram",
		ChannelID:     fmt.Sprintf("%d", chat.ID),
		ChannelName:   chat.Title,
		ChatID:        fmt.Sprintf("%d", chat.ID),
		ChatName:      chat.Title,
		Content:       content,
		Timestamp:     time.Unix(int64(msg.Date), 0),
		Type:          msgType,
		HasAttachment: len(attachments) > 0,
		HasLink:       hasLink,
		IsPinned:      msg.Pinned,
		Attachments:   attachments,
	}
}

// detectMessageType determines the message type from its media content.
func detectMessageType(msg *tg.Message) string {
	if msg.Media == nil {
		return "text"
	}

	switch msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		return "image"
	case *tg.MessageMediaDocument:
		return "file"
	case *tg.MessageMediaWebPage:
		return "link"
	case *tg.MessageMediaGeo, *tg.MessageMediaGeoLive:
		return "embed"
	case *tg.MessageMediaContact:
		return "embed"
	case *tg.MessageMediaPoll:
		return "embed"
	case *tg.MessageMediaVenue:
		return "embed"
	default:
		return "file"
	}
}

// extractAttachments extracts attachment metadata from message media.
func extractAttachments(msg *tg.Message) []types.Attachment {
	if msg.Media == nil {
		return nil
	}

	switch m := msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		if m.Photo != nil {
			return []types.Attachment{{
				Filename:    "photo.jpg",
				ContentType: "image/jpeg",
			}}
		}
	case *tg.MessageMediaDocument:
		if doc, ok := m.Document.(*tg.Document); ok {
			att := types.Attachment{
				Size:        doc.Size,
				ContentType: doc.MimeType,
			}
			// Try to find filename from attributes.
			for _, attr := range doc.Attributes {
				if fa, ok := attr.(*tg.DocumentAttributeFilename); ok {
					att.Filename = fa.FileName
					break
				}
			}
			if att.Filename == "" {
				att.Filename = fmt.Sprintf("document_%d", doc.ID)
			}
			return []types.Attachment{att}
		}
	}

	return nil
}

// CanDeleteInChat reports whether a user can delete their own messages in a given chat type.
// In private chats and basic groups, users can always delete their own messages.
// In supergroups/channels, users can delete their own messages within ~48 hours,
// or without limit if they are admin.
func CanDeleteInChat(chatType ChatType) bool {
	// Users can always attempt deletion of their own messages in all chat types.
	// The API will return errors for messages that can't be deleted (e.g., too old in supergroups).
	return true
}

// ChatFromDialog extracts a Chat from dialog peer and the associated entities maps.
func ChatFromDialog(peer tg.PeerClass, users map[int64]*tg.User, chats map[int64]*tg.Chat, channels map[int64]*tg.Channel) (Chat, bool) {
	switch p := peer.(type) {
	case *tg.PeerUser:
		if u, ok := users[p.UserID]; ok {
			title := strings.TrimSpace(u.FirstName + " " + u.LastName)
			if title == "" {
				title = u.Username
			}
			return Chat{
				ID:         u.ID,
				AccessHash: u.AccessHash,
				Title:      title,
				Type:       ChatTypePrivate,
			}, true
		}
	case *tg.PeerChat:
		if c, ok := chats[p.ChatID]; ok {
			return Chat{
				ID:    c.ID,
				Title: c.Title,
				Type:  ChatTypeGroup,
			}, true
		}
	case *tg.PeerChannel:
		if ch, ok := channels[p.ChannelID]; ok {
			chatType := ChatTypeChannel
			if ch.Megagroup {
				chatType = ChatTypeSupergroup
			}
			return Chat{
				ID:         ch.ID,
				AccessHash: ch.AccessHash,
				Title:      ch.Title,
				Type:       chatType,
			}, true
		}
	}
	return Chat{}, false
}
