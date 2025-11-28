package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

// Result stores the result of checking a URL.
type Result struct {
	URL        string        `json:"url"`
	StatusCode int           `json:"status_code"`
	Duration   time.Duration `json:"duration_ns"` // Duration in nanoseconds for JSON
	Err        error         `json:"-"`           // Skip error interface, we'll handle it manually if needed or add a string field
	ErrorMsg   string        `json:"error,omitempty"`
}

// Check processes URLs using a worker pool and supports cancellation via context.
func Check(ctx context.Context, urls []string, concurrency int, outputFormat string) {
	// Shared HTTP client.
	// Note: The client timeout is for each individual request.
	// The context controls the global timeout.
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	jobs := make(chan string, len(urls))
	results := make(chan Result, len(urls))
	var wg sync.WaitGroup

	slog.Info("Starting URL checks", "total_urls", len(urls), "workers", concurrency)
	startTotal := time.Now()

	// 1. Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker(ctx, client, jobs, results, &wg)
	}

	// 2. Send jobs
	// Use a goroutine to send jobs to avoid blocking if the buffer fills up
	// (although here the buffer is len(urls), it's good practice).
	go func() {
		for _, u := range urls {
			select {
			case <-ctx.Done():
				// If context is canceled, stop sending jobs
				close(jobs)
				return
			case jobs <- u:
			}
		}
		close(jobs)
	}()

	// 3. Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// 4. Collect results and calculate statistics
	var resultsList []Result
	var okCount, failCount int

	for res := range results {
		if outputFormat == "text" {
			printResult(res)
		}

		if res.Err != nil || res.StatusCode < 200 || res.StatusCode >= 300 {
			failCount++
		} else {
			okCount++
		}
		resultsList = append(resultsList, res)
	}

	// Check if we finished due to timeout
	if ctx.Err() == context.DeadlineExceeded {
		slog.Error("Global timeout reached", "timeout", ctx.Err())
	}

	if outputFormat == "json" {
		printJSON(resultsList, okCount, failCount, time.Since(startTotal))
		return
	}

	totalDuration := time.Since(startTotal)
	slog.Info("Check completed",
		"total", len(urls),
		"ok", okCount,
		"fail", failCount,
		"duration", totalDuration,
	)
}

// worker is the function executed by each goroutine in the pool.
func worker(ctx context.Context, client *http.Client, jobs <-chan string, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			// Context canceled, exit worker
			return
		case url, ok := <-jobs:
			if !ok {
				// Channel closed, no more jobs
				return
			}
			results <- checkURL(ctx, client, url)
		}
	}
}

// checkURL performs the HTTP request and returns a Result.
func checkURL(ctx context.Context, client *http.Client, url string) Result {
	start := time.Now()

	// Create a request with the context so it cancels if the context dies
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return Result{URL: url, Duration: time.Since(start), Err: err, ErrorMsg: err.Error()}
	}

	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		return Result{
			URL:      url,
			Duration: duration,
			Err:      err,
			ErrorMsg: err.Error(),
		}
	}
	defer resp.Body.Close()

	return Result{
		URL:        url,
		StatusCode: resp.StatusCode,
		Duration:   duration,
		Err:        nil,
	}
}

// printJSON formats the output as JSON.
func printJSON(results []Result, ok, fail int, totalDuration time.Duration) {
	type Summary struct {
		Total         int     `json:"total"`
		OK            int     `json:"ok"`
		Fail          int     `json:"fail"`
		TotalDuration float64 `json:"total_duration_s"`
	}

	type Output struct {
		Results []Result `json:"results"`
		Summary Summary  `json:"summary"`
	}

	out := Output{
		Results: results,
		Summary: Summary{
			Total:         len(results),
			OK:            ok,
			Fail:          fail,
			TotalDuration: totalDuration.Seconds(),
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}
}

// printResult formats and prints the result to the console.
func printResult(r Result) {
	if r.Err != nil {
		slog.Error("Check failed", "url", r.URL, "error", r.Err, "duration", r.Duration)
	} else {
		slog.Info("Check success", "url", r.URL, "status", r.StatusCode, "duration", r.Duration)
	}
}
