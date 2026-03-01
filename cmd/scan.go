package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pavlo/purge/internal/discord"
	"github.com/pavlo/purge/internal/filter"
	"github.com/pavlo/purge/internal/ratelimit"
	"github.com/pavlo/purge/internal/telegram"
	"github.com/pavlo/purge/internal/types"
	"github.com/pavlo/purge/internal/ui"
)

var scanCmd = &cobra.Command{
	Use:   "scan [discord|telegram]",
	Short: "Scan messages matching filters",
	Long: `Scan your messages on Discord or Telegram, applying optional filters.
Displays a summary of matching messages without modifying anything.`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"discord", "telegram"},
	RunE: func(cmd *cobra.Command, args []string) error {
		platform := args[0]
		fo := ParseFilterOptions(cmd)

		switch platform {
		case "discord":
			return runDiscordScan(fo)
		case "telegram":
			return runTelegramScan(fo)
		default:
			return fmt.Errorf("unsupported platform: %s (use 'discord' or 'telegram')", platform)
		}
	},
}

func runDiscordScan(fo FilterOptions) error {
	token, err := loadDiscordToken()
	if err != nil {
		return err
	}

	filterOpts, err := toFilterOptions(fo)
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

	// Determine which channels to scan.
	var channels []discord.Channel
	var guildID, guildName string

	switch {
	case fo.DMs:
		dmChannels, err := client.GetDMChannels(ctx)
		if err != nil {
			return fmt.Errorf("fetching DM channels: %w", err)
		}
		channels = dmChannels

	case fo.Server != "":
		guild, err := client.FindGuild(ctx, fo.Server)
		if err != nil {
			return err
		}
		guildID = guild.ID
		guildName = guild.Name

		if fo.Channel != "" {
			// Scan specific channel in server.
			allChannels, err := client.GetTextChannels(ctx, guild.ID)
			if err != nil {
				return err
			}
			for _, ch := range allChannels {
				if ch.ID == fo.Channel || ch.Name == fo.Channel {
					channels = append(channels, ch)
					break
				}
			}
			if len(channels) == 0 {
				return fmt.Errorf("channel %q not found in server %q", fo.Channel, fo.Server)
			}
		} else {
			// Scan all text channels in server.
			channels, err = client.GetTextChannels(ctx, guild.ID)
			if err != nil {
				return err
			}
		}

	case fo.Channel != "":
		// Channel without server — assume it's a channel ID.
		channels = append(channels, discord.Channel{ID: fo.Channel, Name: fo.Channel})

	default:
		return fmt.Errorf("specify --server, --channel, or --dms to target messages")
	}

	// Scan each channel.
	var results []types.ScanResult

	for _, ch := range channels {
		chName := ch.Name
		if chName == "" {
			chName = ch.DMName()
		}

		var allCommon []*types.Message

		if guildID != "" {
			// Use guild search for server channels.
			allCommon, err = searchDiscordChannel(ctx, client, user.ID, ch.ID, guildID, guildName, chName, filterOpts)
		} else {
			// Use channel messages for DMs or standalone channels.
			allCommon, err = fetchDiscordDMChannel(ctx, client, user.ID, ch, guildID, guildName, filterOpts)
		}
		if err != nil {
			if viper.GetBool("verbose") {
				fmt.Fprintf(os.Stderr, "Warning: error scanning channel %s: %v\n", chName, err)
			}
			continue
		}

		if len(allCommon) == 0 {
			continue
		}

		typeCh := types.Channel{
			ID:       ch.ID,
			Name:     chName,
			Platform: "discord",
			ServerID: guildID,
		}
		results = append(results, buildScanResult(typeCh, allCommon))
	}

	return printScanResults(results)
}

// searchDiscordChannel searches a guild channel using the search API and paginates all results.
func searchDiscordChannel(ctx context.Context, client *discord.Client, userID, channelID, guildID, guildName, channelName string, filterOpts filter.Options) ([]*types.Message, error) {
	var allCommon []*types.Message
	offset := 0

	for {
		resp, err := client.SearchChannelMessages(ctx, channelID, discord.SearchOptions{
			Offset: offset,
		})
		if err != nil {
			return allCommon, err
		}

		msgs := resp.ExtractMessages(userID)
		if len(msgs) == 0 {
			break
		}

		for i := range msgs {
			common := msgs[i].ToCommon(channelName, guildID, guildName)
			if filter.Match(common, filterOpts) {
				allCommon = append(allCommon, common)
			}
		}

		offset += len(msgs)
		if offset >= resp.TotalResults {
			break
		}
	}

	return allCommon, nil
}

