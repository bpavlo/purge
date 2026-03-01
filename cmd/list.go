package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bpavlo/purge/internal/discord"
	"github.com/bpavlo/purge/internal/ratelimit"
	"github.com/bpavlo/purge/internal/telegram"
	"github.com/bpavlo/purge/internal/ui"
)

// listItem is a unified representation of a chat/channel/server for display.
type listItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Platform string `json:"platform"`
	ServerID string `json:"server_id,omitempty"`
	Server   string `json:"server,omitempty"`
}

var listCmd = &cobra.Command{
	Use:   "list [discord|telegram]",
	Short: "List available chats, channels, and servers",
	Long: `List your Discord servers, channels, and DMs, or Telegram chats.
Use flags to filter the results.

Examples:
  purge list discord                    # list all servers
  purge list discord --server MyServer  # list channels in a server
  purge list discord --dms              # list DM conversations
  purge list telegram                   # list all chats
  purge list telegram --dms             # list only private chats
  purge list telegram --type group      # list only groups
  purge list telegram --type supergroup # list only supergroups`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"discord", "telegram"},
	RunE: func(cmd *cobra.Command, args []string) error {
		platform := args[0]

		switch platform {
		case "discord":
			return runDiscordList(cmd)
		case "telegram":
			return runTelegramList(cmd)
		default:
			return fmt.Errorf("unsupported platform: %s (use 'discord' or 'telegram')", platform)
		}
	},
}

