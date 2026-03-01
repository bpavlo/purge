package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pavlo/purge/internal/archive"
	"github.com/pavlo/purge/internal/checkpoint"
	"github.com/pavlo/purge/internal/discord"
	"github.com/pavlo/purge/internal/filter"
	"github.com/pavlo/purge/internal/ratelimit"
	"github.com/pavlo/purge/internal/telegram"
	"github.com/pavlo/purge/internal/types"
	"github.com/pavlo/purge/internal/ui"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [discord|telegram]",
	Short: "Delete messages matching filters",
	Long: `Delete your messages on Discord or Telegram, applying optional filters.
By default, a confirmation prompt is shown before deletion.`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"discord", "telegram"},
	RunE: func(cmd *cobra.Command, args []string) error {
		platform := args[0]
		fo := ParseFilterOptions(cmd)

		yes, _ := cmd.Flags().GetBool("yes")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		doArchive, _ := cmd.Flags().GetBool("archive")

		switch platform {
		case "discord":
			return runDiscordDelete(fo, yes, dryRun, doArchive)
		case "telegram":
			return runTelegramDelete(fo, yes, dryRun, doArchive)
		default:
			return fmt.Errorf("unsupported platform: %s (use 'discord' or 'telegram')", platform)
		}
	},
}

func runDiscordDelete(fo FilterOptions, yes, dryRun, doArchive bool) error {
	token, err := loadDiscordToken()
	if err != nil {
		return err
	}

	filterOpts, err := toFilterOptions(fo)
	if err != nil {
		return err
	}

	rl := ratelimit.New(ratelimit.DefaultConfig())
	client := discord.NewClient(token, rl)

	ctx := context.Background()
	user, err := client.ValidateToken(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Determine channels (same logic as scan).
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
			channels, err = client.GetTextChannels(ctx, guild.ID)
			if err != nil {
				return err
			}
		}

	case fo.Channel != "":
		channels = append(channels, discord.Channel{ID: fo.Channel, Name: fo.Channel})

	default:
		return fmt.Errorf("specify --server, --channel, or --dms to target messages")
	}

	// Collect all messages across channels.
	type discordMsg struct {
		channelID   string
		channelName string
		messageID   string
		common      *types.Message
	}

	var allMsgs []discordMsg

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

		for _, msg := range common {
			allMsgs = append(allMsgs, discordMsg{
				channelID:   ch.ID,
				channelName: chName,
				messageID:   msg.ID,
				common:      msg,
			})
		}
	}

	totalCount := len(allMsgs)

	if totalCount == 0 {
		fmt.Println("No messages found matching the specified filters.")
		return nil
	}

	// Dry run — just print what would be deleted.
	if dryRun {
		fmt.Printf("Dry run: would delete %d messages\n", totalCount)
		return nil
	}

	// Archive before deleting if requested.
	if doArchive {
		dir := archiveDir("")
		archiver := archive.NewArchiver(dir)

		// Group messages by channel for archiving.
		channelMsgs := make(map[string][]types.Message)
		channelNames := make(map[string]string)
		for _, msg := range allMsgs {
			channelMsgs[msg.channelID] = append(channelMsgs[msg.channelID], *msg.common)
			channelNames[msg.channelID] = msg.channelName
		}
		for chID, msgs := range channelMsgs {
			err := archiver.Archive(msgs, archive.ArchiveMetadata{
				Platform:    "discord",
				ChannelID:   chID,
				ChannelName: channelNames[chID],
				ServerName:  guildName,
			})
			if err != nil {
				return fmt.Errorf("archiving messages for channel %s: %w", channelNames[chID], err)
			}
		}
		fmt.Printf("Archived %d messages before deletion.\n", totalCount)
	}

	// Confirmation prompt.
	if !yes {
		target := guildName
		if fo.DMs {
			target = "DMs"
		}
		if fo.Channel != "" {
			target = fo.Channel
		}

		err := ui.ConfirmDeletion(totalCount, "discord", target, filterDescriptionString(fo), doArchive, os.Stdin, os.Stdout)
		if err != nil {
			return err
		}
	}

	// Set up checkpoint manager.
	cpManager, err := checkpoint.NewManager("")
	if err != nil {
		return fmt.Errorf("creating checkpoint manager: %w", err)
	}
	stopSignal := cpManager.RegisterSignalHandler()
	defer stopSignal()

	mode := outputMode()
	bar := ui.NewProgressBar(totalCount, "Deleting messages", mode)

	deleted, failed, skipped := 0, 0, 0

	for _, msg := range allMsgs {
		err := client.DeleteMessage(ctx, msg.channelID, msg.messageID)
		if err != nil {
			failed++
			if viper.GetBool("verbose") {
				fmt.Fprintf(os.Stderr, "Failed to delete message %s: %v\n", msg.messageID, err)
			}
		} else {
			deleted++
		}

		bar.Increment()

		// Save checkpoint periodically.
		_ = cpManager.Save(checkpoint.Checkpoint{
			Operation:       "delete",
			Platform:        "discord",
			ServerID:        guildID,
			LastProcessedID: msg.messageID,
			DeletedCount:    deleted,
			FailedCount:     failed,
			SkippedCount:    skipped,
			StartedAt:       time.Now(),
		})
	}

	bar.Finish()

	// Clear checkpoint on successful completion.
	_ = cpManager.Clear()

	// Print summary.
	if mode == ui.ModeJSON {
		jsonStr, err := ui.FormatDeleteJSON(deleted, failed, skipped)
		if err != nil {
			return err
		}
		fmt.Println(jsonStr)
	} else {
		fmt.Println(ui.DeleteSummary(deleted, failed, skipped))
	}

	return nil
}

