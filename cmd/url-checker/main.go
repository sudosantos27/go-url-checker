package main

import (
	"bufio"
	"os"
)

func main() {
	Execute()
}

// readURLs reads the file line by line and returns a slice of strings.
// Kept here or moved to a utility package? For now keeping it in main package but outside main func.
// Since root.go is in package main, it can access this.
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
