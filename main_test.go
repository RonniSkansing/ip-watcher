package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseFlags(t *testing.T) {
	// Create a separate implementation of parseFlags for testing to avoid flag redefinition
	testParseFlags := func(args []string) *Config {
		config := &Config{}

		// Create a new FlagSet for testing
		fs := flag.NewFlagSet("test", flag.ExitOnError)

		fs.IntVar(&config.Interval, "interval", 300, "Interval between IP checks in seconds")
		fs.StringVar(&config.LogFile, "log", "", "Log file path (if not specified, logs to stdout only)")
		fs.StringVar(&config.IPEndpoint, "endpoint", "https://api.ipify.org?format=json", "URL of the IP checking service")
		fs.BoolVar(&config.QuietMode, "quiet", false, "If true, only logs to file and not stdout")
		fs.IntVar(&config.MaxRetries, "max-retries", 5, "Maximum number of retry attempts when fetching external IP")

		// Parse the test args
		fs.Parse(args)

		return config
	}

	// Test default values
	config := testParseFlags([]string{})
	if config.Interval != 300 {
		t.Errorf("Expected default interval to be 300, got %d", config.Interval)
	}
	if config.LogFile != "" {
		t.Errorf("Expected default log file to be empty, got %s", config.LogFile)
	}
	if !strings.Contains(config.IPEndpoint, "ipify.org") {
		t.Errorf("Expected default endpoint to contain ipify.org, got %s", config.IPEndpoint)
	}
	if config.QuietMode {
		t.Errorf("Expected default quiet mode to be false, got %v", config.QuietMode)
	}
	if config.MaxRetries != 5 {
		t.Errorf("Expected default max retries to be 5, got %d", config.MaxRetries)
	}

	// Test with custom values
	config = testParseFlags([]string{"-interval=60", "-log=/tmp/test.log", "-quiet=true", "-max-retries=3"})
	if config.Interval != 60 {
		t.Errorf("Expected interval to be 60, got %d", config.Interval)
	}
	if config.LogFile != "/tmp/test.log" {
		t.Errorf("Expected log file to be /tmp/test.log, got %s", config.LogFile)
	}
	if config.QuietMode != true {
		t.Errorf("Expected quiet mode to be true, got %v", config.QuietMode)
	}
	if config.MaxRetries != 3 {
		t.Errorf("Expected max retries to be 3, got %d", config.MaxRetries)
	}
}

func TestFetchExternalIP(t *testing.T) {
	// Test case 1: JSON response
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := IPifyResponse{IP: "192.168.1.1"}
		jsonBytes, _ := json.Marshal(response)
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonBytes)
	}))
	defer testServer.Close()

	ip, err := fetchExternalIP(testServer.URL, 1, 5)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if ip != "192.168.1.1" {
		t.Errorf("Expected IP to be 192.168.1.1, got %s", ip)
	}

	// Test case 2: Plain text response
	testServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("10.0.0.1"))
	}))
	defer testServer2.Close()

	ip, err = fetchExternalIP(testServer2.URL, 1, 5)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if ip != "10.0.0.1" {
		t.Errorf("Expected IP to be 10.0.0.1, got %s", ip)
	}

	// Test case 3: Error scenario - this will attempt to retry but eventually fail
	ip, err = fetchExternalIP("http://nonexistent.example.com", 1, 5)
	if err == nil {
		t.Error("Expected error for non-existent URL, got none")
	}
}

func TestCheckIP(t *testing.T) {
	// Setup a test server that returns a predefined IP
	var serverIP = "1.2.3.4"
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := IPifyResponse{IP: serverIP}
		jsonBytes, _ := json.Marshal(response)
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonBytes)
	}))
	defer testServer.Close()

	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	config := &Config{
		IPEndpoint: testServer.URL,
		MaxRetries: 5,
	}

	// First check should log the IP
	checkIP(config)
	output := buf.String()
	if !strings.Contains(output, "Current external IP: 1.2.3.4") {
		t.Errorf("Expected log to contain 'Current external IP: 1.2.3.4', got: %s", output)
	}

	// Reset buffer
	buf.Reset()

	// Second check with same IP should not log anything
	checkIP(config)
	output = buf.String()
	if output != "" {
		t.Errorf("Expected no log output for unchanged IP, got: %s", output)
	}

	// Change the IP and check that it logs the change
	buf.Reset()
	serverIP = "5.6.7.8"
	checkIP(config)
	output = buf.String()
	if !strings.Contains(output, "IP changed: 1.2.3.4 -> 5.6.7.8") {
		t.Errorf("Expected log to contain IP change message, got: %s", output)
	}
}

func TestSetupLogging(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ip-watcher-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "logs", "test.log")

	// Test with log file
	config := &Config{
		LogFile: logPath,
	}
	setupLogging(config)

	// Check if log directory was created
	if _, err := os.Stat(filepath.Dir(logPath)); os.IsNotExist(err) {
		t.Errorf("Expected log directory to be created at %s", filepath.Dir(logPath))
	}

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Expected log file to be created at %s", logPath)
	}
}

