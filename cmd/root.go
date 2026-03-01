package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Exit codes
const (
	ExitSuccess     = 0
	ExitError       = 1
	ExitAuthFailure = 2
	ExitPartial     = 3
	ExitInterrupted = 130
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "purge",
	Short: "Bulk-delete your own messages from Discord and Telegram",
	Long: `Purge is a CLI tool for bulk-deleting your own messages from
Discord servers/DMs and Telegram chats. It supports scanning,
filtering, archiving, and deleting messages across platforms.`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(ExitError)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.config/purge/purge.yaml)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "quiet mode (errors + summary only)")
	rootCmd.PersistentFlags().Bool("json", false, "machine-readable JSON output")
	rootCmd.PersistentFlags().String("log-level", "info", "log level: debug|info|warn|error")

	// Bind flags to viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
	viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))
	viper.BindPFlag("log_level", rootCmd.PersistentFlags().Lookup("log-level"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Warning: could not determine home directory:", err)
			return
		}

		viper.AddConfigPath(home + "/.config/purge")
		viper.SetConfigType("yaml")
		viper.SetConfigName("purge")
	}

	viper.SetEnvPrefix("PURGE")
	viper.AutomaticEnv()

	// Silently ignore missing config file — it's optional
	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}