func runDiscordList(cmd *cobra.Command) error {
	token, err := loadDiscordToken()
	if err != nil {
		return err
	}

	rl := ratelimit.New(discordRateLimitConfig())
	client := discord.NewClient(token, rl)

	ctx := context.Background()
	user, err := client.ValidateToken(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if viper.GetBool("verbose") {
		fmt.Fprintf(os.Stderr, "Authenticated as %s#%s\n", user.Username, user.Discriminator)
	}

	server, _ := cmd.Flags().GetString("server")
	dms, _ := cmd.Flags().GetBool("dms")
	filter, _ := cmd.Flags().GetString("filter")

	var items []listItem

	switch {
	case dms:
		// List DM channels.
		channels, err := client.GetDMChannels(ctx)
		if err != nil {
			return fmt.Errorf("fetching DM channels: %w", err)
		}
		for _, ch := range channels {
			name := ch.DMName()
			if filter != "" && !containsFold(name, filter) {
				continue
			}
			chType := "dm"
			if ch.Type == discord.ChannelTypeGroupDM {
				chType = "group_dm"
			}
			items = append(items, listItem{
				ID:       ch.ID,
				Name:     name,
				Type:     chType,
				Platform: "discord",
			})
		}

	case server != "":
		// List channels in a specific server.
		guild, err := client.FindGuild(ctx, server)
		if err != nil {
			return err
		}
		channels, err := client.GetChannels(ctx, guild.ID)
		if err != nil {
			return fmt.Errorf("fetching channels: %w", err)
		}
		for _, ch := range channels {
			if ch.Type == discord.ChannelTypeGuildCategory || ch.Type == discord.ChannelTypeGuildVoice {
				continue
			}
			if filter != "" && !containsFold(ch.Name, filter) {
				continue
			}
			items = append(items, listItem{
				ID:       ch.ID,
				Name:     ch.Name,
				Type:     discordChannelTypeName(ch.Type),
				Platform: "discord",
				ServerID: guild.ID,
				Server:   guild.Name,
			})
		}

	default:
		// List all servers.
		guilds, err := client.GetGuilds(ctx)
		if err != nil {
			return fmt.Errorf("fetching servers: %w", err)
		}
		for _, g := range guilds {
			if filter != "" && !containsFold(g.Name, filter) {
				continue
			}
			items = append(items, listItem{
				ID:       g.ID,
				Name:     g.Name,
				Type:     "server",
				Platform: "discord",
			})
		}
	}

	return printListItems(items)
}

func runTelegramList(cmd *cobra.Command) error {
	apiID, apiHash, err := loadTelegramConfig()
	if err != nil {
		return err
	}

	sessionPath, err := telegramSessionPath()
	if err != nil {
		return err
	}

	rl := ratelimit.New(telegramRateLimitConfig())
	client := telegram.NewClient(apiID, apiHash, sessionPath, rl)

	ctx := context.Background()
	dms, _ := cmd.Flags().GetBool("dms")
	chatType, _ := cmd.Flags().GetString("type")
	filter, _ := cmd.Flags().GetString("filter")

	var items []listItem

	err = client.Run(ctx, func(ctx context.Context) error {
		authorized, err := client.IsAuthorized(ctx)
		if err != nil {
			return fmt.Errorf("checking auth: %w", err)
		}
		if !authorized {
			return fmt.Errorf("not authenticated. Run 'purge auth telegram' first")
		}

		dialogs, err := client.GetDialogs(ctx)
		if err != nil {
			return fmt.Errorf("getting dialogs: %w", err)
		}

		for _, chat := range dialogs {
			// Apply --dms filter.
			if dms && chat.Type != telegram.ChatTypePrivate {
				continue
			}
			// Apply --type filter.
			if chatType != "" && string(chat.Type) != chatType {
				continue
			}
			// Apply --filter text search.
			if filter != "" && !containsFold(chat.Title, filter) {
				continue
			}

			items = append(items, listItem{
				ID:       fmt.Sprintf("%d", chat.ID),
				Name:     chat.Title,
				Type:     string(chat.Type),
				Platform: "telegram",
			})
		}

		return nil
	})
	if err != nil {
		return err
	}

	return printListItems(items)
}

// printListItems outputs list results in table or JSON format.
func printListItems(items []listItem) error {
	if len(items) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	mode := outputMode()

	if mode == ui.ModeJSON {
		data, err := json.Marshal(items)
		if err != nil {
			return fmt.Errorf("marshaling results: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Calculate column widths.
	maxName := 4 // "Name"
	maxType := 4 // "Type"
	maxID := 2   // "ID"
	for _, item := range items {
		if len(item.Name) > maxName {
			maxName = len(item.Name)
		}
		if len(item.Type) > maxType {
			maxType = len(item.Type)
		}
		if len(item.ID) > maxID {
			maxID = len(item.ID)
		}
	}
	// Cap name width.
	if maxName > 40 {
		maxName = 40
	}

	styles := ui.NewStyles()
	headerFmt := fmt.Sprintf(" %%-%ds  %%-%ds  %%-%ds", maxName, maxType, maxID)
	lineWidth := maxName + maxType + maxID + 7

	// Header.
	header := fmt.Sprintf(headerFmt, "Name", "Type", "ID")
	fmt.Println(styles.Bold.Render(header))
	fmt.Println(strings.Repeat("\u2500", lineWidth))

	// Rows.
	dimStyle := lipgloss.NewStyle()
	if ui.IsTTY() {
		dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	}

	for _, item := range items {
		name := item.Name
		if len(name) > 40 {
			name = name[:37] + "..."
		}
		namePart := fmt.Sprintf(" %-*s  %-*s  ", maxName, name, maxType, item.Type)
		idPart := fmt.Sprintf("%-*s", maxID, item.ID)
		fmt.Print(namePart)
		fmt.Println(dimStyle.Render(idPart))
	}

	fmt.Println(strings.Repeat("\u2500", lineWidth))
	fmt.Println(styles.Bold.Render(fmt.Sprintf(" %d items", len(items))))

	return nil
}

// containsFold reports whether s contains substr (case-insensitive).
func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// discordChannelTypeName returns a human-readable name for a Discord channel type.
func discordChannelTypeName(t discord.ChannelType) string {
	switch t {
	case discord.ChannelTypeGuildText:
		return "text"
	case discord.ChannelTypeDM:
		return "dm"
	case discord.ChannelTypeGuildVoice:
		return "voice"
	case discord.ChannelTypeGroupDM:
		return "group_dm"
	case discord.ChannelTypeGuildCategory:
		return "category"
	case discord.ChannelTypeGuildNews:
		return "news"
	case discord.ChannelTypeGuildForum:
		return "forum"
	default:
		return "other"
	}
}

func init() {
	listCmd.Flags().String("server", "", "Discord server name or ID (lists channels in server)")
	listCmd.Flags().Bool("dms", false, "list DM conversations")
	listCmd.Flags().String("type", "", "filter by chat type (private, group, supergroup, channel)")
	listCmd.Flags().String("filter", "", "filter by name (case-insensitive substring match)")

	rootCmd.AddCommand(listCmd)
}
