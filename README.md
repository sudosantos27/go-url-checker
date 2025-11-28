# go-url-checker

`go-url-checker` is a high-performance, concurrent CLI tool written in Go for checking the status of multiple URLs. It uses a worker pool pattern to process URLs in parallel, making it efficient for handling large lists of websites.

## Features

- **Concurrency**: Process multiple URLs in parallel using a configurable worker pool.
- **Global Timeout**: Set a maximum execution time for the entire process.
- **Detailed Reporting**: View status codes, response times, and errors for each URL, plus a final summary.
- **File Input**: Read URLs from a text file (one per line).

## Prerequisites

- [Go](https://go.dev/dl/) 1.20 or higher.

## Installation

1.  Clone the repository:
    ```bash
    git clone https://github.com/sudosantos27/go-url-checker.git
    cd go-url-checker
    ```

2.  Build the binary:
    ```bash
    go build -o url-checker ./cmd/url-checker
    ```

## Usage

You can run the tool directly using `go run` or by executing the built binary.

### Basic Usage

Check URLs listed in a file named `urls.txt`:

```bash
./url-checker -file urls.txt
```

### Run without building

You can also run the project directly without building the binary:

```bash
go run ./cmd/url-checker -file urls.txt
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `-file` | string | (required) | Path to the file containing the list of URLs. |
| `-concurrency` | int | `5` | Number of concurrent workers to use. |
| `-timeout` | duration | `30s` | Global timeout for the process (e.g., `10s`, `1m`). |

### Examples

**Run with 10 concurrent workers:**

```bash
./url-checker -file urls.txt -concurrency 10
```

**Run with a 1-minute global timeout:**

```bash
./url-checker -file urls.txt -timeout 1m
```

**Combine flags:**

```bash
./url-checker -file my-urls.txt -concurrency 20 -timeout 45s
```

## Input File Format

The input file should contain one URL per line. Example `urls.txt`:

```text
https://google.com
https://github.com
https://golang.org
https://example.com
https://non-existent-url.test
```

## Output Example

```text
Processing 5 URLs with 5 workers...
OK   https://google.com             (200) 150ms
OK   https://github.com             (200) 210ms
OK   https://golang.org             (200) 180ms
OK   https://example.com            (200) 120ms
FAIL https://non-existent-url.test  (error: dial tcp: lookup...) 50ms

--- Summary ---
Total: 5 URLs
OK:    4
FAIL:  1
Total duration: 250ms
```

## Testing

To run the unit tests for the project:

```bash
go test -v ./...
```

## Project Structure

- `cmd/url-checker/`: Contains the main entry point (`main.go`).
- `internal/checker/`: Contains the core logic for checking URLs and managing the worker pool.
