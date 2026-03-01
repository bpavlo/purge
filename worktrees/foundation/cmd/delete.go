package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
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
		_ = ParseFilterOptions(cmd)

		yes, _ := cmd.Flags().GetBool("yes")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		archive, _ := cmd.Flags().GetBool("archive")

		_ = yes
		_ = dryRun
		_ = archive

		switch platform {
		case "discord":
			return runDiscordDelete()
		case "telegram":
			return runTelegramDelete()
		default:
			return fmt.Errorf("unsupported platform: %s (use 'discord' or 'telegram')", platform)
		}
	},
}

func runDiscordDelete() error {
	fmt.Println("Discord delete: not implemented")
	return nil
}

func runTelegramDelete() error {
	fmt.Println("Telegram delete: not implemented")
	return nil
}

func init() {
	AddFilterFlags(deleteCmd)
	deleteCmd.Flags().Bool("yes", false, "skip confirmation prompt")
	deleteCmd.Flags().Bool("dry-run", false, "preview only, don't delete")
	deleteCmd.Flags().Bool("archive", false, "archive messages before deleting")
	rootCmd.AddCommand(deleteCmd)
}
