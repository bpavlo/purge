package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with a messaging platform",
	Long:  `Authenticate with Discord or Telegram to allow purge to access your messages.`,
}

var authDiscordCmd = &cobra.Command{
	Use:   "discord",
	Short: "Authenticate with Discord",
	Long: `Authenticate with Discord by providing your user token.

Your token is stored locally and never sent to any third party.
To find your Discord token, open Discord in a browser, press F12,
go to the Network tab, and look for the Authorization header.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Discord authentication: not implemented")
		fmt.Println("This will prompt you for your Discord user token.")
		return nil
	},
}

var authTelegramCmd = &cobra.Command{
	Use:   "telegram",
	Short: "Authenticate with Telegram",
	Long: `Authenticate with Telegram using your API credentials.

You will need your api_id and api_hash from https://my.telegram.org.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Telegram authentication: not implemented")
		fmt.Println("This will prompt you for your Telegram API credentials.")
		return nil
	},
}

func init() {
	authCmd.AddCommand(authDiscordCmd)
	authCmd.AddCommand(authTelegramCmd)
	rootCmd.AddCommand(authCmd)
}
