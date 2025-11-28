package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/sudosantos27/go-url-checker/internal/checker"
)

var (
	cfgFile string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "url-checker",
	Short: "A concurrent URL checker CLI",
	Long: `go-url-checker is a high-performance, concurrent CLI tool 
for checking the status of multiple URLs.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Configure logger
		opts := &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}
		if viper.GetBool("debug") {
			opts.Level = slog.LevelDebug
		}
		// Use TextHandler writing to Stderr to avoid polluting stdout (JSON output)
		logger := slog.New(slog.NewTextHandler(os.Stderr, opts))
		slog.SetDefault(logger)
	},
	Run: func(cmd *cobra.Command, args []string) {
		file := viper.GetString("file")
		concurrency := viper.GetInt("concurrency")
		timeout := viper.GetDuration("timeout")
		output := viper.GetString("output")
		retries := viper.GetInt("retries")
		rateLimit := viper.GetInt("rate-limit")

		// Validation
		if file == "" {
			fmt.Fprintln(os.Stderr, "Error: file is required (via flag -f, config, or env URL_CHECKER_FILE).")
			cmd.Usage()
			os.Exit(1)
		}

		if concurrency < 1 {
			fmt.Fprintln(os.Stderr, "Warning: concurrency must be at least 1. Using 1.")
			concurrency = 1
		}

		// Read URLs
		urls, err := readURLs(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
			os.Exit(1)
		}

		if len(urls) == 0 {
			fmt.Println("The file is empty. Nothing to process.")
			return
		}

		// Context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// Run checker
		cfg := checker.Config{
			Concurrency: concurrency,
			Timeout:     timeout,
			Retries:     retries,
			RateLimit:   rateLimit,
		}
		checker.Check(ctx, urls, cfg, output)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.go-url-checker.yaml)")

	rootCmd.Flags().StringP("file", "f", "", "Path to the file containing URLs (required)")
	rootCmd.Flags().IntP("concurrency", "c", 5, "Number of concurrent workers")
	rootCmd.Flags().DurationP("timeout", "t", 30*time.Second, "Global timeout for the process")
	rootCmd.Flags().StringP("output", "o", "text", "Output format (text, json)")
	rootCmd.Flags().Bool("debug", false, "Enable debug logging")
	rootCmd.Flags().Int("retries", 0, "Number of retries for failed requests")
	rootCmd.Flags().Int("rate-limit", 0, "Rate limit in requests per second (0 = unlimited)")

	// Bind flags to viper
	viper.BindPFlag("file", rootCmd.Flags().Lookup("file"))
	viper.BindPFlag("concurrency", rootCmd.Flags().Lookup("concurrency"))
	viper.BindPFlag("timeout", rootCmd.Flags().Lookup("timeout"))
	viper.BindPFlag("output", rootCmd.Flags().Lookup("output"))
	viper.BindPFlag("debug", rootCmd.Flags().Lookup("debug"))
	viper.BindPFlag("retries", rootCmd.Flags().Lookup("retries"))
	viper.BindPFlag("rate-limit", rootCmd.Flags().Lookup("rate-limit"))
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".go-url-checker" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".go-url-checker")
	}

	viper.SetEnvPrefix("URL_CHECKER")
	viper.AutomaticEnv() // read in environment variables that match

	if err := viper.ReadInConfig(); err == nil {
		slog.Debug("Using config file", "file", viper.ConfigFileUsed())
	}
}