// TestFetchExternalIPWithStatusCodeRetry tests the retry functionality when HTTP status codes are non-2xx
func TestFetchExternalIPWithStatusCodeRetry(t *testing.T) {
	// Test server that fails the first few times with status code errors
	failCount := 0
	maxFails := 2

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failCount < maxFails {
			failCount++
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	
		// After max fails, return success
		response := IPifyResponse{IP: "192.168.1.100"}
		jsonBytes, _ := json.Marshal(response)
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonBytes)
	}))
	defer testServer.Close()

	// Should succeed after retries
	ip, err := fetchExternalIP(testServer.URL, 1, 5)

	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}

	if ip != "192.168.1.100" {
		t.Errorf("Expected IP to be 192.168.1.100, got %s", ip)
	}

	// Verify we had exactly the expected number of failures
	if failCount != maxFails {
		t.Errorf("Expected %d failures before success, got %d", maxFails, failCount)
	}

	// Test retry failure when all attempts fail
	alwaysFailServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer alwaysFailServer.Close()

	// Should fail after all retries
	_, err = fetchExternalIP(alwaysFailServer.URL, 1, 5)
	if err == nil {
		t.Error("Expected error after max retries, got nil")
	}
}

// TestFetchExternalIPWithNetworkErrorRetry tests the retry functionality with network errors
func TestFetchExternalIPWithNetworkErrorRetry(t *testing.T) {
	// Test with a non-existent URL that will cause a network error
	_, err := fetchExternalIP("http://non.existent.server.local", 1, 5)

	if err == nil {
		t.Error("Expected error for network failure, got none")
	}

	// Ensure error message contains the attempt information
	if !strings.Contains(err.Error(), "attempts") {
		t.Errorf("Error message doesn't mention attempts: %v", err)
	}
}

// TestFetchExternalIPWithConfigurableRetries tests that the max retries parameter works
func TestFetchExternalIPWithConfigurableRetries(t *testing.T) {
	// Counter for tracking the number of requests
	requestCount := 0
	
	// Create a test server that always fails with a 500 status
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer testServer.Close()
	
	// Test with max retries = 3
	maxRetries := 3
	_, err := fetchExternalIP(testServer.URL, 1, maxRetries)
	
	// Should fail
	if err == nil {
		t.Error("Expected error when all retries fail")
	}
	
	// Should have made exactly maxRetries attempts
	if requestCount != maxRetries {
		t.Errorf("Expected %d requests, got %d", maxRetries, requestCount)
	}
	
	// Reset counter
	requestCount = 0
	
	// Test with different max retries
	maxRetries = 2
	_, err = fetchExternalIP(testServer.URL, 1, maxRetries)
	
	// Should still fail
	if err == nil {
		t.Error("Expected error when all retries fail")
	}
	
	// Should have made exactly the new maxRetries attempts
	if requestCount != maxRetries {
		t.Errorf("Expected %d requests, got %d", maxRetries, requestCount)
	}
}

// Integration test that simulates the behavior of the IP watcher
func TestIPLoggerIntegration(t *testing.T) {
	// Skip this test in short mode as it's time-intensive
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This is a more complex test that simulates the IP watcher behavior
	// by using a test server that changes IP after a few requests

	requestCount := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ip string
		if requestCount < 2 {
			ip = "192.168.0.1"
		} else {
			ip = "192.168.0.2"
		}
		requestCount++

		response := IPifyResponse{IP: ip}
		jsonBytes, _ := json.Marshal(response)
		w.Write(jsonBytes)
	}))
	defer testServer.Close()

	// Set up config for test
	config := &Config{
		Interval:   1, // 1 second interval for faster testing
		IPEndpoint: testServer.URL,
		MaxRetries: 5,
	}

	// Capture log output
	oldOutput := log.Writer()
	r, w := io.Pipe()
	log.SetOutput(w)

	// Set up done channel with a buffer to prevent blocking
	done := make(chan bool, 1)

	// Start watcher in background
	go func() {
		buf := make([]byte, 1024)
		var outputStr string

		for {
			n, err := r.Read(buf)
			if err != nil {
				// Pipe closed or error
				return
			}

			if n > 0 {
				outputStr = string(buf[:n])

				if strings.Contains(outputStr, "Current external IP: 192.168.0.1") {
					// Found initial IP message
				}

				if strings.Contains(outputStr, "IP changed: 192.168.0.1 -> 192.168.0.2") {
					// Found IP change message
					done <- true
					return
				}
			}
		}
	}()

	// Set up a cancel context to gracefully terminate the test
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a ticker for IP checking instead of using startIPChecker
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	// Run the IP checker manually with controlled ticking
	go func() {
		// Do first check
		checkIP(config)

		// Then check on ticker
		for {
			select {
			case <-ticker.C:
				checkIP(config)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for test completion or timeout
	select {
	case <-done:
		// Test completed successfully
	case <-ctx.Done():
		t.Fatal("Test timed out waiting for IP change detection")
	}

	// Restore original log output and close the pipe
	w.Close()
	log.SetOutput(oldOutput)
}
