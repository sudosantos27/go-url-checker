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

// rootCmd represents the base command when called without any subcommands.
// It defines the main entry point for the CLI application.
var rootCmd = &cobra.Command{
	Use:   "url-checker",
	Short: "A concurrent URL checker CLI",
	Long: `go-url-checker is a high-performance, concurrent CLI tool 
for checking the status of multiple URLs.

It supports:
- Concurrent execution with worker pools.
- Configurable timeouts, retries, and rate limiting.
- Structured JSON output.
- Configuration via flags, environment variables, or config files.`,

	// PersistentPreRun is executed before the Run command.
	// We use it here to initialize the logger based on the debug flag.
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

	// Run contains the main logic of the command.
	Run: func(cmd *cobra.Command, args []string) {
		// Retrieve configuration values from Viper.
		// Viper handles the precedence: Flag > Env Var > Config File > Default.
		file := viper.GetString("file")
		concurrency := viper.GetInt("concurrency")
		timeout := viper.GetDuration("timeout")
		output := viper.GetString("output")
		retries := viper.GetInt("retries")
		rateLimit := viper.GetInt("rate-limit")

		// Validation: Ensure the input file is specified.
		if file == "" {
			fmt.Fprintln(os.Stderr, "Error: file is required (via flag -f, config, or env URL_CHECKER_FILE).")
			_ = cmd.Usage()
			os.Exit(1)
		}

		// Validation: Ensure concurrency is at least 1.
		if concurrency < 1 {
			fmt.Fprintln(os.Stderr, "Warning: concurrency must be at least 1. Using 1.")
			concurrency = 1
		}

		// Read URLs from the specified file.
		urls, err := readURLs(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
			os.Exit(1)
		}

		if len(urls) == 0 {
			fmt.Println("The file is empty. Nothing to process.")
			return
		}

		// Create a context with a global timeout.
		// This context will be propagated to all workers and HTTP requests.
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// Initialize configuration struct for the checker.
		cfg := checker.Config{
			Concurrency: concurrency,
			Timeout:     timeout,
			Retries:     retries,
			RateLimit:   rateLimit,
		}

		// Execute the URL checker logic.
		checker.Check(ctx, urls, cfg, output)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// init initializes the flags and configuration settings.
func init() {
	cobra.OnInitialize(initConfig)

	// Define persistent flags (available to this command and all subcommands).
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.go-url-checker.yaml)")

	// Define local flags.
	rootCmd.Flags().StringP("file", "f", "", "Path to the file containing URLs (required)")
	rootCmd.Flags().IntP("concurrency", "c", 5, "Number of concurrent workers")
	rootCmd.Flags().DurationP("timeout", "t", 30*time.Second, "Global timeout for the process")
	rootCmd.Flags().StringP("output", "o", "text", "Output format (text, json)")
	rootCmd.Flags().Bool("debug", false, "Enable debug logging")
	rootCmd.Flags().Int("retries", 0, "Number of retries for failed requests")
	rootCmd.Flags().Int("rate-limit", 0, "Rate limit in requests per second (0 = unlimited)")

	// Bind flags to viper to enable environment variable and config file support.
	_ = viper.BindPFlag("file", rootCmd.Flags().Lookup("file"))
	_ = viper.BindPFlag("concurrency", rootCmd.Flags().Lookup("concurrency"))
	_ = viper.BindPFlag("timeout", rootCmd.Flags().Lookup("timeout"))
	_ = viper.BindPFlag("output", rootCmd.Flags().Lookup("output"))
	_ = viper.BindPFlag("debug", rootCmd.Flags().Lookup("debug"))
	_ = viper.BindPFlag("retries", rootCmd.Flags().Lookup("retries"))
	_ = viper.BindPFlag("rate-limit", rootCmd.Flags().Lookup("rate-limit"))
}

// initConfig reads in config file and ENV variables if set.
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

	// Read in environment variables that match "URL_CHECKER_..."
	viper.SetEnvPrefix("URL_CHECKER")
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		slog.Debug("Using config file", "file", viper.ConfigFileUsed())
	}
}