// fetchDiscordDMChannel fetches messages from a DM channel using pagination.
func fetchDiscordDMChannel(ctx context.Context, client *discord.Client, userID string, ch discord.Channel, guildID, guildName string, filterOpts filter.Options) ([]*types.Message, error) {
	var allCommon []*types.Message
	before := ""
	chName := ch.Name
	if chName == "" {
		chName = ch.DMName()
	}

	for {
		msgs, err := client.GetChannelMessages(ctx, ch.ID, before, 100)
		if err != nil {
			return allCommon, err
		}
		if len(msgs) == 0 {
			break
		}

		for i := range msgs {
			msg := &msgs[i]
			if msg.Author.ID != userID {
				continue
			}
			common := msg.ToCommon(chName, guildID, guildName)
			if filter.Match(common, filterOpts) {
				allCommon = append(allCommon, common)
			}
		}

		before = msgs[len(msgs)-1].ID
	}

	return allCommon, nil
}

func runTelegramScan(fo FilterOptions) error {
	filterOpts, err := toFilterOptions(fo)
	if err != nil {
		return err
	}

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
	var results []types.ScanResult

	err = client.Run(ctx, func(ctx context.Context) error {
		authorized, err := client.IsAuthorized(ctx)
		if err != nil {
			return fmt.Errorf("checking auth: %w", err)
		}
		if !authorized {
			return fmt.Errorf("not authenticated. Run 'purge auth telegram' first")
		}

		_, err = client.GetSelf(ctx)
		if err != nil {
			return fmt.Errorf("getting self: %w", err)
		}

		dialogs, err := client.GetDialogs(ctx)
		if err != nil {
			return fmt.Errorf("getting dialogs: %w", err)
		}

		// Filter chats based on flags.
		var targetChats []telegram.Chat
		for _, chat := range dialogs {
			switch {
			case fo.Chat != "":
				if chat.Title == fo.Chat || fmt.Sprintf("%d", chat.ID) == fo.Chat {
					targetChats = append(targetChats, chat)
				}
			case fo.DMs:
				if chat.Type == telegram.ChatTypePrivate {
					targetChats = append(targetChats, chat)
				}
			case fo.AllChats:
				targetChats = append(targetChats, chat)
			}
		}

		if len(targetChats) == 0 && !fo.AllChats {
			return fmt.Errorf("specify --chat, --dms, or --all-chats to target messages")
		}

		for _, chat := range targetChats {
			searchOpts := telegram.SearchOptions{
				FromSelf: true,
			}
			if !filterOpts.After.IsZero() {
				searchOpts.MinDate = filterOpts.After
			}
			if !filterOpts.Before.IsZero() {
				searchOpts.MaxDate = filterOpts.Before
			}
			if filterOpts.Keyword != "" {
				searchOpts.Query = filterOpts.Keyword
			}

			msgs, err := client.GetMessages(ctx, chat, searchOpts)
			if err != nil {
				if viper.GetBool("verbose") {
					fmt.Fprintf(os.Stderr, "Warning: error scanning chat %s: %v\n", chat.Title, err)
				}
				continue
			}

			var filtered []*types.Message
			for _, msg := range msgs {
				common := telegram.MessageToCommon(msg, chat)
				if filter.Match(common, filterOpts) {
					filtered = append(filtered, common)
				}
			}

			if len(filtered) == 0 {
				continue
			}

			ch := types.Channel{
				ID:       fmt.Sprintf("%d", chat.ID),
				Name:     chat.Title,
				Platform: "telegram",
				ChatID:   fmt.Sprintf("%d", chat.ID),
			}
			results = append(results, buildScanResult(ch, filtered))
		}

		return nil
	})
	if err != nil {
		return err
	}

	return printScanResults(results)
}

// loadTelegramConfig reads Telegram API ID and Hash from viper config.
func loadTelegramConfig() (int, string, error) {
	apiIDStr := viper.GetString("telegram.api_id")
	apiHash := viper.GetString("telegram.api_hash")

	if apiIDStr == "" || apiHash == "" {
		return 0, "", fmt.Errorf("Telegram API credentials not configured. Set telegram.api_id and telegram.api_hash in config or run 'purge auth telegram'")
	}

	apiID, err := strconv.Atoi(apiIDStr)
	if err != nil {
		return 0, "", fmt.Errorf("invalid telegram.api_id %q: must be a number", apiIDStr)
	}

	return apiID, apiHash, nil
}

// printScanResults outputs scan results based on output mode.
func printScanResults(results []types.ScanResult) error {
	mode := outputMode()

	if mode == ui.ModeJSON {
		jsonStr, err := ui.FormatSummaryJSON(results)
		if err != nil {
			return err
		}
		fmt.Println(jsonStr)
		return nil
	}

	fmt.Print(ui.FormatSummaryTable(results))
	return nil
}

func init() {
	AddFilterFlags(scanCmd)
	rootCmd.AddCommand(scanCmd)
}
