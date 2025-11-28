package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/sudosantos27/go-url-checker/internal/checker"
)

var (
	fileFlag        string
	concurrencyFlag int
	timeoutFlag     time.Duration
	outputFlag      string
	debugFlag       bool
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
		if debugFlag {
			opts.Level = slog.LevelDebug
		}
		// Use TextHandler writing to Stderr to avoid polluting stdout (JSON output)
		logger := slog.New(slog.NewTextHandler(os.Stderr, opts))
		slog.SetDefault(logger)
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Validation
		if fileFlag == "" {
			fmt.Fprintln(os.Stderr, "Error: -file flag is required.")
			cmd.Usage()
			os.Exit(1)
		}

		if concurrencyFlag < 1 {
			fmt.Fprintln(os.Stderr, "Warning: -concurrency must be at least 1. Using 1.")
			concurrencyFlag = 1
		}

		// Read URLs
		urls, err := readURLs(fileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
			os.Exit(1)
		}

		if len(urls) == 0 {
			fmt.Println("The file is empty. Nothing to process.")
			return
		}

		// Context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), timeoutFlag)
		defer cancel()

		// Run checker
		checker.Check(ctx, urls, concurrencyFlag, outputFlag)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&fileFlag, "file", "f", "", "Path to the file containing URLs (required)")
	rootCmd.Flags().IntVarP(&concurrencyFlag, "concurrency", "c", 5, "Number of concurrent workers")
	rootCmd.Flags().DurationVarP(&timeoutFlag, "timeout", "t", 30*time.Second, "Global timeout for the process")
	rootCmd.Flags().StringVarP(&outputFlag, "output", "o", "text", "Output format (text, json)")
	rootCmd.Flags().BoolVar(&debugFlag, "debug", false, "Enable debug logging")

	// Mark file as required? Cobra has MarkFlagRequired but we are doing manual check for now to match previous behavior
	// rootCmd.MarkFlagRequired("file")
}
