package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bpavlo/purge/internal/archive"
	"github.com/bpavlo/purge/internal/checkpoint"
	"github.com/bpavlo/purge/internal/discord"
	"github.com/bpavlo/purge/internal/filter"
	"github.com/bpavlo/purge/internal/ratelimit"
	"github.com/bpavlo/purge/internal/telegram"
	"github.com/bpavlo/purge/internal/types"
	"github.com/bpavlo/purge/internal/ui"
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

		// Use config defaults when CLI flags aren't explicitly set.
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if !cmd.Flags().Changed("dry-run") {
			dryRun = viper.GetBool("defaults.dry_run")
		}

		doArchive, _ := cmd.Flags().GetBool("archive")
		if !cmd.Flags().Changed("archive") {
			doArchive = viper.GetBool("defaults.archive_before_delete")
		}

		// Apply defaults.exclude_pinned from config if flag not explicitly set.
		if !cmd.Flags().Changed("exclude-pinned") {
			if viper.GetBool("defaults.exclude_pinned") {
				fo.ExcludePinned = true
			}
		}

		var err error
		switch platform {
		case "discord":
			err = runDiscordDelete(fo, yes, dryRun, doArchive)
		case "telegram":
			err = runTelegramDelete(fo, yes, dryRun, doArchive)
		default:
			return fmt.Errorf("unsupported platform: %s (use 'discord' or 'telegram')", platform)
		}

		if err != nil {
			// Check for auth errors and exit with appropriate code.
			var authErr *discord.ErrAuth
			if errors.As(err, &authErr) {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(ExitAuthFailure)
			}
			if strings.Contains(err.Error(), "not authenticated") || strings.Contains(err.Error(), "authentication failed") {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(ExitAuthFailure)
			}
		}

		return err
	},
}

