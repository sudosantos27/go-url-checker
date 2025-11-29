package main

import (
	"bufio"
	"os"
)

// main is the entry point of the application.
// It delegates the execution to the root command defined in the cmd/url-checker package.
func main() {
	Execute()
}

// readURLs reads a file from the given path and returns a slice of strings containing the URLs.
// It expects one URL per line. Empty lines are ignored.
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
