package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckURL_Success(t *testing.T) {
	// Mock server that returns 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	client := server.Client() // Use the client configured for the mock server

	result := checkURL(ctx, client, server.URL)

	if result.Err != nil {
		t.Errorf("Expected no error, got: %v", result.Err)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status code 200, got: %d", result.StatusCode)
	}
	if result.URL != server.URL {
		t.Errorf("Expected URL %s, got: %s", server.URL, result.URL)
	}
}

func TestCheckURL_NotFound(t *testing.T) {
	// Mock server that returns 404 Not Found
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ctx := context.Background()
	client := server.Client()

	result := checkURL(ctx, client, server.URL)

	if result.Err != nil {
		t.Errorf("Expected no error (even for 404), got: %v", result.Err)
	}
	if result.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status code 404, got: %d", result.StatusCode)
	}
}

func TestCheckURL_ContextCancellation(t *testing.T) {
	// Mock server that sleeps to allow cancellation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	client := server.Client()

	result := checkURL(ctx, client, server.URL)

	if result.Err == nil {
		t.Error("Expected error due to context cancellation, got nil")
	}
}

func TestCheckURL_NetworkError(t *testing.T) {
	// Use a closed port to simulate connection refusal
	client := &http.Client{Timeout: 1 * time.Second}
	ctx := context.Background()

	// Assuming nothing is listening on localhost:12345
	result := checkURL(ctx, client, "http://localhost:12345")

	if result.Err == nil {
		t.Error("Expected network error, got nil")
	}
}