// promptResumeCheckpoint checks for an existing checkpoint and prompts the user to resume.
// Returns the checkpoint to resume from (or nil for fresh start), and the checkpoint manager.
func promptResumeCheckpoint(platform, target string) (*checkpoint.Checkpoint, *checkpoint.Manager, error) {
	cpManager, err := checkpoint.NewManager("")
	if err != nil {
		return nil, nil, fmt.Errorf("creating checkpoint manager: %w", err)
	}

	cp, err := cpManager.Load()
	if err != nil {
		return nil, cpManager, fmt.Errorf("loading checkpoint: %w", err)
	}

	if cp == nil {
		return nil, cpManager, nil
	}

	// Check if checkpoint matches current operation.
	if cp.Platform == platform && (cp.ServerID == target || cp.ChatID == target) {
		fmt.Fprintf(os.Stderr, "Found checkpoint from %s: %d already deleted. Resume? [y/N] ",
			cp.StartedAt.Format("2006-01-02 15:04:05"), cp.DeletedCount)

		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))

		if answer == "y" || answer == "yes" {
			return cp, cpManager, nil
		}

		// User chose not to resume — clear checkpoint.
		_ = cpManager.Clear()
		return nil, cpManager, nil
	}

	// Checkpoint exists but doesn't match current target.
	fmt.Fprintf(os.Stderr, "Warning: existing checkpoint for %s/%s does not match current target %s/%s. Starting fresh.\n",
		cp.Platform, cp.ServerID+cp.ChatID, platform, target)
	_ = cpManager.Clear()
	return nil, cpManager, nil
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

	rl := ratelimit.New(discordRateLimitConfig())
	client := discord.NewClient(token, rl)

	ctx := context.Background()
	user, err := client.ValidateToken(ctx)
	if err != nil {
		// Wrap auth errors for proper exit code handling.
		var authErr *discord.ErrAuth
		if errors.As(err, &authErr) {
			return err
		}
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

	// Check for checkpoint resume before scanning.
	cpTarget := guildID
	if fo.DMs {
		cpTarget = "dms"
	}
	if fo.Channel != "" {
		cpTarget = fo.Channel
	}

	resumeCP, cpManager, err := promptResumeCheckpoint("discord", cpTarget)
	if err != nil {
		return err
	}

	// Collect all messages across channels.
	type discordMsg struct {
		channelID   string
		channelName string
		messageID   string
		common      *types.Message
	}

	var allMsgs []discordMsg

	if useGuildSearch {
		// Guild-wide search: single query paginated across all channels.
		guildMsgs, err := searchGuildAllMessages(ctx, client, user.ID, guildID, guildName, filterOpts)
		if err != nil {
			return err
		}
		for _, msg := range guildMsgs {
			allMsgs = append(allMsgs, discordMsg{
				channelID:   msg.ChannelID,
				channelName: msg.ChannelName,
				messageID:   msg.ID,
				common:      msg,
			})
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

			for _, msg := range common {
				allMsgs = append(allMsgs, discordMsg{
					channelID:   ch.ID,
					channelName: chName,
					messageID:   msg.ID,
					common:      msg,
				})
			}
		}
	}

	// If resuming, skip messages up to last_processed_id and initialize counters.
	deleted, failed, skipped := 0, 0, 0
	startIdx := 0

	if resumeCP != nil {
		deleted = resumeCP.DeletedCount
		failed = resumeCP.FailedCount
		skipped = resumeCP.SkippedCount

		for i, msg := range allMsgs {
			if msg.messageID == resumeCP.LastProcessedID {
				startIdx = i + 1
				break
			}
		}
		if startIdx > 0 {
			fmt.Fprintf(os.Stderr, "Resuming from message %s (%d already processed)\n",
				resumeCP.LastProcessedID, startIdx)
		}
	}

	totalCount := len(allMsgs)

	if totalCount == 0 {
		fmt.Println("No messages found matching the specified filters.")
		return nil
	}

	remaining := totalCount - startIdx
	if remaining <= 0 {
		fmt.Println("All messages already processed.")
		_ = cpManager.Clear()
		return nil
	}

	// Dry run — just print what would be deleted.
	if dryRun {
		fmt.Printf("Dry run: would delete %d messages\n", remaining)
		return nil
	}

	// Archive before deleting if requested.
	if doArchive {
		dir := archiveDir("")
		archiver := archive.NewArchiver(dir)

		// Group messages by channel for archiving.
		channelMsgs := make(map[string][]types.Message)
		channelNames := make(map[string]string)
		for _, msg := range allMsgs[startIdx:] {
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
		fmt.Printf("Archived %d messages before deletion.\n", remaining)
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

		err := ui.ConfirmDeletion(remaining, "discord", target, filterDescriptionString(fo), doArchive, os.Stdin, os.Stdout)
		if err != nil {
			return err
		}
	}

	// Set up signal handler.
	stopSignal := cpManager.RegisterSignalHandler()
	defer stopSignal()

	mode := outputMode()
	bar := ui.NewProgressBar(remaining, "Deleting messages", mode)
	verbose := viper.GetBool("verbose")
	startedAt := time.Now()

	// Set up rate limit event listener for progress bar updates.
	rateLimitCh := make(chan discord.RateLimitEvent, 1)
	client.SetRateLimitChannel(rateLimitCh)

	for _, msg := range allMsgs[startIdx:] {
		// Check for any pending rate limit notification.
		select {
		case evt := <-rateLimitCh:
			bar.SetStatus(fmt.Sprintf("Rate limited — pausing %.1fs...", evt.RetryAfter.Seconds()))
		default:
		}

		alreadyDeleted, err := client.DeleteMessage(ctx, msg.channelID, msg.messageID)
		if err != nil {
			// Check for permission errors — skip with warning.
			var forbiddenErr *discord.ErrForbidden
			if errors.As(err, &forbiddenErr) {
				skipped++
				if verbose {
					fmt.Fprintf(os.Stderr, "Skipped message %s in #%s: insufficient permissions\n", msg.messageID, msg.channelName)
				}
			} else {
				failed++
				if verbose {
					fmt.Fprintf(os.Stderr, "Failed message %s: %v\n", msg.messageID, err)
				}
			}
		} else if alreadyDeleted {
			skipped++
			if verbose {
				fmt.Fprintf(os.Stderr, "Skipped message %s (already deleted)\n", msg.messageID)
			}
		} else {
			deleted++
			if verbose {
				fmt.Fprintf(os.Stderr, "Deleted message %s in #%s\n", msg.messageID, msg.channelName)
			}
		}

		// Restore normal description after successful request.
		bar.SetStatus("Deleting messages")
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
			StartedAt:       startedAt,
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

	// Exit with partial failure code if some messages failed.
	if deleted > 0 && failed > 0 {
		os.Exit(ExitPartial)
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

	rl := ratelimit.New(telegramRateLimitConfig())
	tgClient := telegram.NewClient(apiID, apiHash, sessionPath, rl)

	ctx := context.Background()

	return tgClient.Run(ctx, func(ctx context.Context) error {
		authorized, err := tgClient.IsAuthorized(ctx)
		if err != nil {
			return fmt.Errorf("checking auth: %w", err)
		}
		if !authorized {
			fmt.Fprintf(os.Stderr, "Error: not authenticated. Run 'purge auth telegram' first\n")
			os.Exit(ExitAuthFailure)
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

		// Determine checkpoint target.
		cpTarget := fo.Chat
		if fo.DMs {
			cpTarget = "dms"
		}
		if fo.AllChats {
			cpTarget = "all-chats"
		}

		resumeCP, cpManager, err := promptResumeCheckpoint("telegram", cpTarget)
		if err != nil {
			return err
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

		// Set up checkpoint manager and signal handler.
		stopSignal := cpManager.RegisterSignalHandler()
		defer stopSignal()

		mode := outputMode()
		bar := ui.NewProgressBar(totalCount, "Deleting messages", mode)
		verbose := viper.GetBool("verbose")
		startedAt := time.Now()

		deleted, failed, skipped := 0, 0, 0

		// If resuming, initialize counters from checkpoint.
		if resumeCP != nil {
			deleted = resumeCP.DeletedCount
			failed = resumeCP.FailedCount
			skipped = resumeCP.SkippedCount
		}

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
				if verbose {
					fmt.Fprintf(os.Stderr, "Failed to delete in chat %s: %v\n", cm.chat.Title, err)
				}
			} else {
				if verbose {
					fmt.Fprintf(os.Stderr, "Deleted %d messages in %s\n", len(cm.msgIDs), cm.chat.Title)
				}
			}

			// Save checkpoint after each chat's batch.
			_ = cpManager.Save(checkpoint.Checkpoint{
				Operation:       "delete",
				Platform:        "telegram",
				ChatID:          cpTarget,
				LastProcessedID: fmt.Sprintf("%d", cm.msgIDs[len(cm.msgIDs)-1]),
				DeletedCount:    deleted,
				FailedCount:     failed,
				SkippedCount:    skipped,
				StartedAt:       startedAt,
			})
		}

		bar.Finish()

		// Clear checkpoint on successful completion.
		_ = cpManager.Clear()

		if mode == ui.ModeJSON {
			jsonStr, err := ui.FormatDeleteJSON(deleted, failed, skipped)
			if err != nil {
				return err
			}
			fmt.Println(jsonStr)
		} else {
			fmt.Println(ui.DeleteSummary(deleted, failed, skipped))
		}

		// Exit with partial failure code if some messages failed.
		if deleted > 0 && failed > 0 {
			os.Exit(ExitPartial)
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
