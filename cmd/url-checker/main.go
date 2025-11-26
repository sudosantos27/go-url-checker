package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sudosantos27/go-url-checker/internal/checker"
)

func main() {
	// 1. Flag definition
	fileFlag := flag.String("file", "", "Path to the file containing URLs (required)")
	concurrencyFlag := flag.Int("concurrency", 5, "Number of concurrent workers")
	timeoutFlag := flag.Duration("timeout", 30*time.Second, "Global timeout for the process (e.g., 30s, 1m)")

	flag.Parse()

	// 2. Flag validation
	if *fileFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: -file flag is required.")
		flag.Usage()
		os.Exit(1)
	}

	if *concurrencyFlag < 1 {
		fmt.Fprintln(os.Stderr, "Warning: -concurrency must be at least 1. Using 1.")
		*concurrencyFlag = 1
	}

	// 3. Read URLs from file
	urls, err := readURLs(*fileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	if len(urls) == 0 {
		fmt.Println("The file is empty. Nothing to process.")
		return
	}

	// 4. Configure context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	// 5. Call internal package
	checker.Check(ctx, urls, *concurrencyFlag)
}

// readURLs reads the file line by line and returns a slice of strings.
func readURLs(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			urls = append(urls, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return urls, nil
}
