package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Result represents the outcome of a single URL check operation.
// It includes metadata such as the status code, duration, and any error encountered.
// This struct is tagged for JSON serialization to support structured output.
type Result struct {
	URL        string        `json:"url"`
	StatusCode int           `json:"status_code"`
	Duration   time.Duration `json:"duration_ns"` // Duration in nanoseconds for JSON
	Retries    int           `json:"retries"`     // Number of retries performed
	Err        error         `json:"-"`           // Skip error interface
	ErrorMsg   string        `json:"error,omitempty"`
}

// Config holds the configuration parameters for the URL checker.
// It controls concurrency levels, timeouts, retry policies, and rate limiting.
type Config struct {
	Concurrency int           // Number of concurrent workers
	Timeout     time.Duration // Global timeout context (not used directly in struct, but good for context)
	Retries     int           // Maximum number of retries for failed requests
	RateLimit   int           // Rate limit in requests per second (0 = unlimited)
}

// Check is the main entry point for the URL checking logic.
// It initializes the worker pool, manages the channels for jobs and results,
// and handles the aggregation of statistics.
//
// Parameters:
//   - ctx: Context for global cancellation and timeout.
//   - urls: Slice of URL strings to check.
//   - cfg: Configuration object containing concurrency, retry, and rate limit settings.
//   - outputFormat: Format for the final output ("text" or "json").
func Check(ctx context.Context, urls []string, cfg Config, outputFormat string) {
	// Initialize a shared HTTP client.
	// We use a default timeout of 10 seconds per individual request.
	// Note: The global timeout is managed by the passed 'ctx'.
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Initialize the Rate Limiter if a limit is configured.
	// We use a burst size of 1 to strictly enforce the rate.
	var limiter *rate.Limiter
	if cfg.RateLimit > 0 {
		limiter = rate.NewLimiter(rate.Limit(cfg.RateLimit), 1)
	}

	// Create buffered channels for jobs and results to prevent blocking.
	jobs := make(chan string, len(urls))
	results := make(chan Result, len(urls))
	var wg sync.WaitGroup

	slog.Info("Starting URL checks", "total_urls", len(urls), "workers", cfg.Concurrency, "retries", cfg.Retries, "rate_limit", cfg.RateLimit)
	startTotal := time.Now()

	// 1. Start the worker pool.
	// We spawn 'cfg.Concurrency' goroutines to process URLs in parallel.
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go worker(ctx, client, limiter, jobs, results, &wg, cfg.Retries)
	}

	// 2. Dispatch jobs to the workers.
	// We run this in a separate goroutine to ensure that job submission doesn't block
	// the main execution flow, although with a fully buffered channel this is less of a concern.
	go func() {
		for _, u := range urls {
			select {
			case <-ctx.Done():
				// If context is canceled (e.g. timeout), stop sending jobs immediately.
				close(jobs)
				return
			case jobs <- u:
			}
		}
		close(jobs)
	}()

	// 3. Monitor workers and close the results channel.
	// This goroutine waits for all workers to finish their tasks before closing the results channel,
	// signaling to the consumer that no more results will arrive.
	go func() {
		wg.Wait()
		close(results)
	}()

	// 4. Collect results and calculate statistics.
	// We iterate over the results channel as results come in.
	var resultsList []Result
	var okCount, failCount int

	for res := range results {
		// In text mode, we print results as they arrive for real-time feedback.
		if outputFormat == "text" {
			printResult(res)
		}

		// Determine success vs failure.
		// We consider a check successful if there is no error and the status code is 2xx.
		if res.Err != nil || res.StatusCode < 200 || res.StatusCode >= 300 {
			failCount++
		} else {
			okCount++
		}
		resultsList = append(resultsList, res)
	}

	// Check if the operation was terminated due to the global timeout.
	if ctx.Err() == context.DeadlineExceeded {
		slog.Error("Global timeout reached", "timeout", ctx.Err())
	}

	// If JSON output is requested, print the full report in JSON format and exit.
	if outputFormat == "json" {
		printJSON(resultsList, okCount, failCount, time.Since(startTotal))
		return
	}

	// Print a summary of the execution statistics (Text mode).
	totalDuration := time.Since(startTotal)
	slog.Info("Check completed",
		"total", len(urls),
		"ok", okCount,
		"fail", failCount,
		"duration", totalDuration,
	)
}

// worker is the core logic for each goroutine in the worker pool.
// It continuously pulls URLs from the jobs channel and processes them.
//
// Parameters:
//   - ctx: Context for cancellation.
//   - client: Shared HTTP client.
//   - limiter: Rate limiter (can be nil).
//   - jobs: Channel to receive URLs from.
//   - results: Channel to send results to.
//   - wg: WaitGroup to signal completion.
//   - maxRetries: Maximum number of retries allowed for failed requests.
func worker(ctx context.Context, client *http.Client, limiter *rate.Limiter, jobs <-chan string, results chan<- Result, wg *sync.WaitGroup, maxRetries int) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			// Context canceled, exit worker immediately.
			return
		case url, ok := <-jobs:
			if !ok {
				// Channel closed, no more jobs to process.
				return
			}

			// Apply rate limiting if configured.
			// Wait() blocks until the limiter allows the event to happen.
			if limiter != nil {
				if err := limiter.Wait(ctx); err != nil {
					// Context canceled while waiting.
					return
				}
			}

			results <- checkURLWithRetries(ctx, client, url, maxRetries)
		}
	}
}

// checkURLWithRetries performs the HTTP request with retry logic using exponential backoff.
// It attempts to fetch the URL up to 'maxRetries' + 1 times.
func checkURLWithRetries(ctx context.Context, client *http.Client, url string, maxRetries int) Result {
	var res Result
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			// Calculate exponential backoff delay: 500ms, 1s, 2s...
			backoff := time.Duration(math.Pow(2, float64(i-1))) * 500 * time.Millisecond
			slog.Debug("Retrying request", "url", url, "attempt", i+1, "backoff", backoff)

			// Wait for the backoff duration or context cancellation.
			select {
			case <-ctx.Done():
				return Result{URL: url, Err: ctx.Err(), ErrorMsg: ctx.Err().Error()}
			case <-time.After(backoff):
			}
		}

		res = checkURL(ctx, client, url)
		res.Retries = i

		// If the request was successful (2xx) or returned a 404 (which is a valid HTTP response),
		// we consider it "done" and do not retry.
		// We only retry on network errors or 5xx server errors.
		if res.Err == nil && res.StatusCode < 500 {
			return res
		}
	}
	return res
}

// checkURL performs a single HTTP GET request.
// It wraps the request in the provided context to support cancellation.
func checkURL(ctx context.Context, client *http.Client, url string) Result {
	start := time.Now()

	// Create a new request with the provided context.
	// This ensures that if the global context is canceled, the in-flight request is aborted.
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

// printJSON formats and prints the entire result set as a JSON object.
// This is used when the --output=json flag is set.
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

// printResult logs the result of a single URL check to the console using structured logging.
func printResult(r Result) {
	if r.Err != nil {
		slog.Error("Check failed", "url", r.URL, "error", r.Err, "duration", r.Duration)
	} else {
		slog.Info("Check success", "url", r.URL, "status", r.StatusCode, "duration", r.Duration)
	}
}
