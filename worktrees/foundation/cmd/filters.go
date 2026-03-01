package cmd

import "github.com/spf13/cobra"

// FilterOptions holds the shared filter flags used by scan, delete, and archive commands.
type FilterOptions struct {
	Server        string
	Channel       string
	Chat          string
	DMs           bool
	AllChats      bool
	After         string
	Before        string
	Keyword       string
	HasAttachment bool
	HasLink       bool
	MinLength     int
	ExcludePinned bool
}

// AddFilterFlags adds the shared filter flags to the given command.
func AddFilterFlags(cmd *cobra.Command) {
	cmd.Flags().String("server", "", "Discord server name or ID")
	cmd.Flags().String("channel", "", "Discord channel name or ID")
	cmd.Flags().String("chat", "", "Telegram chat name or ID")
	cmd.Flags().Bool("dms", false, "target all DMs / private chats")
	cmd.Flags().Bool("all-chats", false, "target all chats")
	cmd.Flags().String("after", "", "only messages after this date (inclusive, YYYY-MM-DD)")
	cmd.Flags().String("before", "", "only messages before this date (inclusive, YYYY-MM-DD)")
	cmd.Flags().String("keyword", "", "case-insensitive text match")
	cmd.Flags().Bool("has-attachment", false, "only messages with attachments")
	cmd.Flags().Bool("has-link", false, "only messages containing links")
	cmd.Flags().Int("min-length", 0, "minimum message length")
	cmd.Flags().Bool("exclude-pinned", false, "exclude pinned messages")
}

// ParseFilterOptions reads filter flag values from the given command.
func ParseFilterOptions(cmd *cobra.Command) FilterOptions {
	server, _ := cmd.Flags().GetString("server")
	channel, _ := cmd.Flags().GetString("channel")
	chat, _ := cmd.Flags().GetString("chat")
	dms, _ := cmd.Flags().GetBool("dms")
	allChats, _ := cmd.Flags().GetBool("all-chats")
	after, _ := cmd.Flags().GetString("after")
	before, _ := cmd.Flags().GetString("before")
	keyword, _ := cmd.Flags().GetString("keyword")
	hasAttachment, _ := cmd.Flags().GetBool("has-attachment")
	hasLink, _ := cmd.Flags().GetBool("has-link")
	minLength, _ := cmd.Flags().GetInt("min-length")
	excludePinned, _ := cmd.Flags().GetBool("exclude-pinned")

	return FilterOptions{
		Server:        server,
		Channel:       channel,
		Chat:          chat,
		DMs:           dms,
		AllChats:      allChats,
		After:         after,
		Before:        before,
		Keyword:       keyword,
		HasAttachment: hasAttachment,
		HasLink:       hasLink,
		MinLength:     minLength,
		ExcludePinned: excludePinned,
	}
}
