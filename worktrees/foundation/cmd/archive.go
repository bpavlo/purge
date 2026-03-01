package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
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
		_ = ParseFilterOptions(cmd)

		output, _ := cmd.Flags().GetString("output")
		_ = output

		switch platform {
		case "discord":
			return runDiscordArchive()
		case "telegram":
			return runTelegramArchive()
		default:
			return fmt.Errorf("unsupported platform: %s (use 'discord' or 'telegram')", platform)
		}
	},
}

func runDiscordArchive() error {
	fmt.Println("Discord archive: not implemented")
	return nil
}

func runTelegramArchive() error {
	fmt.Println("Telegram archive: not implemented")
	return nil
}

func init() {
	AddFilterFlags(archiveCmd)
	archiveCmd.Flags().StringP("output", "o", "", "output directory (default from config archive_dir)")
	rootCmd.AddCommand(archiveCmd)
}
