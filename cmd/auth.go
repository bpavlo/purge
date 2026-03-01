package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bpavlo/purge/internal/discord"
	"github.com/bpavlo/purge/internal/ratelimit"
	"github.com/bpavlo/purge/internal/telegram"
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
		return runAuthDiscord()
	},
}

var authTelegramCmd = &cobra.Command{
	Use:   "telegram",
	Short: "Authenticate with Telegram",
	Long: `Authenticate with Telegram using your API credentials.

You will need your api_id and api_hash from https://my.telegram.org.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAuthTelegram()
	},
}

func runAuthDiscord() error {
	fmt.Println("Discord Authentication")
	fmt.Println("======================")
	fmt.Println()
	fmt.Println("To get your Discord user token:")
	fmt.Println("  1. Open Discord in your browser (discord.com)")
	fmt.Println("  2. Press F12 to open Developer Tools")
	fmt.Println("  3. Go to the Network tab")
	fmt.Println("  4. Send a message or navigate to any channel")
	fmt.Println("  5. Click on any request to discord.com/api")
	fmt.Println("  6. Look for the 'Authorization' header in the request headers")
	fmt.Println("  7. Copy the token value")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your Discord token: ")
	token, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading token: %w", err)
	}
	token = strings.TrimSpace(token)

	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	// Validate the token by calling the Discord API.
	rl := ratelimit.New(discordRateLimitConfig())
	client := discord.NewClient(token, rl)

	ctx := context.Background()
	user, err := client.ValidateToken(ctx)
	if err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	// Store the token.
	if err := saveDiscordToken(token); err != nil {
		return fmt.Errorf("saving token: %w", err)
	}

	fmt.Println()
	fmt.Printf("Authenticated as %s#%s\n", user.Username, user.Discriminator)
	fmt.Println()
	fmt.Println("WARNING: Your Discord token grants full access to your account.")
	fmt.Println("Never share it with anyone. The token is stored locally at:")
	path, _ := discordTokenPath()
	fmt.Printf("  %s\n", path)

	return nil
}

func runAuthTelegram() error {
	// Read credentials from config or prompt.
	apiIDStr := viper.GetString("telegram.api_id")
	apiHash := viper.GetString("telegram.api_hash")
	phone := viper.GetString("telegram.phone")

	reader := bufio.NewReader(os.Stdin)

	if apiIDStr == "" {
		fmt.Print("Enter Telegram API ID: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading API ID: %w", err)
		}
		apiIDStr = strings.TrimSpace(input)
	}

	apiID, err := strconv.Atoi(apiIDStr)
	if err != nil {
		return fmt.Errorf("invalid API ID %q: must be a number", apiIDStr)
	}

	if apiHash == "" {
		fmt.Print("Enter Telegram API Hash: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading API Hash: %w", err)
		}
		apiHash = strings.TrimSpace(input)
	}

	if phone == "" {
		fmt.Print("Enter phone number (with country code, e.g. +1234567890): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading phone number: %w", err)
		}
		phone = strings.TrimSpace(input)
	}

	sessionPath, err := telegramSessionPath()
	if err != nil {
		return err
	}

	rl := ratelimit.New(telegramRateLimitConfig())
	client := telegram.NewClient(apiID, apiHash, sessionPath, rl)

	ctx := context.Background()
	err = client.Run(ctx, func(ctx context.Context) error {
		return client.Authenticate(ctx, phone, "")
	})
	if err != nil {
		return fmt.Errorf("Telegram authentication failed: %w", err)
	}

	// Persist API credentials to config file so subsequent commands work.
	if err := saveTelegramConfig(apiIDStr, apiHash); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save Telegram credentials to config: %v\n", err)
		fmt.Fprintf(os.Stderr, "You may need to set telegram.api_id and telegram.api_hash in your config file manually.\n")
	}

	fmt.Println()
	fmt.Println("Telegram session saved. You can now use 'purge scan telegram' and other commands.")

	return nil
}

func init() {
	authCmd.AddCommand(authDiscordCmd)
	authCmd.AddCommand(authTelegramCmd)
	rootCmd.AddCommand(authCmd)
}
