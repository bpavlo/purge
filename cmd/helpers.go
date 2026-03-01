package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/pavlo/purge/internal/discord"
	"github.com/pavlo/purge/internal/filter"
	"github.com/pavlo/purge/internal/types"
	"github.com/pavlo/purge/internal/ui"
)

const (
	discordTokenFile = "discord_token"
	telegramSession  = "telegram_session"
	configDirName    = ".config/purge"
	dateFormat       = "2006-01-02"
)

// configDir returns ~/.config/purge, creating it if needed.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	dir := filepath.Join(home, configDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating config directory: %w", err)
	}
	return dir, nil
}

// discordTokenPath returns the path to the stored Discord token file.
func discordTokenPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, discordTokenFile), nil
}

// loadDiscordToken reads the Discord token from disk.
func loadDiscordToken() (string, error) {
	path, err := discordTokenPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("Discord token not found. Run 'purge auth discord' first")
		}
		return "", fmt.Errorf("reading Discord token: %w", err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("Discord token file is empty. Run 'purge auth discord' first")
	}
	return token, nil
}

// saveDiscordToken writes the Discord token to disk with 0600 permissions.
func saveDiscordToken(token string) error {
	path, err := discordTokenPath()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(token), 0o600)
}

// telegramSessionPath returns the path for the Telegram session file.
func telegramSessionPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, telegramSession), nil
}

// toFilterOptions converts CLI FilterOptions (string dates) to filter.Options (time.Time).
func toFilterOptions(fo FilterOptions) (filter.Options, error) {
	var opts filter.Options

	if fo.After != "" {
		t, err := time.Parse(dateFormat, fo.After)
		if err != nil {
			return opts, fmt.Errorf("invalid --after date %q (expected YYYY-MM-DD): %w", fo.After, err)
		}
		opts.After = t
	}
	if fo.Before != "" {
		t, err := time.Parse(dateFormat, fo.Before)
		if err != nil {
			return opts, fmt.Errorf("invalid --before date %q (expected YYYY-MM-DD): %w", fo.Before, err)
		}
		// Make before inclusive of the entire day
		opts.Before = t.Add(24*time.Hour - time.Nanosecond)
	}

	opts.Keyword = fo.Keyword
	opts.HasAttachment = fo.HasAttachment
	opts.HasLink = fo.HasLink
	opts.MinLength = fo.MinLength
	opts.ExcludePinned = fo.ExcludePinned

	return opts, nil
}

// outputMode returns the ui.OutputMode based on viper config flags.
func outputMode() ui.OutputMode {
	if viper.GetBool("json") {
		return ui.ModeJSON
	}
	if viper.GetBool("quiet") {
		return ui.ModeQuiet
	}
	return ui.ModeNormal
}

// archiveDir returns the output directory for archives, choosing the flag value,
// viper config, or a default.
func archiveDir(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if dir := viper.GetString("archive_dir"); dir != "" {
		return dir
	}
	return "purge-archive"
}

// buildScanResult builds a types.ScanResult from a set of common messages for a given channel.
func buildScanResult(ch types.Channel, messages []*types.Message) types.ScanResult {
	result := types.ScanResult{
		Channel:      ch,
		MessageCount: len(messages),
	}

	typesSet := make(map[string]bool)
	for _, msg := range messages {
		typesSet[msg.Type] = true
		if result.FirstDate.IsZero() || msg.Timestamp.Before(result.FirstDate) {
			result.FirstDate = msg.Timestamp
		}
		if result.LastDate.IsZero() || msg.Timestamp.After(result.LastDate) {
			result.LastDate = msg.Timestamp
		}
	}

	for t := range typesSet {
		result.TypesFound = append(result.TypesFound, t)
	}

	return result
}

// filterDescriptionString returns a human-readable description of active filters.
func filterDescriptionString(fo FilterOptions) string {
	var parts []string
	if fo.After != "" {
		parts = append(parts, "after="+fo.After)
	}
	if fo.Before != "" {
		parts = append(parts, "before="+fo.Before)
	}
	if fo.Keyword != "" {
		parts = append(parts, "keyword="+fo.Keyword)
	}
	if fo.HasAttachment {
		parts = append(parts, "has-attachment")
	}
	if fo.HasLink {
		parts = append(parts, "has-link")
	}
	if fo.MinLength > 0 {
		parts = append(parts, fmt.Sprintf("min-length=%d", fo.MinLength))
	}
	if fo.ExcludePinned {
		parts = append(parts, "exclude-pinned")
	}
	return strings.Join(parts, ", ")
}

// buildChannelNameMap fetches all channels for a guild and returns a map of channel ID → name.
func buildChannelNameMap(ctx context.Context, client *discord.Client, guildID string) (map[string]string, error) {
	channels, err := client.GetChannels(ctx, guildID)
	if err != nil {
		return nil, err
	}
	nameMap := make(map[string]string, len(channels))
	for _, ch := range channels {
		nameMap[ch.ID] = ch.Name
	}
	return nameMap, nil
}

// groupMessagesByChannel groups messages by their ChannelID and returns per-channel ScanResult entries.
func groupMessagesByChannel(messages []*types.Message, guildID string) []types.ScanResult {
	// Group messages by channel ID.
	channelMsgs := make(map[string][]*types.Message)
	channelNames := make(map[string]string)
	for _, msg := range messages {
		channelMsgs[msg.ChannelID] = append(channelMsgs[msg.ChannelID], msg)
		if channelNames[msg.ChannelID] == "" {
			channelNames[msg.ChannelID] = msg.ChannelName
		}
	}

	// Build sorted results.
	var channelIDs []string
	for id := range channelMsgs {
		channelIDs = append(channelIDs, id)
	}
	sort.Strings(channelIDs)

	var results []types.ScanResult
	for _, chID := range channelIDs {
		msgs := channelMsgs[chID]
		chName := channelNames[chID]
		if chName == "" {
			chName = chID
		}
		ch := types.Channel{
			ID:       chID,
			Name:     chName,
			Platform: "discord",
			ServerID: guildID,
		}
		results = append(results, buildScanResult(ch, msgs))
	}

	return results
}
