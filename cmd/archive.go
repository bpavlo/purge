package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bpavlo/purge/internal/archive"
	"github.com/bpavlo/purge/internal/discord"
	"github.com/bpavlo/purge/internal/filter"
	"github.com/bpavlo/purge/internal/ratelimit"
	"github.com/bpavlo/purge/internal/telegram"
	"github.com/bpavlo/purge/internal/types"
)

var archiveCmd = &cobra.Command{
	Use:   "archive [discord|telegram]",
	Short: "Archive messages matching filters",
	Long: `Archive your messages on Discord or Telegram to a local directory.
Messages are exported in JSON format with optional attachments.`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"discord", "telegram"},
	RunE: func(cmd *cobra.Command, args []string) error {
		platform := args[0]
		fo := ParseFilterOptions(cmd)

		output, _ := cmd.Flags().GetString("output")

		switch platform {
		case "discord":
			return runDiscordArchive(fo, output)
		case "telegram":
			return runTelegramArchive(fo, output)
		default:
			return fmt.Errorf("unsupported platform: %s (use 'discord' or 'telegram')", platform)
		}
	},
}

func runDiscordArchive(fo FilterOptions, output string) error {
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

	// Determine channels (same logic as scan).
	var channels []discord.Channel
	var guildID, guildName string
	useGuildSearch := false

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
			// Use guild-wide search instead of iterating channels.
			useGuildSearch = true
		}

	case fo.Channel != "":
		channels = append(channels, discord.Channel{ID: fo.Channel, Name: fo.Channel})

	default:
		return fmt.Errorf("specify --server, --channel, or --dms to target messages")
	}

	dir := archiveDir(output)
	archiver := archive.NewArchiver(dir)
	totalArchived := 0

	if useGuildSearch {
		// Guild-wide search: single query paginated across all channels.
		allMsgs, err := searchGuildAllMessages(ctx, client, user.ID, guildID, guildName, filterOpts)
		if err != nil {
			return err
		}

		// Group by channel for per-channel archive files.
		channelMsgs := make(map[string][]types.Message)
		channelNames := make(map[string]string)
		for _, msg := range allMsgs {
			channelMsgs[msg.ChannelID] = append(channelMsgs[msg.ChannelID], *msg)
			if channelNames[msg.ChannelID] == "" {
				channelNames[msg.ChannelID] = msg.ChannelName
			}
		}

		for chID, msgs := range channelMsgs {
			chName := channelNames[chID]
			if chName == "" {
				chName = chID
			}
			err := archiver.Archive(msgs, archive.ArchiveMetadata{
				Platform:    "discord",
				ChannelID:   chID,
				ChannelName: chName,
				ServerName:  guildName,
			})
			if err != nil {
				return fmt.Errorf("archiving messages for channel %s: %w", chName, err)
			}
			totalArchived += len(msgs)
		}
	} else {
		for _, ch := range channels {
			chName := ch.Name
			if chName == "" {
				chName = ch.DMName()
			}

			var common []*types.Message
			if guildID != "" {
				common, err = searchDiscordChannel(ctx, client, user.ID, ch.ID, guildID, guildName, chName, filterOpts)
			} else {
				common, err = fetchDiscordDMChannel(ctx, client, user.ID, ch, guildID, guildName, filterOpts)
			}
			if err != nil {
				if viper.GetBool("verbose") {
					fmt.Fprintf(os.Stderr, "Warning: error scanning channel %s: %v\n", chName, err)
				}
				continue
			}

			if len(common) == 0 {
				continue
			}

			msgs := make([]types.Message, len(common))
			for i, m := range common {
				msgs[i] = *m
			}

			err = archiver.Archive(msgs, archive.ArchiveMetadata{
				Platform:    "discord",
				ChannelID:   ch.ID,
				ChannelName: chName,
				ServerName:  guildName,
			})
			if err != nil {
				return fmt.Errorf("archiving messages for channel %s: %w", chName, err)
			}
			totalArchived += len(msgs)
		}
	}

	if totalArchived == 0 {
		fmt.Println("No messages found matching the specified filters.")
	} else {
		fmt.Printf("Archived %d messages to %s\n", totalArchived, dir)
	}

	return nil
}

func runTelegramArchive(fo FilterOptions, output string) error {
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
	tgClient := telegram.NewClient(apiID, apiHash, sessionPath, rl)

	ctx := context.Background()

	return tgClient.Run(ctx, func(ctx context.Context) error {
		authorized, err := tgClient.IsAuthorized(ctx)
		if err != nil {
			return fmt.Errorf("checking auth: %w", err)
		}
		if !authorized {
			return fmt.Errorf("not authenticated. Run 'purge auth telegram' first")
		}

		_, err = tgClient.GetSelf(ctx)
		if err != nil {
			return fmt.Errorf("getting self: %w", err)
		}

		dialogs, err := tgClient.GetDialogs(ctx)
		if err != nil {
			return fmt.Errorf("getting dialogs: %w", err)
		}

		// Filter chats.
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

		dir := archiveDir(output)
		archiver := archive.NewArchiver(dir)
		totalArchived := 0

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

			msgs, err := tgClient.GetMessages(ctx, chat, searchOpts)
			if err != nil {
				if viper.GetBool("verbose") {
					fmt.Fprintf(os.Stderr, "Warning: error scanning chat %s: %v\n", chat.Title, err)
				}
				continue
			}

			var filtered []types.Message
			for _, msg := range msgs {
				common := telegram.MessageToCommon(msg, chat)
				if filter.Match(common, filterOpts) {
					filtered = append(filtered, *common)
				}
			}

			if len(filtered) == 0 {
				continue
			}

			err = archiver.Archive(filtered, archive.ArchiveMetadata{
				Platform:    "telegram",
				ChannelID:   fmt.Sprintf("%d", chat.ID),
				ChannelName: chat.Title,
				ChatName:    chat.Title,
			})
			if err != nil {
				return fmt.Errorf("archiving messages for chat %s: %w", chat.Title, err)
			}
			totalArchived += len(filtered)
		}

		if totalArchived == 0 {
			fmt.Println("No messages found matching the specified filters.")
		} else {
			fmt.Printf("Archived %d messages to %s\n", totalArchived, dir)
		}

		return nil
	})
}

func init() {
	AddFilterFlags(archiveCmd)
	archiveCmd.Flags().StringP("output", "o", "", "output directory (default from config archive_dir)")
	rootCmd.AddCommand(archiveCmd)
}
