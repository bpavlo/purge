package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
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
		_ = ParseFilterOptions(cmd)

		switch platform {
		case "discord":
			return runDiscordScan()
		case "telegram":
			return runTelegramScan()
		default:
			return fmt.Errorf("unsupported platform: %s (use 'discord' or 'telegram')", platform)
		}
	},
}

func runDiscordScan() error {
	fmt.Println("Discord scan: not implemented")
	return nil
}

func runTelegramScan() error {
	fmt.Println("Telegram scan: not implemented")
	return nil
}

func init() {
	AddFilterFlags(scanCmd)
	rootCmd.AddCommand(scanCmd)
}