func runTelegramDelete(fo FilterOptions, yes, dryRun, doArchive bool) error {
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

	rl := ratelimit.New(ratelimit.DefaultConfig())
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

		// Collect messages from all target chats.
		type chatMessages struct {
			chat     telegram.Chat
			messages []*types.Message
			msgIDs   []int
		}

		var allChatMsgs []chatMessages

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

			var filtered []*types.Message
			var filteredIDs []int
			for _, msg := range msgs {
				common := telegram.MessageToCommon(msg, chat)
				if filter.Match(common, filterOpts) {
					filtered = append(filtered, common)
					filteredIDs = append(filteredIDs, msg.ID)
				}
			}

			if len(filtered) > 0 {
				allChatMsgs = append(allChatMsgs, chatMessages{
					chat:     chat,
					messages: filtered,
					msgIDs:   filteredIDs,
				})
			}
		}

		totalCount := 0
		for _, cm := range allChatMsgs {
			totalCount += len(cm.messages)
		}

		if totalCount == 0 {
			fmt.Println("No messages found matching the specified filters.")
			return nil
		}

		if dryRun {
			fmt.Printf("Dry run: would delete %d messages\n", totalCount)
			return nil
		}

		// Archive if requested.
		if doArchive {
			dir := archiveDir("")
			archiver := archive.NewArchiver(dir)
			for _, cm := range allChatMsgs {
				commonMsgs := make([]types.Message, len(cm.messages))
				for i, m := range cm.messages {
					commonMsgs[i] = *m
				}
				err := archiver.Archive(commonMsgs, archive.ArchiveMetadata{
					Platform:    "telegram",
					ChannelID:   fmt.Sprintf("%d", cm.chat.ID),
					ChannelName: cm.chat.Title,
					ChatName:    cm.chat.Title,
				})
				if err != nil {
					return fmt.Errorf("archiving messages for chat %s: %w", cm.chat.Title, err)
				}
			}
			fmt.Printf("Archived %d messages before deletion.\n", totalCount)
		}

		// Confirmation.
		if !yes {
			target := fo.Chat
			if fo.DMs {
				target = "DMs"
			}
			if fo.AllChats {
				target = "all chats"
			}

			err := ui.ConfirmDeletion(totalCount, "telegram", target, filterDescriptionString(fo), doArchive, os.Stdin, os.Stdout)
			if err != nil {
				return err
			}
		}

		mode := outputMode()
		bar := ui.NewProgressBar(totalCount, "Deleting messages", mode)

		deleted, failed := 0, 0

		for _, cm := range allChatMsgs {
			revoke := cm.chat.Type == telegram.ChatTypePrivate

			err := tgClient.BatchDelete(ctx, cm.chat, cm.msgIDs, revoke, func(deletedSoFar int) {
				for i := deleted; i < deletedSoFar; i++ {
					bar.Increment()
				}
				deleted = deletedSoFar
			})
			if err != nil {
				failed += len(cm.msgIDs)
				if viper.GetBool("verbose") {
					fmt.Fprintf(os.Stderr, "Error deleting in chat %s: %v\n", cm.chat.Title, err)
				}
			} else {
				deleted += len(cm.msgIDs)
				for i := 0; i < len(cm.msgIDs); i++ {
					bar.Increment()
				}
			}
		}

		bar.Finish()

		if mode == ui.ModeJSON {
			jsonStr, err := ui.FormatDeleteJSON(deleted, failed, 0)
			if err != nil {
				return err
			}
			fmt.Println(jsonStr)
		} else {
			fmt.Println(ui.DeleteSummary(deleted, failed, 0))
		}

		return nil
	})
}

func init() {
	AddFilterFlags(deleteCmd)
	deleteCmd.Flags().Bool("yes", false, "skip confirmation prompt")
	deleteCmd.Flags().Bool("dry-run", false, "preview only, don't delete")
	deleteCmd.Flags().Bool("archive", false, "archive messages before deleting")
	rootCmd.AddCommand(deleteCmd)
}
